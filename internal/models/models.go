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
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         UserRole  `json:"role"`
	IsAdmin      bool      `json:"is_admin"` // Deprecated, use Role instead
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
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

// Person represents a family tree member
type Person struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Birth     string    `json:"birth"`
	Location  string    `json:"location"`
	Avatar    string    `json:"avatar"`
	Bio       string    `json:"bio"`
	Children  []string  `json:"children"`
	CreatedBy string    `json:"created_by"` // User ID of creator
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RegisterRequest represents registration data
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
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
	Avatar   string   `json:"avatar" binding:"required"`
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
