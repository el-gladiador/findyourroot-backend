package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mamiri/findyourroot/internal/models"
	"github.com/mamiri/findyourroot/internal/utils"
	"google.golang.org/api/iterator"
)

type FirestoreTreeHandler struct {
	client *firestore.Client
}

func NewFirestoreTreeHandler(client *firestore.Client) *FirestoreTreeHandler {
	return &FirestoreTreeHandler{client: client}
}

// generateDefaultAvatar creates a default avatar URL based on the person's name
func generateDefaultAvatar(name string) string {
	// Use DiceBear Avataaars for consistent, reproducible avatars
	encodedName := url.QueryEscape(name)
	return fmt.Sprintf("https://api.dicebear.com/7.x/avataaars/svg?seed=%s&backgroundColor=b6e3f4", encodedName)
}

// GetAllPeople returns all people in the tree
// Also validates references and cleans up any dangling ones
func (h *FirestoreTreeHandler) GetAllPeople(c *gin.Context) {
	ctx := context.Background()

	iter := h.client.Collection("people").Documents(ctx)
	defer iter.Stop()

	var people []models.Person
	var allPersonIDs = make(map[string]bool)
	var allUserIDs = make(map[string]bool)

	// First pass: collect all people and build ID sets
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch people"})
			return
		}

		var person models.Person
		if err := doc.DataTo(&person); err != nil {
			continue
		}
		people = append(people, person)
		allPersonIDs[person.ID] = true
	}

	// Fetch all valid user IDs for liked_by and linked_user_id validation
	usersIter := h.client.Collection("users").Documents(ctx)
	for {
		doc, err := usersIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			break // Non-critical, continue without user validation
		}
		allUserIDs[doc.Ref.ID] = true
	}
	usersIter.Stop()

	// Second pass: validate references and clean up in background
	integrityService := NewReferentialIntegrityService(h.client)
	for i := range people {
		person := &people[i]
		needsCleanup := false

		// Check children references
		validChildren := make([]string, 0)
		for _, childID := range person.Children {
			if allPersonIDs[childID] {
				validChildren = append(validChildren, childID)
			} else {
				needsCleanup = true
				log.Printf("[GetAllPeople] Found dangling child reference %s in person %s", childID, person.ID)
			}
		}
		if needsCleanup {
			person.Children = validChildren
		}

		// Check liked_by references
		validLikedBy := make([]string, 0)
		likedByChanged := false
		for _, userID := range person.LikedBy {
			if allUserIDs[userID] {
				validLikedBy = append(validLikedBy, userID)
			} else {
				likedByChanged = true
				log.Printf("[GetAllPeople] Found dangling liked_by reference %s in person %s", userID, person.ID)
			}
		}
		if likedByChanged {
			person.LikedBy = validLikedBy
			person.LikesCount = len(validLikedBy)
			needsCleanup = true
		}

		// Check linked_user_id
		if person.LinkedUserID != "" && !allUserIDs[person.LinkedUserID] {
			log.Printf("[GetAllPeople] Found dangling linked_user_id %s in person %s", person.LinkedUserID, person.ID)
			person.LinkedUserID = ""
			needsCleanup = true
		}

		// Clean up in background if needed
		if needsCleanup {
			go func(personID string) {
				integrityService.ValidatePersonReferences(context.Background(), personID)
			}(person.ID)
		}
	}

	if people == nil {
		people = []models.Person{}
	}

	c.JSON(http.StatusOK, people)
}

// GetPerson returns a single person by ID
func (h *FirestoreTreeHandler) GetPerson(c *gin.Context) {
	id := c.Param("id")
	ctx := context.Background()

	doc, err := h.client.Collection("people").Doc(id).Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Person not found"})
		return
	}

	var person models.Person
	if err := doc.DataTo(&person); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse person data"})
		return
	}

	c.JSON(http.StatusOK, person)
}

