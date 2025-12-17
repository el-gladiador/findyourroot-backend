package handlers

import (
	"context"
	"net/http"
	"sort"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mamiri/findyourroot/internal/models"
	"google.golang.org/api/iterator"
)

// FirestoreIdentityClaimHandler handles identity claim operations
type FirestoreIdentityClaimHandler struct {
	client *firestore.Client
}

// NewFirestoreIdentityClaimHandler creates a new identity claim handler
func NewFirestoreIdentityClaimHandler(client *firestore.Client) *FirestoreIdentityClaimHandler {
	return &FirestoreIdentityClaimHandler{client: client}
}

// ClaimIdentity allows a user to claim they are a specific person in the tree
func (h *FirestoreIdentityClaimHandler) ClaimIdentity(c *gin.Context) {
	var req models.ClaimIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	userEmail, _ := c.Get("email")
	ctx := context.Background()

	// Check if user already has a linked person
	userDoc, err := h.client.Collection("users").Doc(userID.(string)).Get(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
		return
	}

	var user models.User
	if err := userDoc.DataTo(&user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user data"})
		return
	}

	if user.PersonID != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You are already linked to a person in the tree"})
		return
	}

	// Check if the person exists
	personDoc, err := h.client.Collection("people").Doc(req.PersonID).Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Person not found in the tree"})
		return
	}

	var person models.Person
	if err := personDoc.DataTo(&person); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse person data"})
		return
	}

	// Check if this person is already claimed by someone else
	if person.LinkedUserID != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This person is already linked to another user"})
		return
	}

	// Check if user already has a pending claim
	iter := h.client.Collection("identity_claims").
		Where("user_id", "==", userID.(string)).
		Where("status", "==", "pending").
		Limit(1).
		Documents(ctx)

	existingDoc, err := iter.Next()
	if err != iterator.Done && existingDoc != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You already have a pending identity claim"})
		return
	}

	// Create the claim request
	claimID := uuid.New().String()
	now := time.Now()

	claim := models.IdentityClaimRequest{
		ID:         claimID,
		UserID:     userID.(string),
		UserEmail:  userEmail.(string),
		PersonID:   req.PersonID,
		PersonName: person.Name,
		Message:    req.Message,
		Status:     "pending",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	_, err = h.client.Collection("identity_claims").Doc(claimID).Set(ctx, claim)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create identity claim"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Identity claim submitted successfully. An admin will review your request.",
		"claim":   claim,
	})
}

// GetMyIdentityClaim returns the current user's identity claim status
func (h *FirestoreIdentityClaimHandler) GetMyIdentityClaim(c *gin.Context) {
	userID, _ := c.Get("user_id")
	ctx := context.Background()

	// Get user to check if already linked
	userDoc, err := h.client.Collection("users").Doc(userID.(string)).Get(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
		return
	}

	var user models.User
	if err := userDoc.DataTo(&user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user data"})
		return
	}

	// If user is already linked, return that info
	if user.PersonID != "" {
		personDoc, err := h.client.Collection("people").Doc(user.PersonID).Get(ctx)
		if err == nil {
			var person models.Person
			if err := personDoc.DataTo(&person); err == nil {
				c.JSON(http.StatusOK, gin.H{
					"linked": true,
					"person": person,
				})
				return
			}
		}
	}

	// Find any pending or recent claims - query without OrderBy to avoid index requirement
	iter := h.client.Collection("identity_claims").
		Where("user_id", "==", userID.(string)).
		Documents(ctx)

	var claims []models.IdentityClaimRequest
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch claims"})
			return
		}

		var claim models.IdentityClaimRequest
		if err := doc.DataTo(&claim); err != nil {
			continue
		}
		claims = append(claims, claim)
	}

	if len(claims) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"linked": false,
			"claim":  nil,
		})
		return
	}

	// Sort by created_at descending and get the most recent
	sort.Slice(claims, func(i, j int) bool {
		return claims[i].CreatedAt.After(claims[j].CreatedAt)
	})

	c.JSON(http.StatusOK, gin.H{
		"linked": false,
		"claim":  claims[0],
	})
}

// GetIdentityClaims returns all identity claims (admin only)
func (h *FirestoreIdentityClaimHandler) GetIdentityClaims(c *gin.Context) {
	status := c.DefaultQuery("status", "pending")
	ctx := context.Background()

	// Query without OrderBy to avoid needing composite index
	iter := h.client.Collection("identity_claims").
		Where("status", "==", status).
		Documents(ctx)
	defer iter.Stop()

	var claims []models.IdentityClaimRequest
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch claims"})
			return
		}

		var claim models.IdentityClaimRequest
		if err := doc.DataTo(&claim); err != nil {
			continue
		}
		claims = append(claims, claim)
	}

	if claims == nil {
		claims = []models.IdentityClaimRequest{}
	}

	// Sort by created_at descending in code
	sort.Slice(claims, func(i, j int) bool {
		return claims[i].CreatedAt.After(claims[j].CreatedAt)
	})

	c.JSON(http.StatusOK, claims)
}

