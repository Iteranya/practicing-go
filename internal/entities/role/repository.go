package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrRoleNotFound      = errors.New("role not found")
	ErrDuplicateRoleSlug = errors.New("role slug already exists")
	ErrInvalidRoleInput  = errors.New("invalid role input")
)

type RoleRepository interface {
	Create(ctx context.Context, role *Role) error
	GetByID(ctx context.Context, id int) (*Role, error)
	GetBySlug(ctx context.Context, slug string) (*Role, error)
	Update(ctx context.Context, role *Role) error
	Delete(ctx context.Context, id int) error
	List(ctx context.Context) ([]*Role, error)
}

type roleRepository struct {
	db *sql.DB
}

func NewRoleRepository(db *sql.DB) RoleRepository {
	return &roleRepository{db: db}
}

func (r *roleRepository) Create(ctx context.Context, role *Role) error {
	if role.Slug == "" || role.Name == "" {
		return ErrInvalidRoleInput
	}

	// Default to empty array if nil to store [] instead of null in DB
	if role.Permissions == nil {
		role.Permissions = []string{}
	}

	permsJSON, err := json.Marshal(role.Permissions)
	if err != nil {
		return fmt.Errorf("failed to marshal permissions: %w", err)
	}

	query := `
        INSERT INTO roles (slug, name, permissions)
        VALUES ($1, $2, $3)
        RETURNING id
    `

	err = r.db.QueryRowContext(
		ctx, query,
		role.Slug, role.Name, permsJSON,
	).Scan(&role.Id)

	if err != nil {
		// Note: Adapt this check based on your specific DB driver error
		if isDuplicateKeyError(err) {
			return ErrDuplicateRoleSlug
		}
		return fmt.Errorf("failed to create role: %w", err)
	}

	return nil
}

func (r *roleRepository) GetByID(ctx context.Context, id int) (*Role, error) {
	query := `
        SELECT id, slug, name, permissions
        FROM roles
        WHERE id = $1
    `

	role := &Role{}
	var permsJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&role.Id, &role.Slug, &role.Name, &permsJSON,
	)

	if err == sql.ErrNoRows {
		return nil, ErrRoleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	if len(permsJSON) > 0 {
		if err := json.Unmarshal(permsJSON, &role.Permissions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
		}
	}

	return role, nil
}

func (r *roleRepository) GetBySlug(ctx context.Context, slug string) (*Role, error) {
	query := `
        SELECT id, slug, name, permissions
        FROM roles
        WHERE slug = $1
    `

	role := &Role{}
	var permsJSON []byte

	err := r.db.QueryRowContext(ctx, query, slug).Scan(
		&role.Id, &role.Slug, &role.Name, &permsJSON,
	)

	if err == sql.ErrNoRows {
		return nil, ErrRoleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	if len(permsJSON) > 0 {
		if err := json.Unmarshal(permsJSON, &role.Permissions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
		}
	}

	return role, nil
}

func (r *roleRepository) Update(ctx context.Context, role *Role) error {
	if role.Id == 0 {
		return ErrInvalidRoleInput
	}

	permsJSON, err := json.Marshal(role.Permissions)
	if err != nil {
		return fmt.Errorf("failed to marshal permissions: %w", err)
	}

	query := `
        UPDATE roles
        SET slug = $1, name = $2, permissions = $3
        WHERE id = $4
    `

	result, err := r.db.ExecContext(
		ctx, query,
		role.Slug, role.Name, permsJSON, role.Id,
	)

	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrDuplicateRoleSlug
		}
		return fmt.Errorf("failed to update role: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrRoleNotFound
	}

	return nil
}

func (r *roleRepository) Delete(ctx context.Context, id int) error {
	// Optional: Check if any users are assigned this role before deleting
	// or rely on Foreign Key constraints in the DB (ON DELETE RESTRICT)

	query := `DELETE FROM roles WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrRoleNotFound
	}

	return nil
}

func (r *roleRepository) List(ctx context.Context) ([]*Role, error) {
	query := `
        SELECT id, slug, name, permissions
        FROM roles
        ORDER BY name ASC
    `

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}
	defer rows.Close()

	var roles []*Role
	for rows.Next() {
		role := &Role{}
		var permsJSON []byte

		err := rows.Scan(&role.Id, &role.Slug, &role.Name, &permsJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}

		if len(permsJSON) > 0 {
			if err := json.Unmarshal(permsJSON, &role.Permissions); err != nil {
				return nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
			}
		}

		roles = append(roles, role)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return roles, nil
}
func isDuplicateKeyError(_ error) bool {
	return false
}
