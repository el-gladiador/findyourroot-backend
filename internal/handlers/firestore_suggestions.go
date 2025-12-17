package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mamiri/findyourroot/internal/models"
	"google.golang.org/api/iterator"
)

type FirestoreSuggestionHandler struct {
	client *firestore.Client
}

func NewFirestoreSuggestionHandler(client *firestore.Client) *FirestoreSuggestionHandler {
	return &FirestoreSuggestionHandler{client: client}
}

// CreateSuggestion creates a new suggestion for tree changes (contributors)
func (h *FirestoreSuggestionHandler) CreateSuggestion(c *gin.Context) {
	userID, _ := c.Get("user_id")
	email, _ := c.Get("email")

	var req models.CreateSuggestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate suggestion type
	if req.Type != models.SuggestionAdd && req.Type != models.SuggestionEdit && req.Type != models.SuggestionDelete {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid suggestion type. Must be 'add', 'edit', or 'delete'"})
		return
	}

	// Validate required fields based on type
	if req.Type == models.SuggestionAdd {
		if req.PersonData == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "person_data is required for add suggestions"})
			return
		}
		if req.PersonData.Name == "" || req.PersonData.Role == "" || req.PersonData.Birth == "" || req.PersonData.Location == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name, role, birth, and location are required in person_data"})
			return
		}
	}

	if req.Type == models.SuggestionEdit {
		if req.TargetPersonID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target_person_id is required for edit suggestions"})
			return
		}
		if req.PersonData == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "person_data is required for edit suggestions"})
			return
		}
	}

	if req.Type == models.SuggestionDelete {
		if req.TargetPersonID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target_person_id is required for delete suggestions"})
			return
		}
	}

	ctx := context.Background()

	// For edit/delete, verify the target person exists
	if req.Type == models.SuggestionEdit || req.Type == models.SuggestionDelete {
		_, err := h.client.Collection("people").Doc(req.TargetPersonID).Get(ctx)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Target person not found"})
			return
		}
	}

	// For add with parent, verify parent exists
	if req.Type == models.SuggestionAdd && req.TargetPersonID != "" {
		_, err := h.client.Collection("people").Doc(req.TargetPersonID).Get(ctx)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Parent person not found"})
			return
		}
	}

	now := time.Now()
	suggestion := models.Suggestion{
		ID:             uuid.New().String(),
		Type:           req.Type,
		TargetPersonID: req.TargetPersonID,
		PersonData:     req.PersonData,
		Message:        req.Message,
		Status:         "pending",
		UserID:         userID.(string),
		UserEmail:      email.(string),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	_, err := h.client.Collection("suggestions").Doc(suggestion.ID).Set(ctx, suggestion)
	if err != nil {
		log.Printf("[Suggestion] Error creating suggestion: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create suggestion"})
		return
	}

	log.Printf("[Suggestion] Created suggestion %s by %s: type=%s", suggestion.ID, email, suggestion.Type)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Suggestion submitted successfully",
		"id":      suggestion.ID,
	})
}

// GetMySuggestions returns suggestions created by the current user
func (h *FirestoreSuggestionHandler) GetMySuggestions(c *gin.Context) {
	userID, _ := c.Get("user_id")
	status := c.DefaultQuery("status", "")

	ctx := context.Background()

	query := h.client.Collection("suggestions").Where("user_id", "==", userID.(string))
	if status != "" {
		query = query.Where("status", "==", status)
	}

	iter := query.Documents(ctx)
	defer iter.Stop()

	var suggestions []models.SuggestionResponse
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch suggestions"})
			return
		}

		var s models.Suggestion
		if err := doc.DataTo(&s); err != nil {
			continue
		}

		resp := h.suggestionToResponse(ctx, s)
		suggestions = append(suggestions, resp)
	}

	if suggestions == nil {
		suggestions = []models.SuggestionResponse{}
	}

	// Sort by created_at descending
	sort.Slice(suggestions, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, suggestions[i].CreatedAt)
		tj, _ := time.Parse(time.RFC3339, suggestions[j].CreatedAt)
		return ti.After(tj)
	})

	c.JSON(http.StatusOK, suggestions)
}

