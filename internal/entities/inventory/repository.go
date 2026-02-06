package inventory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrNotFound      = errors.New("inventory not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrDuplicateSlug = errors.New("slug already exists")
)

type InventoryRepository interface {
	Create(ctx context.Context, inv *Inventory) error
	GetByID(ctx context.Context, id int) (*Inventory, error)
	GetBySlug(ctx context.Context, slug string) (*Inventory, error)
	Update(ctx context.Context, inv *Inventory) error
	Delete(ctx context.Context, id int) error
	List(ctx context.Context, opts ListOptions) ([]*Inventory, error)
	UpdateStock(ctx context.Context, id int, delta int64) error
	Search(ctx context.Context, query string) ([]*Inventory, error)
}

type ListOptions struct {
	Tag    string
	Label  string
	Limit  int
	Offset int
}

type inventoryRepository struct {
	db *sql.DB
}

func NewInventoryRepository(db *sql.DB) InventoryRepository {
	return &inventoryRepository{db: db}
}

// CREATE
func (r *inventoryRepository) Create(ctx context.Context, inv *Inventory) error {
	if inv.Slug == "" || inv.Name == "" {
		return ErrInvalidInput
	}

	customJSON, err := json.Marshal(inv.Custom)
	if err != nil {
		return fmt.Errorf("failed to marshal custom data: %w", err)
	}

	query := `
		INSERT INTO inventory (slug, name, desc, tag, label, stock, custom)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	err = r.db.QueryRowContext(
		ctx, query,
		inv.Slug, inv.Name, inv.Desc, inv.Tag, inv.Label, inv.Stock, customJSON,
	).Scan(&inv.Id)

	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrDuplicateSlug
		}
		return fmt.Errorf("failed to create inventory: %w", err)
	}

	return nil
}

// READ BY ID
func (r *inventoryRepository) GetByID(ctx context.Context, id int) (*Inventory, error) {
	query := `
		SELECT id, slug, name, desc, tag, label, stock, custom
		FROM inventory
		WHERE id = $1
	`

	inv := &Inventory{}
	var customJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&inv.Id, &inv.Slug, &inv.Name, &inv.Desc,
		&inv.Tag, &inv.Label, &inv.Stock, &customJSON,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}

	if len(customJSON) > 0 {
		if err := json.Unmarshal(customJSON, &inv.Custom); err != nil {
			return nil, fmt.Errorf("failed to unmarshal custom data: %w", err)
		}
	}

	return inv, nil
}

// READ BY SLUG
func (r *inventoryRepository) GetBySlug(ctx context.Context, slug string) (*Inventory, error) {
	query := `
		SELECT id, slug, name, desc, tag, label, stock, custom
		FROM inventory
		WHERE slug = $1
	`

	inv := &Inventory{}
	var customJSON []byte

	err := r.db.QueryRowContext(ctx, query, slug).Scan(
		&inv.Id, &inv.Slug, &inv.Name, &inv.Desc,
		&inv.Tag, &inv.Label, &inv.Stock, &customJSON,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}

	if len(customJSON) > 0 {
		if err := json.Unmarshal(customJSON, &inv.Custom); err != nil {
			return nil, fmt.Errorf("failed to unmarshal custom data: %w", err)
		}
	}

	return inv, nil
}

// UPDATE
func (r *inventoryRepository) Update(ctx context.Context, inv *Inventory) error {
	if inv.Id == 0 {
		return ErrInvalidInput
	}

	customJSON, err := json.Marshal(inv.Custom)
	if err != nil {
		return fmt.Errorf("failed to marshal custom data: %w", err)
	}

	query := `
		UPDATE inventory
		SET slug = $1, name = $2, desc = $3, tag = $4, label = $5, stock = $6, custom = $7
		WHERE id = $8
	`

	result, err := r.db.ExecContext(
		ctx, query,
		inv.Slug, inv.Name, inv.Desc, inv.Tag, inv.Label, inv.Stock, customJSON, inv.Id,
	)

	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrDuplicateSlug
		}
		return fmt.Errorf("failed to update inventory: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// DELETE
func (r *inventoryRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM inventory WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete inventory: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// READ ALL
func (r *inventoryRepository) List(ctx context.Context, opts ListOptions) ([]*Inventory, error) {
	query := `
		SELECT id, slug, name, desc, tag, label, stock, custom
		FROM inventory
		WHERE 1=1
	`
	args := []any{}
	argPos := 1

	if opts.Tag != "" {
		query += fmt.Sprintf(" AND tag = $%d", argPos)
		args = append(args, opts.Tag)
		argPos++
	}

	if opts.Label != "" {
		query += fmt.Sprintf(" AND label = $%d", argPos)
		args = append(args, opts.Label)
		argPos++
	}

	query += " ORDER BY id"

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, opts.Limit)
		argPos++
	}

	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, opts.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list inventory: %w", err)
	}
	defer rows.Close()

	var items []*Inventory
	for rows.Next() {
		inv := &Inventory{}
		var customJSON []byte

		err := rows.Scan(
			&inv.Id, &inv.Slug, &inv.Name, &inv.Desc,
			&inv.Tag, &inv.Label, &inv.Stock, &customJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan inventory: %w", err)
		}

		if len(customJSON) > 0 {
			if err := json.Unmarshal(customJSON, &inv.Custom); err != nil {
				return nil, fmt.Errorf("failed to unmarshal custom data: %w", err)
			}
		}

		items = append(items, inv)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return items, nil
}

// UPDATE STOCK
func (r *inventoryRepository) UpdateStock(ctx context.Context, id int, delta int64) error {
	query := `
		UPDATE inventory
		SET stock = stock + $1
		WHERE id = $2
		RETURNING stock
	`

	var newStock int64
	err := r.db.QueryRowContext(ctx, query, delta, id).Scan(&newStock)

	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to update stock: %w", err)
	}

	return nil
}

// SEARCH
func (r *inventoryRepository) Search(ctx context.Context, query string) ([]*Inventory, error) {
	searchQuery := `
		SELECT id, slug, name, desc, tag, label, stock, custom
		FROM inventory
		WHERE name ILIKE $1 OR desc ILIKE $1 OR tag ILIKE $1
		ORDER BY name
	`

	searchPattern := "%" + query + "%"
	rows, err := r.db.QueryContext(ctx, searchQuery, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search inventory: %w", err)
	}
	defer rows.Close()

	var items []*Inventory
	for rows.Next() {
		inv := &Inventory{}
		var customJSON []byte

		err := rows.Scan(
			&inv.Id, &inv.Slug, &inv.Name, &inv.Desc,
			&inv.Tag, &inv.Label, &inv.Stock, &customJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan inventory: %w", err)
		}

		if len(customJSON) > 0 {
			if err := json.Unmarshal(customJSON, &inv.Custom); err != nil {
				return nil, fmt.Errorf("failed to unmarshal custom data: %w", err)
			}
		}

		items = append(items, inv)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return items, nil
}

func isDuplicateKeyError(err error) bool {
	return false
}