// ReviewIdentityClaim allows admin to approve or reject an identity claim
func (h *FirestoreIdentityClaimHandler) ReviewIdentityClaim(c *gin.Context) {
	claimID := c.Param("id")

	var req models.ReviewClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminID, _ := c.Get("user_id")
	ctx := context.Background()

	// Get the claim
	claimDoc, err := h.client.Collection("identity_claims").Doc(claimID).Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Claim not found"})
		return
	}

	var claim models.IdentityClaimRequest
	if err := claimDoc.DataTo(&claim); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse claim data"})
		return
	}

	if claim.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This claim has already been reviewed"})
		return
	}

	now := time.Now()
	newStatus := "rejected"
	if req.Approved {
		newStatus = "approved"
	}

	// Use a transaction to update claim, user, and person atomically
	err = h.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// Update the claim
		claimRef := h.client.Collection("identity_claims").Doc(claimID)
		if err := tx.Update(claimRef, []firestore.Update{
			{Path: "status", Value: newStatus},
			{Path: "reviewed_by", Value: adminID.(string)},
			{Path: "review_notes", Value: req.ReviewNotes},
			{Path: "updated_at", Value: now},
		}); err != nil {
			return err
		}

		if req.Approved {
			// Link the user to the person
			userRef := h.client.Collection("users").Doc(claim.UserID)
			if err := tx.Update(userRef, []firestore.Update{
				{Path: "person_id", Value: claim.PersonID},
				{Path: "is_verified", Value: true},
				{Path: "updated_at", Value: now},
			}); err != nil {
				return err
			}

			// Link the person to the user
			personRef := h.client.Collection("people").Doc(claim.PersonID)
			if err := tx.Update(personRef, []firestore.Update{
				{Path: "linked_user_id", Value: claim.UserID},
				{Path: "updated_at", Value: now},
			}); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process review"})
		return
	}

	message := "Identity claim rejected"
	if req.Approved {
		message = "Identity claim approved. User is now linked to the tree node."
	}

	c.JSON(http.StatusOK, gin.H{"message": message})
}

// UnlinkIdentity allows admin to unlink a user from a tree node
func (h *FirestoreIdentityClaimHandler) UnlinkIdentity(c *gin.Context) {
	userID := c.Param("user_id")
	ctx := context.Background()

	// Get user
	userDoc, err := h.client.Collection("users").Doc(userID).Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var user models.User
	if err := userDoc.DataTo(&user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user data"})
		return
	}

	if user.PersonID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User is not linked to any person"})
		return
	}

	personID := user.PersonID
	now := time.Now()

	// Use transaction to unlink both user and person
	err = h.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// Unlink user
		userRef := h.client.Collection("users").Doc(userID)
		if err := tx.Update(userRef, []firestore.Update{
			{Path: "person_id", Value: ""},
			{Path: "updated_at", Value: now},
		}); err != nil {
			return err
		}

		// Unlink person
		personRef := h.client.Collection("people").Doc(personID)
		if err := tx.Update(personRef, []firestore.Update{
			{Path: "linked_user_id", Value: ""},
			{Path: "updated_at", Value: now},
		}); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unlink identity"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User unlinked from tree node successfully"})
}

// LinkUserToPersonRequest represents a request to link a user to a tree node by admin
type LinkUserToPersonRequest struct {
	UserID            string `json:"user_id" binding:"required"`
	PersonID          string `json:"person_id" binding:"required"`
	InstagramUsername string `json:"instagram_username"`
}

// LinkUserToPerson allows admin to directly link a user to a tree node (without user request)
func (h *FirestoreIdentityClaimHandler) LinkUserToPerson(c *gin.Context) {
	var req LinkUserToPersonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminRole, _ := c.Get("role")
	if adminRole != string(models.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admin can link users to tree nodes"})
		return
	}

	ctx := context.Background()

	// Get user
	userDoc, err := h.client.Collection("users").Doc(req.UserID).Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var user models.User
	if err := userDoc.DataTo(&user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user data"})
		return
	}

	if user.PersonID != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User is already linked to a person"})
		return
	}

	// Get person
	personDoc, err := h.client.Collection("people").Doc(req.PersonID).Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Person not found"})
		return
	}

	var person models.Person
	if err := personDoc.DataTo(&person); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse person data"})
		return
	}

	if person.LinkedUserID != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Person is already linked to another user"})
		return
	}

	now := time.Now()

	// Use transaction to link both user and person
	err = h.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// Update user
		userRef := h.client.Collection("users").Doc(req.UserID)
		if err := tx.Update(userRef, []firestore.Update{
			{Path: "person_id", Value: req.PersonID},
			{Path: "updated_at", Value: now},
		}); err != nil {
			return err
		}

		// Update person
		personRef := h.client.Collection("people").Doc(req.PersonID)
		updates := []firestore.Update{
			{Path: "linked_user_id", Value: req.UserID},
			{Path: "updated_at", Value: now},
		}
		if req.InstagramUsername != "" {
			updates = append(updates, firestore.Update{Path: "instagram_username", Value: req.InstagramUsername})
		}
		if err := tx.Update(personRef, updates); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to link user to person"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User linked to tree node successfully"})
}

// UpdatePersonInstagramRequest represents a request to update person's Instagram
type UpdatePersonInstagramRequest struct {
	InstagramUsername string `json:"instagram_username" binding:"required"`
}

// UpdatePersonInstagram allows admin to update a person's Instagram username
func (h *FirestoreIdentityClaimHandler) UpdatePersonInstagram(c *gin.Context) {
	personID := c.Param("person_id")

	var req UpdatePersonInstagramRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminRole, _ := c.Get("role")
	if adminRole != string(models.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admin can update Instagram usernames"})
		return
	}

	ctx := context.Background()

	// Get person
	personDoc, err := h.client.Collection("people").Doc(personID).Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Person not found"})
		return
	}

	var person models.Person
	if err := personDoc.DataTo(&person); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse person data"})
		return
	}

	// Person must be linked to a user to have Instagram
	if person.LinkedUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Person must be linked to a user before adding Instagram"})
		return
	}

	now := time.Now()

	// Update person
	_, err = h.client.Collection("people").Doc(personID).Update(ctx, []firestore.Update{
		{Path: "instagram_username", Value: req.InstagramUsername},
		{Path: "updated_at", Value: now},
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update Instagram"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Instagram username updated successfully"})
}
