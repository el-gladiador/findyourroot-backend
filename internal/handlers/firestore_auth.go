package handlers

import (
	"context"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mamiri/findyourroot/internal/models"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/iterator"
)

type FirestoreAuthHandler struct {
	client *firestore.Client
}

func NewFirestoreAuthHandler(client *firestore.Client) *FirestoreAuthHandler {
	return &FirestoreAuthHandler{client: client}
}

// Login handles user authentication
func (h *FirestoreAuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	// Query user by email
	iter := h.client.Collection("users").Where("email", "==", req.Email).Limit(1).Documents(ctx)
	doc, err := iter.Next()
	if err == iterator.Done {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	var user models.User
	if err := doc.DataTo(&user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user data"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	user.ID = doc.Ref.ID

	// Generate JWT token
	token, err := h.generateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":          user.ID,
			"email":       user.Email,
			"role":        user.Role,
			"is_admin":    user.Role == models.RoleAdmin,
			"tree_name":   user.TreeName,
			"is_verified": user.IsVerified,
		},
	})
}

// ValidateToken validates a JWT token and returns user info
func (h *FirestoreAuthHandler) ValidateToken(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	ctx := context.Background()

	// Get user from Firestore
	doc, err := h.client.Collection("users").Doc(userID.(string)).Get(ctx)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	var user models.User
	if err := doc.DataTo(&user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user data"})
		return
	}

	user.ID = doc.Ref.ID

	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"user": gin.H{
			"id":          user.ID,
			"email":       user.Email,
			"role":        user.Role,
			"is_admin":    user.Role == models.RoleAdmin,
			"tree_name":   user.TreeName,
			"is_verified": user.IsVerified,
		},
	})
}

// generateToken creates a JWT token
func (h *FirestoreAuthHandler) generateToken(user models.User) (string, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", jwt.ErrInvalidKey
	}

	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"email":    user.Email,
		"is_admin": user.IsAdmin,
		"role":     string(user.Role),
		"iss":      "findyourroot-api",
		"sub":      user.ID,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
		"nbf":      time.Now().Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

// Register creates a new user with 'viewer' role by default
func (h *FirestoreAuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email and password are required"})
		return
	}

	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 6 characters"})
		return
	}

	// Validate tree name (only "Batur" allowed for now)
	if req.TreeName != "Batur" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tree name. Currently only 'Batur' tree is available."})
		return
	}

	ctx := context.Background()

	// Check if user already exists
	iter := h.client.Collection("users").Where("email", "==", req.Email).Limit(1).Documents(ctx)
	_, err := iter.Next()
	if err != iterator.Done {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	// Verify user exists in the family tree by father's name and birth year
	peopleIter := h.client.Collection("people").Where("birth", "==", req.BirthYear).Documents(ctx)
	defer peopleIter.Stop()

	var foundMatch bool
	for {
		doc, err := peopleIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			continue
		}

		var person models.Person
		if err := doc.DataTo(&person); err != nil {
			continue
		}

		// Find this person's parent and check if father's name matches
		parentsIter := h.client.Collection("people").Where("children", "array-contains", person.ID).Documents(ctx)
		for {
			parentDoc, err := parentsIter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				continue
			}

			var parent models.Person
			if err := parentDoc.DataTo(&parent); err != nil {
				continue
			}

			// Check if this parent's name contains the father's name
			if parent.Name == req.FatherName ||
				(len(parent.Name) > 0 && len(req.FatherName) > 0 &&
					(parent.Name == req.FatherName ||
						(len(parent.Name) >= len(req.FatherName) && parent.Name[:len(req.FatherName)] == req.FatherName))) {
				foundMatch = true
				break
			}
		}
		parentsIter.Stop()

		if foundMatch {
			break
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating user"})
		return
	}

	// Create user with verification status
	now := time.Now()
	user := models.User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Role:         models.RoleViewer,
		IsAdmin:      false,
		TreeName:     req.TreeName,
		FatherName:   req.FatherName,
		BirthYear:    req.BirthYear,
		IsVerified:   foundMatch,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	docRef, _, err := h.client.Collection("users").Add(ctx, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating user"})
		return
	}

	user.ID = docRef.ID

	// Generate token
	token, err := h.generateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user": gin.H{
			"id":          user.ID,
			"email":       user.Email,
			"role":        user.Role,
			"is_admin":    false,
			"tree_name":   user.TreeName,
			"is_verified": user.IsVerified,
		},
		"message": func() string {
			if user.IsVerified {
				return "Account created and verified! You are part of the Batur family tree."
			}
			return "Account created. Verification pending - we couldn't automatically match your information to the tree. An admin will review your details."
		}(),
	})
}

