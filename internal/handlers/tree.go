package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/mamiri/findyourroot/internal/models"
)

type TreeHandler struct {
	db *sql.DB
}

func NewTreeHandler(db *sql.DB) *TreeHandler {
	return &TreeHandler{db: db}
}

// GetAllPeople returns all people in the tree
func (h *TreeHandler) GetAllPeople(c *gin.Context) {
	rows, err := h.db.Query(`
		SELECT id, name, role, birth, location, avatar, bio, children, created_at, updated_at
		FROM people
		ORDER BY created_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var people []models.Person
	for rows.Next() {
		var p models.Person
		var children pq.StringArray
		err := rows.Scan(
			&p.ID, &p.Name, &p.Role, &p.Birth, &p.Location,
			&p.Avatar, &p.Bio, &children, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan person"})
			return
		}
		p.Children = children
		people = append(people, p)
	}

	if people == nil {
		people = []models.Person{}
	}

	c.JSON(http.StatusOK, people)
}

// GetPerson returns a single person by ID
func (h *TreeHandler) GetPerson(c *gin.Context) {
	id := c.Param("id")

	var p models.Person
	var children pq.StringArray
	err := h.db.QueryRow(`
		SELECT id, name, role, birth, location, avatar, bio, children, created_at, updated_at
		FROM people WHERE id = $1
	`, id).Scan(
		&p.ID, &p.Name, &p.Role, &p.Birth, &p.Location,
		&p.Avatar, &p.Bio, &children, &p.CreatedAt, &p.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Person not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	p.Children = children
	c.JSON(http.StatusOK, p)
}

// CreatePerson creates a new person in the tree
func (h *TreeHandler) CreatePerson(c *gin.Context) {
	var req models.CreatePersonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id := uuid.New().String()
	children := pq.Array(req.Children)

	var p models.Person
	var childrenResult pq.StringArray
	err := h.db.QueryRow(`
		INSERT INTO people (id, name, role, birth, location, avatar, bio, children)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, name, role, birth, location, avatar, bio, children, created_at, updated_at
	`, id, req.Name, req.Role, req.Birth, req.Location, req.Avatar, req.Bio, children).Scan(
		&p.ID, &p.Name, &p.Role, &p.Birth, &p.Location,
		&p.Avatar, &p.Bio, &childrenResult, &p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create person"})
		return
	}

	p.Children = childrenResult
	c.JSON(http.StatusCreated, p)
}

// UpdatePerson updates an existing person
func (h *TreeHandler) UpdatePerson(c *gin.Context) {
	id := c.Param("id")

	var req models.UpdatePersonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if person exists
	var exists bool
	err := h.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM people WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Person not found"})
		return
	}

	// Build dynamic update query
	query := `UPDATE people SET updated_at = CURRENT_TIMESTAMP`
	args := []interface{}{}
	argCount := 1

	if req.Name != nil {
		query += `, name = $` + string(rune(argCount+48))
		args = append(args, *req.Name)
		argCount++
	}
	if req.Role != nil {
		query += `, role = $` + string(rune(argCount+48))
		args = append(args, *req.Role)
		argCount++
	}
	if req.Birth != nil {
		query += `, birth = $` + string(rune(argCount+48))
		args = append(args, *req.Birth)
		argCount++
	}
	if req.Location != nil {
		query += `, location = $` + string(rune(argCount+48))
		args = append(args, *req.Location)
		argCount++
	}
	if req.Avatar != nil {
		query += `, avatar = $` + string(rune(argCount+48))
		args = append(args, *req.Avatar)
		argCount++
	}
	if req.Bio != nil {
		query += `, bio = $` + string(rune(argCount+48))
		args = append(args, *req.Bio)
		argCount++
	}
	if req.Children != nil {
		query += `, children = $` + string(rune(argCount+48))
		args = append(args, pq.Array(req.Children))
		argCount++
	}

	query += ` WHERE id = $` + string(rune(argCount+48)) + ` RETURNING id, name, role, birth, location, avatar, bio, children, created_at, updated_at`
	args = append(args, id)

	var p models.Person
	var children pq.StringArray
	err = h.db.QueryRow(query, args...).Scan(
		&p.ID, &p.Name, &p.Role, &p.Birth, &p.Location,
		&p.Avatar, &p.Bio, &children, &p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update person"})
		return
	}

	p.Children = children
	c.JSON(http.StatusOK, p)
}

// DeletePerson deletes a person from the tree
func (h *TreeHandler) DeletePerson(c *gin.Context) {
	id := c.Param("id")

	result, err := h.db.Exec(`DELETE FROM people WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete person"})
		return
	}

	rows, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Person not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Person deleted successfully"})
}
