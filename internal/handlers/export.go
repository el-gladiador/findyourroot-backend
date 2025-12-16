package handlers

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
	"github.com/mamiri/findyourroot/internal/models"
	"google.golang.org/api/iterator"
)

// FirestoreExportHandler handles export operations
type FirestoreExportHandler struct {
	client *firestore.Client
}

// NewFirestoreExportHandler creates a new export handler
func NewFirestoreExportHandler(client *firestore.Client) *FirestoreExportHandler {
	return &FirestoreExportHandler{client: client}
}

// ExportJSON exports tree data as JSON
func (h *FirestoreExportHandler) ExportJSON(c *gin.Context) {
	people, err := h.getAllPeople(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Create export-friendly format (without internal fields)
	type ExportPerson struct {
		ID       string   `json:"id"`
		Name     string   `json:"name"`
		Role     string   `json:"role"`
		Birth    string   `json:"birth"`
		Location string   `json:"location"`
		Avatar   string   `json:"avatar"`
		Bio      string   `json:"bio"`
		Children []string `json:"children"`
	}

	exportData := make([]ExportPerson, len(people))
	for i, p := range people {
		exportData[i] = ExportPerson{
			ID:       p.ID,
			Name:     p.Name,
			Role:     p.Role,
			Birth:    p.Birth,
			Location: p.Location,
			Avatar:   p.Avatar,
			Bio:      p.Bio,
			Children: p.Children,
		}
	}

	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate JSON"})
		return
	}

	filename := fmt.Sprintf("family-tree-%s.json", time.Now().Format("2006-01-02"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/json")
	c.Data(http.StatusOK, "application/json", jsonData)
}

// ExportCSV exports tree data as CSV
func (h *FirestoreExportHandler) ExportCSV(c *gin.Context) {
	people, err := h.getAllPeople(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"ID", "Name", "Role", "Birth Year", "Location", "Bio", "Avatar URL"}
	if err := writer.Write(header); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write CSV header"})
		return
	}

	// Write data rows
	for _, person := range people {
		row := []string{
			person.ID,
			person.Name,
			person.Role,
			person.Birth,
			person.Location,
			person.Bio,
			person.Avatar,
		}
		if err := writer.Write(row); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write CSV row"})
			return
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate CSV"})
		return
	}

	filename := fmt.Sprintf("family-tree-%s.csv", time.Now().Format("2006-01-02"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv")
	c.Data(http.StatusOK, "text/csv", buf.Bytes())
}

// ExportText exports tree data as plain text (for PDF-like output)
func (h *FirestoreExportHandler) ExportText(c *gin.Context) {
	people, err := h.getAllPeople(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var buf bytes.Buffer
	buf.WriteString("FAMILY TREE EXPORT\n")
	buf.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().Format("January 2, 2006")))
	buf.WriteString("================================\n\n")

	for _, person := range people {
		buf.WriteString(fmt.Sprintf("%s (%s)\n", person.Name, person.Role))
		buf.WriteString(fmt.Sprintf("  Born: %s\n", person.Birth))
		buf.WriteString(fmt.Sprintf("  Location: %s\n", person.Location))
		if person.Bio != "" {
			buf.WriteString(fmt.Sprintf("  About: %s\n", person.Bio))
		}
		buf.WriteString("\n")
	}

	filename := fmt.Sprintf("family-tree-%s.txt", time.Now().Format("2006-01-02"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/plain")
	c.Data(http.StatusOK, "text/plain", buf.Bytes())
}

// getAllPeople fetches all people from Firestore
func (h *FirestoreExportHandler) getAllPeople(c *gin.Context) ([]models.Person, error) {
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
			return nil, fmt.Errorf("failed to fetch people: %v", err)
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

	return people, nil
}