// RequestPermission creates a permission request from a user
func (h *FirestoreAuthHandler) RequestPermission(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	email, _ := c.Get("email")

	var req models.PermissionRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if req.RequestedRole != models.RoleEditor && req.RequestedRole != models.RoleAdmin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role requested. Must be 'editor' or 'admin'"})
		return
	}

	ctx := context.Background()

	// Check for existing pending requests
	iter := h.client.Collection("permission_requests").
		Where("user_id", "==", userID).
		Where("status", "==", "pending").
		Documents(ctx)
	_, err := iter.Next()
	if err != iterator.Done {
		c.JSON(http.StatusConflict, gin.H{"error": "You already have a pending permission request"})
		return
	}

	// Create permission request
	now := time.Now()
	permReq := models.PermissionRequest{
		UserID:        userID.(string),
		UserEmail:     email.(string),
		RequestedRole: req.RequestedRole,
		Message:       req.Message,
		Status:        "pending",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	docRef, _, err := h.client.Collection("permission_requests").Add(ctx, permReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating permission request"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Permission request submitted successfully",
		"id":      docRef.ID,
	})
}

// GetPermissionRequests lists permission requests (admin only)
func (h *FirestoreAuthHandler) GetPermissionRequests(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != string(models.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can view permission requests"})
		return
	}

	status := c.Query("status")
	if status == "" {
		status = "pending"
	}

	ctx := context.Background()
	iter := h.client.Collection("permission_requests").
		Where("status", "==", status).
		OrderBy("created_at", firestore.Desc).
		Documents(ctx)

	var requests []models.PermissionRequestResponse
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching requests"})
			return
		}

		var req models.PermissionRequest
		if err := doc.DataTo(&req); err != nil {
			continue
		}

		requests = append(requests, models.PermissionRequestResponse{
			Id:            doc.Ref.ID,
			UserId:        req.UserID,
			UserEmail:     req.UserEmail,
			RequestedRole: string(req.RequestedRole),
			Message:       req.Message,
			Status:        req.Status,
			CreatedAt:     req.CreatedAt,
			UpdatedAt:     req.UpdatedAt,
		})
	}

	if requests == nil {
		requests = []models.PermissionRequestResponse{}
	}

	c.JSON(http.StatusOK, requests)
}

// ApprovePermissionRequest approves a permission request with custom permissions (admin only)
func (h *FirestoreAuthHandler) ApprovePermissionRequest(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != string(models.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can approve permission requests"})
		return
	}

	requestID := c.Param("id")

	ctx := context.Background()

	// Get the permission request
	doc, err := h.client.Collection("permission_requests").Doc(requestID).Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Permission request not found"})
		return
	}

	var req models.PermissionRequest
	if err := doc.DataTo(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error parsing request"})
		return
	}

	if req.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request has already been processed"})
		return
	}

	// Use the requested role from the permission request
	newRole := req.RequestedRole
	isAdmin := newRole == models.RoleAdmin

	// Update user role
	_, err = h.client.Collection("users").Doc(req.UserID).Update(ctx, []firestore.Update{
		{Path: "role", Value: newRole},
		{Path: "is_admin", Value: isAdmin},
		{Path: "updated_at", Value: time.Now()},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating user"})
		return
	}

	// Update permission request status
	_, err = h.client.Collection("permission_requests").Doc(requestID).Update(ctx, []firestore.Update{
		{Path: "status", Value: "approved"},
		{Path: "updated_at", Value: time.Now()},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating request"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Permission request approved",
		"user":    req.UserEmail,
		"role":    newRole,
	})
}

// RejectPermissionRequest rejects a permission request (admin only)
func (h *FirestoreAuthHandler) RejectPermissionRequest(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != string(models.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can reject permission requests"})
		return
	}

	requestID := c.Param("id")
	ctx := context.Background()

	// Get the permission request
	doc, err := h.client.Collection("permission_requests").Doc(requestID).Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Permission request not found"})
		return
	}

	var req models.PermissionRequest
	if err := doc.DataTo(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error parsing request"})
		return
	}

	if req.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request has already been processed"})
		return
	}

	// Update permission request status
	_, err = h.client.Collection("permission_requests").Doc(requestID).Update(ctx, []firestore.Update{
		{Path: "status", Value: "rejected"},
		{Path: "updated_at", Value: time.Now()},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating request"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Permission request rejected",
		"user":    req.UserEmail,
	})
}