// GetAllSuggestions returns all suggestions (for admins/co-admins)
func (h *FirestoreSuggestionHandler) GetAllSuggestions(c *gin.Context) {
	status := c.DefaultQuery("status", "pending")
	email, _ := c.Get("email")
	role, _ := c.Get("role")

	log.Printf("[GetAllSuggestions] Request from %s (role: %s), filter status: %s", email, role, status)

	ctx := context.Background()

	iter := h.client.Collection("suggestions").Where("status", "==", status).Documents(ctx)
	defer iter.Stop()

	var suggestions []models.SuggestionResponse
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("[GetAllSuggestions] Error fetching suggestions: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch suggestions"})
			return
		}

		var s models.Suggestion
		if err := doc.DataTo(&s); err != nil {
			log.Printf("[GetAllSuggestions] Error parsing suggestion %s: %v", doc.Ref.ID, err)
			continue
		}

		resp := h.suggestionToResponse(ctx, s)
		suggestions = append(suggestions, resp)
	}

	if suggestions == nil {
		suggestions = []models.SuggestionResponse{}
	}

	log.Printf("[GetAllSuggestions] Found %d suggestions with status '%s'", len(suggestions), status)

	// Sort by created_at descending
	sort.Slice(suggestions, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, suggestions[i].CreatedAt)
		tj, _ := time.Parse(time.RFC3339, suggestions[j].CreatedAt)
		return ti.After(tj)
	})

	c.JSON(http.StatusOK, suggestions)
}

// ReviewSuggestion approves or rejects a suggestion (admins/co-admins)
func (h *FirestoreSuggestionHandler) ReviewSuggestion(c *gin.Context) {
	suggestionID := c.Param("id")
	reviewerID, _ := c.Get("user_id")
	reviewerEmail, _ := c.Get("email")

	var req models.ReviewSuggestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	ctx := context.Background()

	// Get the suggestion
	doc, err := h.client.Collection("suggestions").Doc(suggestionID).Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Suggestion not found"})
		return
	}

	var suggestion models.Suggestion
	if err := doc.DataTo(&suggestion); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse suggestion"})
		return
	}

	if suggestion.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Suggestion has already been reviewed"})
		return
	}

	now := time.Now()
	newStatus := "rejected"
	if req.Approved {
		newStatus = "approved"
	}

	// If approved, execute the suggestion
	if req.Approved {
		if err := h.executeSuggestion(ctx, suggestion); err != nil {
			log.Printf("[Suggestion] Error executing suggestion %s: %v", suggestionID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to execute suggestion: %v", err)})
			return
		}
	}

	// Update suggestion status
	_, err = h.client.Collection("suggestions").Doc(suggestionID).Update(ctx, []firestore.Update{
		{Path: "status", Value: newStatus},
		{Path: "reviewed_by", Value: reviewerID.(string)},
		{Path: "reviewer_email", Value: reviewerEmail.(string)},
		{Path: "review_notes", Value: req.ReviewNotes},
		{Path: "updated_at", Value: now},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update suggestion"})
		return
	}

	log.Printf("[Suggestion] Suggestion %s %s by %s", suggestionID, newStatus, reviewerEmail)

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Suggestion %s", newStatus),
		"id":      suggestionID,
	})
}

// executeSuggestion performs the actual tree modification
func (h *FirestoreSuggestionHandler) executeSuggestion(ctx context.Context, s models.Suggestion) error {
	switch s.Type {
	case models.SuggestionAdd:
		return h.executeAdd(ctx, s)
	case models.SuggestionEdit:
		return h.executeEdit(ctx, s)
	case models.SuggestionDelete:
		return h.executeDelete(ctx, s)
	default:
		return fmt.Errorf("unknown suggestion type: %s", s.Type)
	}
}

func (h *FirestoreSuggestionHandler) executeAdd(ctx context.Context, s models.Suggestion) error {
	id := uuid.New().String()
	now := time.Now()

	// Generate default avatar if not provided
	avatar := s.PersonData.Avatar
	if avatar == "" {
		encodedName := url.QueryEscape(s.PersonData.Name)
		avatar = fmt.Sprintf("https://api.dicebear.com/7.x/avataaars/svg?seed=%s&backgroundColor=b6e3f4", encodedName)
	}

	person := models.Person{
		ID:        id,
		Name:      s.PersonData.Name,
		Role:      s.PersonData.Role,
		Birth:     s.PersonData.Birth,
		Location:  s.PersonData.Location,
		Avatar:    avatar,
		Bio:       s.PersonData.Bio,
		Children:  []string{},
		CreatedBy: s.UserID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// If parent ID provided, use transaction to add person and update parent
	if s.TargetPersonID != "" {
		return h.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			parentRef := h.client.Collection("people").Doc(s.TargetPersonID)
			parentDoc, err := tx.Get(parentRef)
			if err != nil {
				return fmt.Errorf("parent not found: %v", err)
			}

			var parent models.Person
			if err := parentDoc.DataTo(&parent); err != nil {
				return err
			}

			// Create the new person
			personRef := h.client.Collection("people").Doc(id)
			if err := tx.Set(personRef, person); err != nil {
				return err
			}

			// Update parent's children
			newChildren := append(parent.Children, id)
			if err := tx.Update(parentRef, []firestore.Update{
				{Path: "children", Value: newChildren},
				{Path: "updated_at", Value: now},
			}); err != nil {
				return err
			}

			return nil
		})
	}

	// No parent - just create the person
	_, err := h.client.Collection("people").Doc(id).Set(ctx, person)
	return err
}

