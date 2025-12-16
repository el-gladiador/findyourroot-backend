package handlers

import (
	"context"
	"log"
	"net/http"
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

	// Find any pending or recent claims
	iter := h.client.Collection("identity_claims").
		Where("user_id", "==", userID.(string)).
		OrderBy("created_at", firestore.Desc).
		Limit(1).
		Documents(ctx)

	doc, err := iter.Next()
	if err == iterator.Done {
		c.JSON(http.StatusOK, gin.H{
			"linked": false,
			"claim":  nil,
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch claims"})
		return
	}

	var claim models.IdentityClaimRequest
	if err := doc.DataTo(&claim); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse claim data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"linked": false,
		"claim":  claim,
	})
}

// GetIdentityClaims returns all identity claims (admin only)
func (h *FirestoreIdentityClaimHandler) GetIdentityClaims(c *gin.Context) {
	status := c.DefaultQuery("status", "pending")
	ctx := context.Background()

	log.Printf("[IdentityClaims] Fetching claims with status: %s", status)

	iter := h.client.Collection("identity_claims").
		Where("status", "==", status).
		OrderBy("created_at", firestore.Desc).
		Documents(ctx)
	defer iter.Stop()

	var claims []models.IdentityClaimRequest
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("[IdentityClaims] Error fetching claims: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch claims: " + err.Error()})
			return
		}

		var claim models.IdentityClaimRequest
		if err := doc.DataTo(&claim); err != nil {
			log.Printf("[IdentityClaims] Error parsing claim data: %v", err)
			continue
		}
		claims = append(claims, claim)
	}

	if claims == nil {
		claims = []models.IdentityClaimRequest{}
	}

	log.Printf("[IdentityClaims] Found %d claims with status '%s'", len(claims), status)
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
