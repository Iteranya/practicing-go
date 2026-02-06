package role

import (
	"context"
	"slices"
)

type RoleService interface {
	// Standard CRUD
	CreateRole(ctx context.Context, role Role) (*Role, error)
	GetRole(ctx context.Context, idOrSlug any) (*Role, error)
	UpdateRole(ctx context.Context, id int, role Role) error
	DeleteRole(ctx context.Context, id int) error
	ListRoles(ctx context.Context) ([]*Role, error)

	// Permission Granular Management
	UpdatePermissions(ctx context.Context, id int, permissions []string) error
	AddPermission(ctx context.Context, id int, permission string) error
	RemovePermission(ctx context.Context, id int, permission string) error

	// Auth Helper
	// Fetches all roles and converts them to a map of Slug -> Permissions
	// Used by the Authorization Middleware to check User access against DB rules.
	GetPolicyMap(ctx context.Context) (map[string][]string, error)
}

type roleService struct {
	repo RoleRepository
}

func NewRoleService(repo RoleRepository) RoleService {
	return &roleService{repo: repo}
}

// --- CRUD ---

func (s *roleService) CreateRole(ctx context.Context, role Role) (*Role, error) {
	if role.Slug == "" || role.Name == "" {
		return nil, ErrInvalidRoleInput
	}

	if role.Permissions == nil {
		role.Permissions = []string{}
	}

	err := s.repo.Create(ctx, &role)
	if err != nil {
		return nil, err
	}

	return &role, nil
}

func (s *roleService) GetRole(ctx context.Context, idOrSlug any) (*Role, error) {
	switch v := idOrSlug.(type) {
	case int:
		return s.repo.GetByID(ctx, v)
	case string:
		return s.repo.GetBySlug(ctx, v)
	default:
		return nil, ErrInvalidRoleInput
	}
}

func (s *roleService) UpdateRole(ctx context.Context, id int, role Role) error {
	if id == 0 {
		return ErrInvalidRoleInput
	}

	// Fetch existing to ensure it exists and preserve ID
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	role.Id = existing.Id

	// If permissions are nil in update, preserve existing ones
	if role.Permissions == nil {
		role.Permissions = existing.Permissions
	}

	return s.repo.Update(ctx, &role)
}

func (s *roleService) DeleteRole(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

func (s *roleService) ListRoles(ctx context.Context) ([]*Role, error) {
	return s.repo.List(ctx)
}

// --- Permission Management ---

func (s *roleService) UpdatePermissions(ctx context.Context, id int, permissions []string) error {
	role, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	role.Permissions = permissions
	return s.repo.Update(ctx, role)
}

func (s *roleService) AddPermission(ctx context.Context, id int, permission string) error {
	role, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Check if already exists to avoid duplicates
	if !slices.Contains(role.Permissions, permission) {
		role.Permissions = append(role.Permissions, permission)
		return s.repo.Update(ctx, role)
	}

	return nil // Already exists, no-op works fine
}

func (s *roleService) RemovePermission(ctx context.Context, id int, permission string) error {
	role, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Filter out the permission
	newPerms := []string{}
	for _, p := range role.Permissions {
		if p != permission {
			newPerms = append(newPerms, p)
		}
	}

	// Only update if something actually changed
	if len(newPerms) != len(role.Permissions) {
		role.Permissions = newPerms
		return s.repo.Update(ctx, role)
	}

	return nil
}

// --- Auth Helper ---

func (s *roleService) GetPolicyMap(ctx context.Context) (map[string][]string, error) {
	roles, err := s.repo.List(ctx)
	if err != nil {
		// Return empty map on error to define "deny all" behavior implicitly,
		// or handle error in caller
		return nil, err
	}

	policy := make(map[string][]string)
	for _, r := range roles {
		// Map the Role Slug (stored in User) to the Permission List (stored in Role)
		policy[r.Slug] = r.Permissions
	}

	return policy, nil
}
