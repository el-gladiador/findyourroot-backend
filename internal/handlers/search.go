package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
	"github.com/mamiri/findyourroot/internal/models"
	"google.golang.org/api/iterator"
)

// SearchRequest represents search parameters
type SearchRequest struct {
	Query    string `form:"q"`
	Location string `form:"location"`
	Role     string `form:"role"`
	YearFrom string `form:"year_from"`
	YearTo   string `form:"year_to"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// SearchResponse represents paginated search results
type SearchResponse struct {
	Data       []models.Person `json:"data"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	TotalPages int             `json:"total_pages"`
}

// FirestoreSearchHandler handles search operations
type FirestoreSearchHandler struct {
	client *firestore.Client
}

// NewFirestoreSearchHandler creates a new search handler
func NewFirestoreSearchHandler(client *firestore.Client) *FirestoreSearchHandler {
	return &FirestoreSearchHandler{client: client}
}

// SearchPeople searches for people with filters and pagination
func (h *FirestoreSearchHandler) SearchPeople(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default pagination
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 50
	}

	ctx := context.Background()

	// Fetch all people (Firestore doesn't support complex text search natively)
	// For production, consider using Algolia or Elasticsearch
	iter := h.client.Collection("people").Documents(ctx)
	defer iter.Stop()

	var allPeople []models.Person
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
		allPeople = append(allPeople, person)
	}

	// Apply filters
	var filtered []models.Person
	for _, person := range allPeople {
		if !h.matchesFilters(person, req) {
			continue
		}
		filtered = append(filtered, person)
	}

	// Calculate pagination
	total := len(filtered)
	totalPages := (total + req.PageSize - 1) / req.PageSize
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	var paged []models.Person
	if start < total {
		paged = filtered[start:end]
	} else {
		paged = []models.Person{}
	}

	c.JSON(http.StatusOK, SearchResponse{
		Data:       paged,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	})
}

// matchesFilters checks if a person matches all search filters
func (h *FirestoreSearchHandler) matchesFilters(person models.Person, req SearchRequest) bool {
	// Text search (name, role, location, bio)
	if req.Query != "" {
		query := strings.ToLower(req.Query)
		nameMatch := strings.Contains(strings.ToLower(person.Name), query)
		roleMatch := strings.Contains(strings.ToLower(person.Role), query)
		locationMatch := strings.Contains(strings.ToLower(person.Location), query)
		bioMatch := strings.Contains(strings.ToLower(person.Bio), query)

		if !nameMatch && !roleMatch && !locationMatch && !bioMatch {
			return false
		}
	}

	// Location filter
	if req.Location != "" {
		if !strings.Contains(strings.ToLower(person.Location), strings.ToLower(req.Location)) {
			return false
		}
	}

	// Role filter
	if req.Role != "" {
		if !strings.Contains(strings.ToLower(person.Role), strings.ToLower(req.Role)) {
			return false
		}
	}

	// Year range filter
	if req.YearFrom != "" || req.YearTo != "" {
		birthYear, err := strconv.Atoi(person.Birth)
		if err != nil {
			return false // Can't parse birth year, exclude from filtered results
		}

		if req.YearFrom != "" {
			yearFrom, err := strconv.Atoi(req.YearFrom)
			if err == nil && birthYear < yearFrom {
				return false
			}
		}

		if req.YearTo != "" {
			yearTo, err := strconv.Atoi(req.YearTo)
			if err == nil && birthYear > yearTo {
				return false
			}
		}
	}

	return true
}

// GetLocations returns all unique locations for filter dropdown
func (h *FirestoreSearchHandler) GetLocations(c *gin.Context) {
	ctx := context.Background()

	iter := h.client.Collection("people").Documents(ctx)
	defer iter.Stop()

	locationSet := make(map[string]bool)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch locations"})
			return
		}

		var person models.Person
		if err := doc.DataTo(&person); err != nil {
			continue
		}
		if person.Location != "" {
			locationSet[person.Location] = true
		}
	}

	locations := make([]string, 0, len(locationSet))
	for loc := range locationSet {
		locations = append(locations, loc)
	}

	c.JSON(http.StatusOK, gin.H{"locations": locations})
}

// GetRoles returns all unique roles for filter dropdown
func (h *FirestoreSearchHandler) GetRoles(c *gin.Context) {
	ctx := context.Background()

	iter := h.client.Collection("people").Documents(ctx)
	defer iter.Stop()

	roleSet := make(map[string]bool)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch roles"})
			return
		}

		var person models.Person
		if err := doc.DataTo(&person); err != nil {
			continue
		}
		if person.Role != "" {
			roleSet[person.Role] = true
		}
	}

	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}

	c.JSON(http.StatusOK, gin.H{"roles": roles})
}