// CreatePerson creates a new person in the tree
func (h *FirestoreTreeHandler) CreatePerson(c *gin.Context) {
	var req models.CreatePersonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Debug logging
	if req.ParentID != nil {
		log.Printf("[CreatePerson] Creating child with parent_id: %s", *req.ParentID)
	} else {
		log.Printf("[CreatePerson] Creating root person (no parent_id)")
	}

	ctx := context.Background()
	id := uuid.New().String()
	now := time.Now()

	// Get user ID from context
	userID, _ := c.Get("user_id")

	// Generate default avatar if not provided
	avatar := req.Avatar
	if avatar == "" {
		avatar = generateDefaultAvatar(req.Name)
	}

	person := models.Person{
		ID:        id,
		Name:      req.Name,
		Role:      req.Role,
		Birth:     req.Birth,
		Location:  req.Location,
		Avatar:    avatar,
		Bio:       req.Bio,
		Children:  []string{},
		CreatedBy: userID.(string),
		CreatedAt: now,
		UpdatedAt: now,
	}

	// If parentID is provided, use a transaction to create person and update parent atomically
	if req.ParentID != nil && *req.ParentID != "" {
		err := h.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			// First, check if parent exists
			parentRef := h.client.Collection("people").Doc(*req.ParentID)
			parentDoc, err := tx.Get(parentRef)
			if err != nil {
				log.Printf("[CreatePerson] Error getting parent: %v", err)
				return err
			}

			var parent models.Person
			if err := parentDoc.DataTo(&parent); err != nil {
				log.Printf("[CreatePerson] Error parsing parent data: %v", err)
				return err
			}
			log.Printf("[CreatePerson] Found parent: %s, current children: %v", parent.Name, parent.Children)

			// Create the child person
			personRef := h.client.Collection("people").Doc(id)
			if err := tx.Set(personRef, person); err != nil {
				log.Printf("[CreatePerson] Error creating child: %v", err)
				return err
			}
			log.Printf("[CreatePerson] Created child: %s", person.Name)

			// Update parent's children array using ArrayUnion (atomic, prevents duplicates)
			err = tx.Update(parentRef, []firestore.Update{
				{Path: "children", Value: firestore.ArrayUnion(id)},
				{Path: "updated_at", Value: now},
			})
			if err != nil {
				log.Printf("[CreatePerson] Error updating parent's children: %v", err)
				return err
			}
			log.Printf("[CreatePerson] Successfully updated parent's children array")
			return nil
		})

		if err != nil {
			log.Printf("[CreatePerson] Transaction failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create person with parent relationship: %v", err)})
			return
		}
		log.Printf("[CreatePerson] Transaction completed successfully")
	} else {
		// No parent, just create the person
		_, err := h.client.Collection("people").Doc(id).Set(ctx, person)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create person"})
			return
		}
	}

	c.JSON(http.StatusCreated, person)
}

// UpdatePerson updates an existing person
func (h *FirestoreTreeHandler) UpdatePerson(c *gin.Context) {
	id := c.Param("id")

	var req models.UpdatePersonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	// Check if person exists
	doc, err := h.client.Collection("people").Doc(id).Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Person not found"})
		return
	}

	var person models.Person
	if err := doc.DataTo(&person); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse person data"})
		return
	}

	// Check ownership: only creator or admin can edit
	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")
	if person.CreatedBy != userID.(string) && role != string(models.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only edit nodes you created"})
		return
	}

	// Build update map
	updates := []firestore.Update{
		{Path: "updated_at", Value: time.Now()},
	}

	if req.Name != nil {
		updates = append(updates, firestore.Update{Path: "name", Value: *req.Name})
		person.Name = *req.Name
	}
	if req.Role != nil {
		updates = append(updates, firestore.Update{Path: "role", Value: *req.Role})
		person.Role = *req.Role
	}
	if req.Birth != nil {
		updates = append(updates, firestore.Update{Path: "birth", Value: *req.Birth})
		person.Birth = *req.Birth
	}
	if req.Location != nil {
		updates = append(updates, firestore.Update{Path: "location", Value: *req.Location})
		person.Location = *req.Location
	}
	if req.Avatar != nil {
		updates = append(updates, firestore.Update{Path: "avatar", Value: *req.Avatar})
		person.Avatar = *req.Avatar
	}
	if req.Bio != nil {
		updates = append(updates, firestore.Update{Path: "bio", Value: *req.Bio})
		person.Bio = *req.Bio
	}
	if req.Children != nil {
		updates = append(updates, firestore.Update{Path: "children", Value: req.Children})
		person.Children = req.Children
	}

	_, err = h.client.Collection("people").Doc(id).Update(ctx, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update person"})
		return
	}

	person.UpdatedAt = time.Now()
	c.JSON(http.StatusOK, person)
}

// DeletePerson deletes a person from the tree
func (h *FirestoreTreeHandler) DeletePerson(c *gin.Context) {
	id := c.Param("id")
	ctx := context.Background()

	// Check if person exists and verify ownership
	doc, err := h.client.Collection("people").Doc(id).Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Person not found"})
		return
	}

	var person models.Person
	if err := doc.DataTo(&person); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse person data"})
		return
	}

	// Check ownership: only creator or admin can delete
	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")
	if person.CreatedBy != userID.(string) && role != string(models.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only delete nodes you created"})
		return
	}

	// Use ReferentialIntegrityService to clean up all references BEFORE deleting
	integrityService := NewReferentialIntegrityService(h.client)
	if err := integrityService.OnPersonDeleted(ctx, id); err != nil {
		log.Printf("[DeletePerson] Warning: Integrity cleanup had issues: %v", err)
		// Continue with deletion anyway - cleanup is best-effort
	}

	// Now delete the person
	_, err = h.client.Collection("people").Doc(id).Delete(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete person"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Person deleted successfully"})
}