func (h *FirestoreSuggestionHandler) executeEdit(ctx context.Context, s models.Suggestion) error {
	updates := []firestore.Update{
		{Path: "updated_at", Value: time.Now()},
	}

	if s.PersonData.Name != "" {
		updates = append(updates, firestore.Update{Path: "name", Value: s.PersonData.Name})
	}
	if s.PersonData.Role != "" {
		updates = append(updates, firestore.Update{Path: "role", Value: s.PersonData.Role})
	}
	if s.PersonData.Birth != "" {
		updates = append(updates, firestore.Update{Path: "birth", Value: s.PersonData.Birth})
	}
	if s.PersonData.Location != "" {
		updates = append(updates, firestore.Update{Path: "location", Value: s.PersonData.Location})
	}
	if s.PersonData.Avatar != "" {
		updates = append(updates, firestore.Update{Path: "avatar", Value: s.PersonData.Avatar})
	}
	if s.PersonData.Bio != "" {
		updates = append(updates, firestore.Update{Path: "bio", Value: s.PersonData.Bio})
	}

	_, err := h.client.Collection("people").Doc(s.TargetPersonID).Update(ctx, updates)
	return err
}

func (h *FirestoreSuggestionHandler) executeDelete(ctx context.Context, s models.Suggestion) error {
	// Get the person to delete
	doc, err := h.client.Collection("people").Doc(s.TargetPersonID).Get(ctx)
	if err != nil {
		return fmt.Errorf("person not found: %v", err)
	}

	var person models.Person
	if err := doc.DataTo(&person); err != nil {
		return err
	}

	return h.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// Find and update parent to remove this person from children
		parentsIter := h.client.Collection("people").Where("children", "array-contains", s.TargetPersonID).Documents(ctx)
		for {
			parentDoc, err := parentsIter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return err
			}

			var parent models.Person
			if err := parentDoc.DataTo(&parent); err != nil {
				continue
			}

			// Remove deleted person from parent's children
			newChildren := make([]string, 0)
			for _, childID := range parent.Children {
				if childID != s.TargetPersonID {
					newChildren = append(newChildren, childID)
				}
			}

			if err := tx.Update(parentDoc.Ref, []firestore.Update{
				{Path: "children", Value: newChildren},
				{Path: "updated_at", Value: time.Now()},
			}); err != nil {
				return err
			}
		}
		parentsIter.Stop()

		// Delete the person
		return tx.Delete(h.client.Collection("people").Doc(s.TargetPersonID))
	})
}

// Helper to convert suggestion to response with target person info
func (h *FirestoreSuggestionHandler) suggestionToResponse(ctx context.Context, s models.Suggestion) models.SuggestionResponse {
	resp := models.SuggestionResponse{
		ID:             s.ID,
		Type:           string(s.Type),
		TargetPersonID: s.TargetPersonID,
		PersonData:     s.PersonData,
		Message:        s.Message,
		Status:         s.Status,
		UserID:         s.UserID,
		UserEmail:      s.UserEmail,
		ReviewedBy:     s.ReviewedBy,
		ReviewerEmail:  s.ReviewerEmail,
		ReviewNotes:    s.ReviewNotes,
		CreatedAt:      s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      s.UpdatedAt.Format(time.RFC3339),
	}

	// For edit/delete, include the target person info
	if s.TargetPersonID != "" && (s.Type == models.SuggestionEdit || s.Type == models.SuggestionDelete) {
		doc, err := h.client.Collection("people").Doc(s.TargetPersonID).Get(ctx)
		if err == nil {
			var person models.Person
			if err := doc.DataTo(&person); err == nil {
				resp.TargetPerson = &person
			}
		}
	}

	return resp
}
