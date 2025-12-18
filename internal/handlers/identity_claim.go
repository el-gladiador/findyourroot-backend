package handlers

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mamiri/findyourroot/internal/models"
	"github.com/mamiri/findyourroot/internal/utils"
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

	// Check if user already has a linked person (Person owns this relationship)
	// Query the Person collection to find if any person links to this user
	existingLinkIter := h.client.Collection("people").Where("linked_user_id", "==", userID.(string)).Limit(1).Documents(ctx)
	existingLinkDoc, err := existingLinkIter.Next()
	existingLinkIter.Stop()
	if err == nil && existingLinkDoc != nil {
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

	// Check if user is already linked (Person owns this relationship)
	// Query the Person collection to find if any person links to this user
	linkedPersonIter := h.client.Collection("people").Where("linked_user_id", "==", userID.(string)).Limit(1).Documents(ctx)
	linkedPersonDoc, err := linkedPersonIter.Next()
	linkedPersonIter.Stop()
	if err == nil && linkedPersonDoc != nil {
		var person models.Person
		if err := linkedPersonDoc.DataTo(&person); err == nil {
			person.ID = linkedPersonDoc.Ref.ID
			c.JSON(http.StatusOK, gin.H{
				"linked": true,
				"person": person,
			})
			return
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

	// Use a transaction to update claim and person atomically
	// NOTE: Person owns the link (Person.LinkedUserID), User does NOT store person_id
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
			// Update user verification status (but NOT person_id - Person owns that)
			userRef := h.client.Collection("users").Doc(claim.UserID)
			if err := tx.Update(userRef, []firestore.Update{
				{Path: "is_verified", Value: true},
				{Path: "updated_at", Value: now},
			}); err != nil {
				return err
			}

			// Link the person to the user - Person is the OWNER of this relationship
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
// Person is the OWNER of the link, so we find the person that links to this user and clear it
func (h *FirestoreIdentityClaimHandler) UnlinkIdentity(c *gin.Context) {
	userID := c.Param("user_id")
	ctx := context.Background()

	// Find the person that links to this user (Person owns the relationship)
	iter := h.client.Collection("people").Where("linked_user_id", "==", userID).Limit(1).Documents(ctx)
	personDoc, err := iter.Next()
	iter.Stop()

	if err == iterator.Done {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User is not linked to any person"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find linked person"})
		return
	}

	now := time.Now()

	// Only update Person - Person is the single source of truth for the link
	_, err = h.client.Collection("people").Doc(personDoc.Ref.ID).Update(ctx, []firestore.Update{
		{Path: "linked_user_id", Value: ""},
		{Path: "updated_at", Value: now},
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
	adminUserID, _ := c.Get("user_id")

	// Allow admin and co-admin to link users
	// Co-admins can only link themselves, admins can link anyone
	isAdmin := adminRole == string(models.RoleAdmin)
	isCoAdmin := adminRole == string(models.RoleCoAdmin)
	isSelfLink := adminUserID.(string) == req.UserID

	if !isAdmin && !(isCoAdmin && isSelfLink) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admin can link other users. Co-admins can only link themselves."})
		return
	}

	ctx := context.Background()

	// Verify user exists
	userDoc, err := h.client.Collection("users").Doc(req.UserID).Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if !userDoc.Exists() {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if user is already linked (Person owns this relationship)
	// Query the Person collection to find if any person links to this user
	existingLinkIter := h.client.Collection("people").Where("linked_user_id", "==", req.UserID).Limit(1).Documents(ctx)
	existingLinkDoc, err := existingLinkIter.Next()
	existingLinkIter.Stop()
	if err == nil && existingLinkDoc != nil {
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

	// If Instagram username provided, try to fetch the profile
	instagramUsername := strings.TrimPrefix(req.InstagramUsername, "@")
	var instagramProfile *utils.InstagramProfile
	if instagramUsername != "" {
		profile, err := utils.FetchInstagramProfile(instagramUsername)
		if err == nil && profile != nil {
			instagramProfile = profile
		}
		// Don't fail if Instagram fetch fails - just continue without profile
	}

	// Person is the OWNER of the link relationship
	// Only update Person.linked_user_id - User does NOT store person_id
	personRef := h.client.Collection("people").Doc(req.PersonID)
	updates := []firestore.Update{
		{Path: "linked_user_id", Value: req.UserID},
		{Path: "updated_at", Value: now},
	}
	if instagramUsername != "" {
		updates = append(updates, firestore.Update{Path: "instagram_username", Value: instagramUsername})
	}
	if instagramProfile != nil {
		if instagramProfile.AvatarURL != "" {
			updates = append(updates, firestore.Update{Path: "instagram_avatar_url", Value: instagramProfile.AvatarURL})
		}
		if instagramProfile.FullName != "" {
			updates = append(updates, firestore.Update{Path: "instagram_full_name", Value: instagramProfile.FullName})
		}
		if instagramProfile.Bio != "" {
			updates = append(updates, firestore.Update{Path: "instagram_bio", Value: instagramProfile.Bio})
		}
		updates = append(updates, firestore.Update{Path: "instagram_is_verified", Value: instagramProfile.IsVerified})
	}
	_, err = personRef.Update(ctx, updates)

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

// LookupInstagramProfile allows admin to lookup an Instagram profile before linking
func (h *FirestoreIdentityClaimHandler) LookupInstagramProfile(c *gin.Context) {
	username := c.Query("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username is required"})
		return
	}

	username = strings.TrimPrefix(username, "@")

	if !utils.ValidateInstagramUsername(username) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Instagram username format"})
		return
	}

	profile, err := utils.FetchInstagramProfile(username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Instagram profile not found or unavailable"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username":            profile.Username,
		"full_name":           profile.FullName,
		"avatar_url":          profile.AvatarURL,
		"avatar_url_fallback": utils.GetInstagramAvatarProxyAlternatives(username),
		"bio":                 profile.Bio,
		"is_verified":         profile.IsVerified,
	})
}
