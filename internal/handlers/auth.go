package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mamiri/findyourroot/internal/middleware"
	"github.com/mamiri/findyourroot/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db *sql.DB
}

func NewAuthHandler(db *sql.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

// Login handles user authentication
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user from database
	var user models.User
	var password string
	err := h.db.QueryRow(
		`SELECT id, email, password_hash, role, is_admin, created_at, updated_at 
		 FROM users WHERE email = $1`,
		req.Email,
	).Scan(&user.ID, &user.Email, &password, &user.Role, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}
	if err != nil {
		fmt.Printf("Database error during login: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Generate JWT token
	token, err := h.generateToken(user.Email, user.IsAdmin, string(user.Role))
	if err != nil {
		fmt.Printf("Error generating token: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	fmt.Printf("User logged in successfully: %s (role: %s)\n", user.Email, user.Role)
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"email":    user.Email,
			"role":     user.Role,
			"is_admin": user.IsAdmin,
		},
	})
}

// ValidateToken validates a JWT token and returns user info
func (h *AuthHandler) ValidateToken(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user from database
	var user models.User
	err := h.db.QueryRow(
		`SELECT id, email, role, is_admin, created_at, updated_at 
		 FROM users WHERE id = $1`,
		userID,
	).Scan(&user.ID, &user.Email, &user.Role, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"user": gin.H{
			"id":       user.ID,
			"email":    user.Email,
			"role":     user.Role,
			"is_admin": user.IsAdmin,
		},
	})
}

func (h *AuthHandler) generateToken(email string, isAdmin bool, role string) (string, error) {
	// Get JWT secret
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", fmt.Errorf("JWT_SECRET is not configured")
	}

	// Get user ID from database
	var userID string
	err := h.db.QueryRow("SELECT id FROM users WHERE email = $1", email).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user ID: %w", err)
	}

	// Create claims with expiration
	claims := middleware.Claims{
		UserID:  userID,
		Email:   email,
		IsAdmin: isAdmin,
		Role:    role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "findyourroot-api",
			Subject:   userID,
		},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign and return token
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// Register creates a new user with 'viewer' role by default
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("Invalid registration request: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate email and password
	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email and password are required"})
		return
	}

	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 6 characters"})
		return
	}

	// Check if user already exists
	var existingID string
	err := h.db.QueryRow("SELECT id FROM users WHERE email = $1", req.Email).Scan(&existingID)
	if err == nil {
		fmt.Printf("Registration failed: user already exists: %s\n", req.Email)
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}
	if err != sql.ErrNoRows {
		fmt.Printf("Database error checking existing user: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("Error hashing password: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating user"})
		return
	}

	// Create user with viewer role
	userID := uuid.New().String()
	_, err = h.db.Exec(
		"INSERT INTO users (id, email, password_hash, role, is_admin, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, NOW(), NOW())",
		userID, req.Email, string(hashedPassword), models.RoleViewer, false,
	)
	if err != nil {
		fmt.Printf("Error creating user: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating user"})
		return
	}

	// Generate token
	token, err := h.generateToken(req.Email, false, string(models.RoleViewer))
	if err != nil {
		fmt.Printf("Error generating token for new user: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
		return
	}

	fmt.Printf("User registered successfully: %s (role: viewer)\n", req.Email)
	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user": gin.H{
			"id":       userID,
			"email":    req.Email,
			"role":     models.RoleViewer,
			"is_admin": false,
		},
	})
}

// RequestPermission creates a permission request from a user
func (h *AuthHandler) RequestPermission(c *gin.Context) {
	claims, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userClaims := claims.(*middleware.Claims)

	var req models.PermissionRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("Invalid permission request: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate requested role
	if req.RequestedRole != models.RoleEditor && req.RequestedRole != models.RoleAdmin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role requested. Must be 'editor' or 'admin'"})
		return
	}

	// Check if user already has an admin role
	if userClaims.Role == string(models.RoleAdmin) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You already have admin privileges"})
		return
	}

	// Check if there's already a pending request
	var existingID string
	err := h.db.QueryRow(
		"SELECT id FROM permission_requests WHERE user_email = $1 AND status = 'pending'",
		userClaims.Email,
	).Scan(&existingID)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "You already have a pending permission request"})
		return
	}
	if err != sql.ErrNoRows {
		fmt.Printf("Database error checking existing requests: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Create permission request
	requestID := uuid.New().String()
	_, err = h.db.Exec(
		"INSERT INTO permission_requests (id, user_id, user_email, requested_role, message, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())",
		requestID, userClaims.UserID, userClaims.Email, req.RequestedRole, req.Message, "pending",
	)
	if err != nil {
		fmt.Printf("Error creating permission request: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating request"})
		return
	}

	fmt.Printf("Permission request created: %s requesting %s role\n", userClaims.Email, req.RequestedRole)
	c.JSON(http.StatusCreated, gin.H{
		"id":             requestID,
		"requested_role": req.RequestedRole,
		"status":         "pending",
	})
}

