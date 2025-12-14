package handlers

import (
	"context"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mamiri/findyourroot/internal/models"
	"google.golang.org/api/iterator"
)

type FirestoreTreeHandler struct {
	client *firestore.Client
}

func NewFirestoreTreeHandler(client *firestore.Client) *FirestoreTreeHandler {
	return &FirestoreTreeHandler{client: client}
}

// GetAllPeople returns all people in the tree
func (h *FirestoreTreeHandler) GetAllPeople(c *gin.Context) {
	ctx := context.Background()

	iter := h.client.Collection("people").Documents(ctx)
	defer iter.Stop()

	var people []models.Person
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

	ctx := context.Background()
	id := uuid.New().String()
	now := time.Now()

	person := models.Person{
		ID:        id,
		Name:      req.Name,
		Role:      req.Role,
		Birth:     req.Birth,
		Location:  req.Location,
		Avatar:    req.Avatar,
		Bio:       req.Bio,
		Children:  []string{},
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
				return err
			}

			var parent models.Person
			if err := parentDoc.DataTo(&parent); err != nil {
				return err
			}

			// Create the child person
			personRef := h.client.Collection("people").Doc(id)
			if err := tx.Set(personRef, person); err != nil {
				return err
			}

			// Update parent's children array using ArrayUnion (atomic, prevents duplicates)
			return tx.Update(parentRef, []firestore.Update{
				{Path: "children", Value: firestore.ArrayUnion(id)},
				{Path: "updated_at", Value: now},
			})
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create person with parent relationship"})
			return
		}
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

	// First, remove this person from any parent's children array using ArrayRemove (atomic)
	iter := h.client.Collection("people").Where("children", "array-contains", id).Documents(ctx)
	defer iter.Stop()

	// Collect all parents that need updating
	var parentRefs []*firestore.DocumentRef
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find parent relationships"})
			return
		}
		parentRefs = append(parentRefs, doc.Ref)
	}

	// Use batch write to update all parents atomically
	if len(parentRefs) > 0 {
		batch := h.client.Batch()
		for _, parentRef := range parentRefs {
			batch.Update(parentRef, []firestore.Update{
				{Path: "children", Value: firestore.ArrayRemove(id)},
				{Path: "updated_at", Value: time.Now()},
			})
		}

		// Commit the batch
		_, err := batch.Commit(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cleanup parent relationships"})
			return
		}
	}

	// Now delete the person
	_, err := h.client.Collection("people").Doc(id).Delete(ctx)
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
