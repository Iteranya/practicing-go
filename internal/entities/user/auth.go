package user

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	// Replace this with your actual module path
	"github.com/iteranya/practicing-go/internal/utils"
)

var (
	// In production, ensure this is set via environment variable
	jwtSecret = []byte(getEnv("JWT_SECRET", "super-secret-dev-key"))
	tokenTTL  = 24 * time.Hour
)

// Claims defines the payload inside our signed JWT
type Claims struct {
	UserID int    `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// ---------------------------------------------------------
// DOMAIN METHODS (Attached to the User Struct)
// ---------------------------------------------------------

// SetPassword hashes the raw password using bcrypt and updates the user's Hash field.
func (u *User) SetPassword(rawPassword string) error {
	bytes, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Hash = string(bytes)
	return nil
}

// CheckPassword compares the provided raw password with the user's stored hash.
func (u *User) CheckPassword(rawPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(rawPassword))
	return err == nil
}

// Can checks if this user is allowed to perform a specific action.
//
// How it works:
// 1. The User struct holds the Role Slug (e.g., "manager").
// 2. The 'policy' map (fetched from DB) maps that Slug to a list of Permissions.
// 3. We check if 'requiredPerm' exists in that list.
func (u *User) Can(requiredPerm string, policy map[string][]string) bool {
	// 1. Retrieve the permissions array for this user's role
	myPerms, exists := policy[u.Role]
	if !exists {
		return false // Role doesn't exist in the system policy = Deny Access
	}

	// 2. Use Utils helper to check for matches (including wildcards if supported)
	return utils.HasPermission(myPerms, requiredPerm)
}

// ---------------------------------------------------------
// STATIC HELPERS (JWT Token Management)
// ---------------------------------------------------------

// GenerateToken creates a signed JWT for a specific user instance.
func GenerateToken(u *User) (string, error) {
	claims := Claims{
		UserID: u.Id,
		Role:   u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "inventory-system",
		},
	}

	// Sign the token with HS256 algorithm and our secret key
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ValidateToken parses a raw token string, verifies the signature, and returns the claims.
// This is primarily used by the AuthMiddleware in main.go.
func ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validating the algorithm is crucial to prevent downgrade attacks
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// ---------------------------------------------------------
// INTERNAL HELPERS
// ---------------------------------------------------------

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
