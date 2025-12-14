package models

import "time"

// User represents a user in the system
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	IsAdmin      bool      `json:"is_admin"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
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
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LoginRequest represents login credentials
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
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
