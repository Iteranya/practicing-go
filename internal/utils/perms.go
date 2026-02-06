package utils

import (
	"sort"
	"strings"
)

// --- Constants ---

const (
	// Superuser
	InventoryAdmin = "inventory:*"
	OrderAdmin     = "order:*"
	ProductAdmin   = "product:*"
	UserAdmin      = "user:*"
	RoleAdmin      = "role:*"
	// Inventory
	PermInventoryCreate = "inventory:create"
	PermInventoryRead   = "inventory:read"
	PermInventoryUpdate = "inventory:update"
	PermInventoryDelete = "inventory:delete"

	// Order
	PermOrderCreate = "order:create"
	PermOrderRead   = "order:read"
	PermOrderUpdate = "order:update"
	PermOrderDelete = "order:delete"

	// Product
	PermProductCreate = "product:create"
	PermProductRead   = "product:read"
	PermProductUpdate = "product:update"
	PermProductDelete = "product:delete"

	// User
	PermUserCreate = "user:create"
	PermUserRead   = "user:read"
	PermUserUpdate = "user:update"
	PermUserDelete = "user:delete"

	// Role
	PermRoleCreate = "role:create"
	PermRoleRead   = "role:read"
	PermRoleUpdate = "role:update"
	PermRoleDelete = "role:delete"
)

// --- Validation Map ---

// validPermissions is a set used to validate input strings quickly.
var validPermissions = map[string]struct{}{
	// Inventory
	PermInventoryCreate: {},
	PermInventoryRead:   {},
	PermInventoryUpdate: {},
	PermInventoryDelete: {},

	// Order
	PermOrderCreate: {},
	PermOrderRead:   {},
	PermOrderUpdate: {},
	PermOrderDelete: {},

	// Product
	PermProductCreate: {},
	PermProductRead:   {},
	PermProductUpdate: {},
	PermProductDelete: {},

	// User
	PermUserCreate: {},
	PermUserRead:   {},
	PermUserUpdate: {},
	PermUserDelete: {},

	// Role
	PermRoleCreate: {},
	PermRoleRead:   {},
	PermRoleUpdate: {},
	PermRoleDelete: {},
}

// --- Functions ---

// IsValidPermission checks if a string matches one of the defined permission constants.
func IsValidPermission(perm string) bool {
	_, ok := validPermissions[perm]
	return ok
}

// GetAllPermissions returns a sorted slice of all available permission strings.
// Useful for UI dropdowns or listing capabilities.
func GetAllPermissions() []string {
	perms := make([]string, 0, len(validPermissions))
	for k := range validPermissions {
		perms = append(perms, k)
	}
	sort.Strings(perms)
	return perms
}

// HasPermission checks if a list of user permissions contains the required one.
func HasPermission(userPerms []string, requiredPerm string) bool {
	for _, p := range userPerms {
		if p == requiredPerm {
			return true
		}
		// Future proofing: If you ever decide to use wildcards like "inventory:*"
		if strings.HasSuffix(p, ":*") {
			prefix := strings.TrimSuffix(p, ":*")
			if strings.HasPrefix(requiredPerm, prefix+":") {
				return true
			}
		}
	}
	return false
}

type ContextKey string

const (
	UserIDKey ContextKey = "userID"   // Holds the int ID of the logged in user
	RoleKey   ContextKey = "userRole" // Holds the string slug of the user's role
)
