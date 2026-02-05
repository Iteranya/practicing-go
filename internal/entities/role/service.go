package main

import (
	"context"
	"slices"
)

type RoleService interface {
	CreateRole(ctx context.Context, role Role) (*Role, error)
	GetRole(ctx context.Context, idOrSlug any) (*Role, error)
	UpdateRole(ctx context.Context, id int, role Role) error
	DeleteRole(ctx context.Context, id int) error
	ListRoles(ctx context.Context) ([]*Role, error)

	// Permission Management
	UpdatePermissions(ctx context.Context, id int, permissions []string) error
	AddPermission(ctx context.Context, id int, permission string) error
	RemovePermission(ctx context.Context, id int, permission string) error
}

type roleService struct {
	repo RoleRepository
}

func NewRoleService(repo RoleRepository) RoleService {
	return &roleService{repo: repo}
}

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

	// If permissions are nil in update, keep existing ones?
	// Or assume empty list means clear?
	// Usually, for a full Update (PUT), nil means empty.
	// But let's be safe: if nil, preserve existing.
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

	// Check if exists
	if !slices.Contains(role.Permissions, permission) {
		role.Permissions = append(role.Permissions, permission)
		return s.repo.Update(ctx, role)
	}

	return nil // Already exists, no-op
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

	// Only update if changed
	if len(newPerms) != len(role.Permissions) {
		role.Permissions = newPerms
		return s.repo.Update(ctx, role)
	}

	return nil
}
