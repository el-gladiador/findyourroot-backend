package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
	"github.com/mamiri/findyourroot/internal/utils"
)

// SSEHandler handles Server-Sent Events for real-time updates
type SSEHandler struct {
	client      *firestore.Client
	adminClients map[string]chan SSEMessage
	mu          sync.RWMutex
}

// SSEMessage represents a message to be sent via SSE
type SSEMessage struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(client *firestore.Client) *SSEHandler {
	handler := &SSEHandler{
		client:       client,
		adminClients: make(map[string]chan SSEMessage),
	}
	
	// Start watching collections
	go handler.watchCollections()
	
	return handler
}

// AdminStream handles SSE connections for admin users
func (h *SSEHandler) AdminStream(c *gin.Context) {
	// Get token from query parameter (EventSource doesn't support headers)
	token := c.Query("token")
	if token == "" {
		// Also try Authorization header for testing
		token = c.GetHeader("Authorization")
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
	}
	
	if token == "" {
		c.JSON(401, gin.H{"error": "Token required"})
		return
	}
	
	// Validate token
	claims, err := utils.ValidateJWTToken(token)
	if err != nil {
		c.JSON(401, gin.H{"error": "Invalid token"})
		return
	}
	
	userID := claims.UserID
	role := claims.Role
	
	// Only admin and co-admin can access admin stream
	if role != "admin" && role != "co-admin" {
		c.JSON(403, gin.H{"error": "Admin access required"})
		return
	}
	
	clientID := fmt.Sprintf("%s-%d", userID, time.Now().UnixNano())
	
	// Set headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")
	
	// Create channel for this client
	messageChan := make(chan SSEMessage, 100)
	
	// Register client
	h.mu.Lock()
	h.adminClients[clientID] = messageChan
	h.mu.Unlock()
	
	log.Printf("[SSE] Admin client connected: %s (role: %s)", clientID, role)
	
	// Send initial connection message
	c.SSEvent("connected", gin.H{
		"message":  "Connected to admin stream",
		"clientId": clientID,
	})
	c.Writer.Flush()
	
	// Send initial data
	h.sendInitialAdminData(c, messageChan)
	
	// Create context that cancels when client disconnects
	ctx := c.Request.Context()
	
	// Keep connection alive and send messages
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			h.mu.Lock()
			delete(h.adminClients, clientID)
			close(messageChan)
			h.mu.Unlock()
			log.Printf("[SSE] Admin client disconnected: %s", clientID)
			return
			
		case msg := <-messageChan:
			// Send message to client
			data, _ := json.Marshal(msg.Data)
			c.SSEvent(msg.Event, string(data))
			c.Writer.Flush()
			
		case <-ticker.C:
			// Send keepalive ping
			c.SSEvent("ping", gin.H{"time": time.Now().Unix()})
			c.Writer.Flush()
		}
	}
}

// sendInitialAdminData sends the current state of all admin collections
func (h *SSEHandler) sendInitialAdminData(c *gin.Context, messageChan chan SSEMessage) {
	ctx := context.Background()
	
	// Fetch and send suggestions
	suggestions, err := h.fetchCollection(ctx, "suggestions", "pending")
	if err == nil {
		data, _ := json.Marshal(gin.H{"items": suggestions, "collection": "suggestions"})
		c.SSEvent("suggestions", string(data))
		c.Writer.Flush()
	}
	
	// Fetch and send permission requests
	permRequests, err := h.fetchCollection(ctx, "permission_requests", "pending")
	if err == nil {
		data, _ := json.Marshal(gin.H{"items": permRequests, "collection": "permission_requests"})
		c.SSEvent("permission_requests", string(data))
		c.Writer.Flush()
	}
	
	// Fetch and send identity claims
	identityClaims, err := h.fetchCollection(ctx, "identity_claims", "pending")
	if err == nil {
		data, _ := json.Marshal(gin.H{"items": identityClaims, "collection": "identity_claims"})
		c.SSEvent("identity_claims", string(data))
		c.Writer.Flush()
	}
}

// fetchCollection fetches documents from a collection with optional status filter
func (h *SSEHandler) fetchCollection(ctx context.Context, collectionName, status string) ([]map[string]interface{}, error) {
	var docs []*firestore.DocumentSnapshot
	var err error
	
	if status != "" {
		docs, err = h.client.Collection(collectionName).Where("status", "==", status).Documents(ctx).GetAll()
	} else {
		docs, err = h.client.Collection(collectionName).Documents(ctx).GetAll()
	}
	
	if err != nil {
		return nil, err
	}
	
	results := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		data := doc.Data()
		data["id"] = doc.Ref.ID
		results = append(results, data)
	}
	
	return results, nil
}

// watchCollections starts Firestore snapshot listeners for admin collections
func (h *SSEHandler) watchCollections() {
	ctx := context.Background()
	
	// Watch suggestions collection
	go h.watchCollection(ctx, "suggestions")
	
	// Watch permission_requests collection
	go h.watchCollection(ctx, "permission_requests")
	
	// Watch identity_claims collection
	go h.watchCollection(ctx, "identity_claims")
	
	log.Println("[SSE] Started watching Firestore collections")
}

// watchCollection watches a single collection for changes
func (h *SSEHandler) watchCollection(ctx context.Context, collectionName string) {
	// Watch for pending items
	snapIter := h.client.Collection(collectionName).
		Where("status", "==", "pending").
		Snapshots(ctx)
	
	defer snapIter.Stop()
	
	for {
		snap, err := snapIter.Next()
		if err == io.EOF {
			log.Printf("[SSE] Snapshot iterator for %s ended", collectionName)
			return
		}
		if err != nil {
			log.Printf("[SSE] Error watching %s: %v", collectionName, err)
			// Wait and retry
			time.Sleep(5 * time.Second)
			continue
		}
		
		// Process changes
		for _, change := range snap.Changes {
			var eventType string
			switch change.Kind {
			case firestore.DocumentAdded:
				eventType = "added"
			case firestore.DocumentModified:
				eventType = "modified"
			case firestore.DocumentRemoved:
				eventType = "removed"
			}
			
			data := change.Doc.Data()
			data["id"] = change.Doc.Ref.ID
			
			message := SSEMessage{
				Event: collectionName,
				Data: gin.H{
					"type":       eventType,
					"item":       data,
					"collection": collectionName,
				},
			}
			
			log.Printf("[SSE] %s change in %s: %s", eventType, collectionName, change.Doc.Ref.ID)
			
			// Broadcast to all admin clients
			h.broadcastToAdmins(message)
		}
	}
}

// broadcastToAdmins sends a message to all connected admin clients
func (h *SSEHandler) broadcastToAdmins(msg SSEMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	for clientID, ch := range h.adminClients {
		select {
		case ch <- msg:
			// Message sent
		default:
			// Channel full, skip this message
			log.Printf("[SSE] Channel full for client %s, skipping message", clientID)
		}
	}
}