// DeleteAllPeople deletes all people from the tree (for testing)
func (h *FirestoreTreeHandler) DeleteAllPeople(c *gin.Context) {
	ctx := context.Background()

	// Get all documents
	iter := h.client.Collection("people").Documents(ctx)
	defer iter.Stop()

	batch := h.client.Batch()
	count := 0

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch people"})
			return
		}

		batch.Delete(doc.Ref)
		count++

		// Firestore batch limit is 500
		if count%500 == 0 {
			if _, err := batch.Commit(ctx); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete people"})
				return
			}
			batch = h.client.Batch()
		}
	}

	// Commit remaining
	if count%500 != 0 {
		if _, err := batch.Commit(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete people"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "All people deleted successfully"})
}

// LikePerson allows a user to like a person
func (h *FirestoreTreeHandler) LikePerson(c *gin.Context) {
	id := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	ctx := context.Background()

	// Use a transaction to atomically update likes
	err := h.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		docRef := h.client.Collection("people").Doc(id)
		doc, err := tx.Get(docRef)
		if err != nil {
			return err
		}

		var person models.Person
		if err := doc.DataTo(&person); err != nil {
			return err
		}

		// Check if user already liked
		for _, uid := range person.LikedBy {
			if uid == userID.(string) {
				return fmt.Errorf("already liked")
			}
		}

		// Add user to liked_by array and increment likes_count
		return tx.Update(docRef, []firestore.Update{
			{Path: "liked_by", Value: firestore.ArrayUnion(userID.(string))},
			{Path: "likes_count", Value: person.LikesCount + 1},
			{Path: "updated_at", Value: time.Now()},
		})
	})

	if err != nil {
		if err.Error() == "already liked" {
			c.JSON(http.StatusConflict, gin.H{"error": "You have already liked this person"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to like person: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Person liked successfully"})
}

// UnlikePerson allows a user to unlike a person
func (h *FirestoreTreeHandler) UnlikePerson(c *gin.Context) {
	id := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	ctx := context.Background()

	// Use a transaction to atomically update likes
	err := h.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		docRef := h.client.Collection("people").Doc(id)
		doc, err := tx.Get(docRef)
		if err != nil {
			return err
		}

		var person models.Person
		if err := doc.DataTo(&person); err != nil {
			return err
		}

		// Check if user has liked
		found := false
		for _, uid := range person.LikedBy {
			if uid == userID.(string) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("not liked")
		}

		// Remove user from liked_by array and decrement likes_count
		newCount := person.LikesCount - 1
		if newCount < 0 {
			newCount = 0
		}

		return tx.Update(docRef, []firestore.Update{
			{Path: "liked_by", Value: firestore.ArrayRemove(userID.(string))},
			{Path: "likes_count", Value: newCount},
			{Path: "updated_at", Value: time.Now()},
		})
	})

	if err != nil {
		if err.Error() == "not liked" {
			c.JSON(http.StatusConflict, gin.H{"error": "You have not liked this person"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to unlike person: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Person unliked successfully"})
}

// CheckDuplicateNameRequest represents a request to check for duplicate names
type CheckDuplicateNameRequest struct {
	Name      string  `json:"name" binding:"required"`
	Threshold float64 `json:"threshold"` // Default 0.8 if not provided
	UseAI     bool    `json:"use_ai"`    // Whether to use Gemini AI for matching
}

// CheckDuplicateName checks if a name already exists or is similar to existing names
func (h *FirestoreTreeHandler) CheckDuplicateName(c *gin.Context) {
	var req CheckDuplicateNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default threshold
	threshold := req.Threshold
	if threshold == 0 {
		threshold = 0.75 // 75% similarity
	}

	ctx := context.Background()

	// Get all existing names
	iter := h.client.Collection("people").Documents(ctx)
	defer iter.Stop()

	existingNames := make(map[string]string) // personID -> name
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch people"})
			return
		}

		var person models.Person
		if err := doc.DataTo(&person); err != nil {
			continue
		}
		existingNames[person.ID] = person.Name
	}

	// Find similar names using traditional algorithm
	matches := utils.FindSimilarNames(req.Name, existingNames, threshold)

	// Optionally enhance with AI matching (if enabled and API key available)
	aiUsed := false
	if req.UseAI {
		aiMatches, err := utils.CheckNameListWithGemini(req.Name, existingNames)
		if err != nil {
			log.Printf("Gemini AI matching failed (falling back to traditional): %v", err)
		} else if len(aiMatches) > 0 {
			aiUsed = true
			// Merge AI results with traditional results, avoiding duplicates
			existingIDs := make(map[string]bool)
			for _, m := range matches {
				existingIDs[m.PersonID] = true
			}
			for _, aiMatch := range aiMatches {
				if !existingIDs[aiMatch.PersonID] {
					matches = append(matches, aiMatch)
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"has_duplicates": len(matches) > 0,
		"matches":        matches,
		"input_name":     req.Name,
		"normalized":     utils.NormalizePersianNameKeepSpaces(req.Name),
		"ai_enhanced":    aiUsed,
	})
}