// GetPermissionRequests returns all permission requests (admin only)
func (h *AuthHandler) GetPermissionRequests(c *gin.Context) {
	claims, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userClaims := claims.(*middleware.Claims)
	if userClaims.Role != string(models.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can view permission requests"})
		return
	}

	status := c.Query("status")
	if status == "" {
		status = "pending"
	}

	rows, err := h.db.Query(
		"SELECT id, user_id, user_email, requested_role, message, status, created_at, updated_at FROM permission_requests WHERE status = $1 ORDER BY created_at DESC",
		status,
	)
	if err != nil {
		fmt.Printf("Error fetching permission requests: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching requests"})
		return
	}
	defer rows.Close()

	var requests []models.PermissionRequestResponse
	for rows.Next() {
		var req models.PermissionRequestResponse
		if err := rows.Scan(&req.Id, &req.UserId, &req.UserEmail, &req.RequestedRole, &req.Message, &req.Status, &req.CreatedAt, &req.UpdatedAt); err != nil {
			fmt.Printf("Error scanning permission request: %v\n", err)
			continue
		}
		requests = append(requests, req)
	}

	if requests == nil {
		requests = []models.PermissionRequestResponse{}
	}

	c.JSON(http.StatusOK, requests)
}

// ApprovePermissionRequest approves a permission request and updates user role (admin only)
func (h *AuthHandler) ApprovePermissionRequest(c *gin.Context) {
	claims, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userClaims := claims.(*middleware.Claims)
	if userClaims.Role != string(models.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can approve permission requests"})
		return
	}

	requestID := c.Param("id")

	// Get the permission request
	var req models.PermissionRequest
	err := h.db.QueryRow(
		"SELECT id, user_id, user_email, requested_role, status FROM permission_requests WHERE id = $1",
		requestID,
	).Scan(&req.ID, &req.UserID, &req.UserEmail, &req.RequestedRole, &req.Status)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Permission request not found"})
		return
	}
	if err != nil {
		fmt.Printf("Error fetching permission request: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if req.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request has already been processed"})
		return
	}

	// Start transaction
	tx, err := h.db.Begin()
	if err != nil {
		fmt.Printf("Error starting transaction: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer tx.Rollback()

	// Update user role
	isAdmin := req.RequestedRole == models.RoleAdmin
	_, err = tx.Exec(
		"UPDATE users SET role = $1, is_admin = $2, updated_at = NOW() WHERE id = $3",
		req.RequestedRole, isAdmin, req.UserID,
	)
	if err != nil {
		fmt.Printf("Error updating user role: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating user"})
		return
	}

	// Update permission request status
	_, err = tx.Exec(
		"UPDATE permission_requests SET status = 'approved', updated_at = NOW() WHERE id = $1",
		requestID,
	)
	if err != nil {
		fmt.Printf("Error updating permission request: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating request"})
		return
	}

	if err := tx.Commit(); err != nil {
		fmt.Printf("Error committing transaction: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error committing changes"})
		return
	}

	fmt.Printf("Permission request approved: %s granted %s role\n", req.UserEmail, req.RequestedRole)
	c.JSON(http.StatusOK, gin.H{
		"message": "Permission request approved",
		"user":    req.UserEmail,
		"role":    req.RequestedRole,
	})
}

// RejectPermissionRequest rejects a permission request (admin only)
func (h *AuthHandler) RejectPermissionRequest(c *gin.Context) {
	claims, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userClaims := claims.(*middleware.Claims)
	if userClaims.Role != string(models.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can reject permission requests"})
		return
	}

	requestID := c.Param("id")

	// Check if request exists and is pending
	var status string
	err := h.db.QueryRow("SELECT status FROM permission_requests WHERE id = $1", requestID).Scan(&status)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Permission request not found"})
		return
	}
	if err != nil {
		fmt.Printf("Error fetching permission request: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request has already been processed"})
		return
	}

	// Update permission request status
	_, err = h.db.Exec(
		"UPDATE permission_requests SET status = 'rejected', updated_at = NOW() WHERE id = $1",
		requestID,
	)
	if err != nil {
		fmt.Printf("Error rejecting permission request: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating request"})
		return
	}

	fmt.Printf("Permission request rejected: %s\n", requestID)
	c.JSON(http.StatusOK, gin.H{"message": "Permission request rejected"})
}
