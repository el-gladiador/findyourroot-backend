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

	person := models.Person{
		ID:        id,
		Name:      req.Name,
		Role:      req.Role,
		Birth:     req.Birth,
		Location:  req.Location,
		Avatar:    req.Avatar,
		Bio:       req.Bio,
		Children:  req.Children,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if person.Children == nil {
		person.Children = []string{}
	}

	_, err := h.client.Collection("people").Doc(id).Set(ctx, person)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create person"})
		return
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

	_, err := h.client.Collection("people").Doc(id).Delete(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete person"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Person deleted successfully"})
}
