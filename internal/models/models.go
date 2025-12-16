package models

import "time"

// UserRole represents user permission levels
type UserRole string

const (
	RoleViewer UserRole = "viewer" // Can only view the tree
	RoleEditor UserRole = "editor" // Can add/edit/delete in the tree
	RoleAdmin  UserRole = "admin"  // Full access + user management
)

// User represents a user in the system
type User struct {
	ID           string    `json:"id" firestore:"id"`
	Email        string    `json:"email" firestore:"email"`
	PasswordHash string    `json:"-" firestore:"password_hash"`
	Role         UserRole  `json:"role" firestore:"role"`
	IsAdmin      bool      `json:"is_admin" firestore:"is_admin"`       // Deprecated, use Role instead
	TreeName     string    `json:"tree_name" firestore:"tree_name"`     // Family tree name (e.g., "Batur")
	FatherName   string    `json:"father_name" firestore:"father_name"` // Father's name for verification
	BirthYear    string    `json:"birth_year" firestore:"birth_year"`   // Birth year for verification
	IsVerified   bool      `json:"is_verified" firestore:"is_verified"` // Whether user is verified as part of the tree
	PersonID     string    `json:"person_id" firestore:"person_id"`     // Linked tree node ID (if user claimed identity)
	CreatedAt    time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" firestore:"updated_at"`
}

// PermissionRequest represents a request for elevated permissions
type PermissionRequest struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	UserEmail     string    `json:"user_email"`
	RequestedRole UserRole  `json:"requested_role"`
	Message       string    `json:"message"`
	Status        string    `json:"status"` // pending, approved, rejected
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// IdentityClaimRequest represents a request to claim a tree node as oneself
type IdentityClaimRequest struct {
	ID          string    `json:"id" firestore:"id"`
	UserID      string    `json:"user_id" firestore:"user_id"`
	UserEmail   string    `json:"user_email" firestore:"user_email"`
	PersonID    string    `json:"person_id" firestore:"person_id"`       // The tree node they claim to be
	PersonName  string    `json:"person_name" firestore:"person_name"`   // Name of the person for display
	Message     string    `json:"message" firestore:"message"`           // Why they believe this is them
	Status      string    `json:"status" firestore:"status"`             // pending, approved, rejected
	ReviewedBy  string    `json:"reviewed_by" firestore:"reviewed_by"`   // Admin who reviewed
	ReviewNotes string    `json:"review_notes" firestore:"review_notes"` // Admin's notes
	CreatedAt   time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" firestore:"updated_at"`
}

// Person represents a family tree member
type Person struct {
	ID           string    `json:"id" firestore:"id"`
	Name         string    `json:"name" firestore:"name"`
	Role         string    `json:"role" firestore:"role"`
	Birth        string    `json:"birth" firestore:"birth"`
	Location     string    `json:"location" firestore:"location"`
	Avatar       string    `json:"avatar" firestore:"avatar"`
	Bio          string    `json:"bio" firestore:"bio"`
	Children     []string  `json:"children" firestore:"children"`
	CreatedBy    string    `json:"created_by" firestore:"created_by"`         // User ID of creator
	LinkedUserID string    `json:"linked_user_id" firestore:"linked_user_id"` // User ID if someone claimed this identity
	CreatedAt    time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" firestore:"updated_at"`
}

// RegisterRequest represents registration data
type RegisterRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=6"`
	TreeName   string `json:"tree_name" binding:"required"` // Must be "Batur" for now
	FatherName string `json:"father_name" binding:"required"`
	BirthYear  string `json:"birth_year" binding:"required"`
}

// LoginRequest represents login credentials
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// PermissionRequestRequest represents a request to elevate permissions
type PermissionRequestRequest struct {
	RequestedRole UserRole `json:"requested_role" binding:"required"`
	Message       string   `json:"message"`
}

// PermissionRequestResponse represents the response to a permission request
type PermissionRequestResponse struct {
	Id            string    `json:"id"`
	UserId        string    `json:"user_id"`
	UserEmail     string    `json:"user_email"`
	RequestedRole string    `json:"requested_role"`
	Message       string    `json:"message"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// LoginResponse represents login response with JWT token
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// CreatePersonRequest represents a request to create a person
type CreatePersonRequest struct {
	Name     string   `json:"name" binding:"required"`
	Role     string   `json:"role" binding:"required"`
	Birth    string   `json:"birth" binding:"required"`
	Location string   `json:"location" binding:"required"`
	Avatar   string   `json:"avatar"` // Optional - backend generates default if empty
	Bio      string   `json:"bio"`
	Children []string `json:"children"`
	ParentID *string  `json:"parent_id"` // Optional parent ID - backend will handle the relationship
}

// UpdatePersonRequest represents a request to update a person
type UpdatePersonRequest struct {
	Name     *string  `json:"name"`
	Role     *string  `json:"role"`
	Birth    *string  `json:"birth"`
	Location *string  `json:"location"`
	Avatar   *string  `json:"avatar"`
	Bio      *string  `json:"bio"`
	Children []string `json:"children"`
}

// ClaimIdentityRequest represents a user's request to claim a tree node
type ClaimIdentityRequest struct {
	PersonID string `json:"person_id" binding:"required"` // The tree node ID they claim to be
	Message  string `json:"message"`                      // Why they believe this is them
}

// ReviewClaimRequest represents admin's review of an identity claim
type ReviewClaimRequest struct {
	Approved    bool   `json:"approved"`
	ReviewNotes string `json:"review_notes"`
}
