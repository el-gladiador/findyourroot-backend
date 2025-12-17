package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mamiri/findyourroot/internal/models"
)

// Claims represents JWT claims
type Claims struct {
	UserID  string `json:"user_id"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"is_admin"`
	Role    string `json:"role"`
	jwt.RegisteredClaims
}

// AuthMiddleware validates JWT tokens
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Parse and validate token
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("is_admin", claims.IsAdmin)
		c.Set("role", claims.Role)
		c.Set("claims", claims)

		c.Next()
	}
}

// RequireContributor ensures user has at least contributor role (can make suggestions)
func RequireContributor() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		userClaims := claims.(*Claims)
		role := models.UserRole(userClaims.Role)

		// Contributors and above can make suggestions
		if role != models.RoleContributor && role != models.RoleEditor &&
			role != models.RoleCoAdmin && role != models.RoleAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Contributor access required", "required_role": "contributor"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireEditor ensures user has at least editor role (can edit directly)
func RequireEditor() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		userClaims := claims.(*Claims)
		role := models.UserRole(userClaims.Role)

		// Editor, co-admin, and admin can edit directly
		if !role.CanEditDirectly() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Editor or Admin access required", "required_role": "editor"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireApprover ensures user can approve/reject suggestions (co-admin or admin)
func RequireApprover() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		userClaims := claims.(*Claims)
		role := models.UserRole(userClaims.Role)

		if !role.CanApprove() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Co-Admin or Admin access required", "required_role": "co-admin"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAdmin ensures user has admin role (tree owner - can manage users)
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		userClaims := claims.(*Claims)
		role := models.UserRole(userClaims.Role)

		if !role.CanManageUsers() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required", "required_role": "admin"})
			c.Abort()
			return
		}

		c.Next()
	}
}
