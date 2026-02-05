package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrProductNotFound      = errors.New("product not found")
	ErrInvalidProductInput  = errors.New("invalid product input")
	ErrDuplicateProductSlug = errors.New("product slug already exists")
)

type ProductRepository interface {
	Create(ctx context.Context, product *Product) error
	GetByID(ctx context.Context, id int) (*Product, error)
	GetBySlug(ctx context.Context, slug string) (*Product, error)
	Update(ctx context.Context, product *Product) error
	Delete(ctx context.Context, id int) error
	List(ctx context.Context, opts ProductListOptions) ([]*Product, error)
	SetAvailability(ctx context.Context, id int, avail bool) error
	GetAvailable(ctx context.Context) ([]*Product, error)
	GetByTag(ctx context.Context, tag string) ([]*Product, error)
	GetByLabel(ctx context.Context, label string) ([]*Product, error)
	GetBundles(ctx context.Context) ([]*Product, error)    // Products that contain other products
	GetWithRecipe(ctx context.Context) ([]*Product, error) // Products that use inventory
	Search(ctx context.Context, query string) ([]*Product, error)
	UpdatePrice(ctx context.Context, id int, price int64) error
	GetByPriceRange(ctx context.Context, minPrice, maxPrice int64) ([]*Product, error)
}

type ProductListOptions struct {
	Tag       string
	Label     string
	Avail     *bool // pointer so we can distinguish between false and not set
	MinPrice  int64
	MaxPrice  int64
	Limit     int
	Offset    int
	SortBy    string // name, price, slug
	SortOrder string // asc, desc
}

type productRepository struct {
	db *sql.DB
}

func NewProductRepository(db *sql.DB) ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) Create(ctx context.Context, product *Product) error {
	if product.Slug == "" || product.Name == "" {
		return ErrInvalidProductInput
	}

	itemsJSON, err := r.marshalNullableSlice(product.Items)
	if err != nil {
		return fmt.Errorf("failed to marshal items: %w", err)
	}

	recipeJSON, err := r.marshalNullableMap(product.Recipe)
	if err != nil {
		return fmt.Errorf("failed to marshal recipe: %w", err)
	}

	customJSON, err := json.Marshal(product.Custom)
	if err != nil {
		return fmt.Errorf("failed to marshal custom data: %w", err)
	}

	query := `
		INSERT INTO products (slug, name, desc, tag, label, price, avail, items, recipe, custom)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`

	err = r.db.QueryRowContext(
		ctx, query,
		product.Slug, product.Name, product.Desc, product.Tag, product.Label,
		product.Price, product.Avail, itemsJSON, recipeJSON, customJSON,
	).Scan(&product.Id)

	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrDuplicateProductSlug
		}
		return fmt.Errorf("failed to create product: %w", err)
	}

	return nil
}

func (r *productRepository) GetByID(ctx context.Context, id int) (*Product, error) {
	query := `
		SELECT id, slug, name, desc, tag, label, price, avail, items, recipe, custom
		FROM products
		WHERE id = $1
	`

	product := &Product{}
	var itemsJSON, recipeJSON, customJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&product.Id, &product.Slug, &product.Name, &product.Desc,
		&product.Tag, &product.Label, &product.Price, &product.Avail,
		&itemsJSON, &recipeJSON, &customJSON,
	)

	if err == sql.ErrNoRows {
		return nil, ErrProductNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	if err := r.unmarshalProductData(product, itemsJSON, recipeJSON, customJSON); err != nil {
		return nil, err
	}

	return product, nil
}

func (r *productRepository) GetBySlug(ctx context.Context, slug string) (*Product, error) {
	query := `
		SELECT id, slug, name, desc, tag, label, price, avail, items, recipe, custom
		FROM products
		WHERE slug = $1
	`

	product := &Product{}
	var itemsJSON, recipeJSON, customJSON []byte

	err := r.db.QueryRowContext(ctx, query, slug).Scan(
		&product.Id, &product.Slug, &product.Name, &product.Desc,
		&product.Tag, &product.Label, &product.Price, &product.Avail,
		&itemsJSON, &recipeJSON, &customJSON,
	)

	if err == sql.ErrNoRows {
		return nil, ErrProductNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	if err := r.unmarshalProductData(product, itemsJSON, recipeJSON, customJSON); err != nil {
		return nil, err
	}

	return product, nil
}

func (r *productRepository) Update(ctx context.Context, product *Product) error {
	if product.Id == 0 {
		return ErrInvalidProductInput
	}

	itemsJSON, err := r.marshalNullableSlice(product.Items)
	if err != nil {
		return fmt.Errorf("failed to marshal items: %w", err)
	}

	recipeJSON, err := r.marshalNullableMap(product.Recipe)
	if err != nil {
		return fmt.Errorf("failed to marshal recipe: %w", err)
	}

	customJSON, err := json.Marshal(product.Custom)
	if err != nil {
		return fmt.Errorf("failed to marshal custom data: %w", err)
	}

	query := `
		UPDATE products
		SET slug = $1, name = $2, desc = $3, tag = $4, label = $5,
		    price = $6, avail = $7, items = $8, recipe = $9, custom = $10
		WHERE id = $11
	`

	result, err := r.db.ExecContext(
		ctx, query,
		product.Slug, product.Name, product.Desc, product.Tag, product.Label,
		product.Price, product.Avail, itemsJSON, recipeJSON, customJSON, product.Id,
	)

	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrDuplicateProductSlug
		}
		return fmt.Errorf("failed to update product: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrProductNotFound
	}

	return nil
}

func (r *productRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM products WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrProductNotFound
	}

	return nil
}

func (r *productRepository) List(ctx context.Context, opts ProductListOptions) ([]*Product, error) {
	query := `
		SELECT id, slug, name, desc, tag, label, price, avail, items, recipe, custom
		FROM products
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

	if opts.Avail != nil {
		query += fmt.Sprintf(" AND avail = $%d", argPos)
		args = append(args, *opts.Avail)
		argPos++
	}

	if opts.MinPrice > 0 {
		query += fmt.Sprintf(" AND price >= $%d", argPos)
		args = append(args, opts.MinPrice)
		argPos++
	}

	if opts.MaxPrice > 0 {
		query += fmt.Sprintf(" AND price <= $%d", argPos)
		args = append(args, opts.MaxPrice)
		argPos++
	}

	// Sorting
	sortBy := "id"
	if opts.SortBy != "" {
		switch opts.SortBy {
		case "name", "price", "slug":
			sortBy = opts.SortBy
		}
	}

	sortOrder := "ASC"
	if opts.SortOrder == "desc" {
		sortOrder = "DESC"
	}

	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

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
		return nil, fmt.Errorf("failed to list products: %w", err)
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		product, err := r.scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return products, nil
}

func (r *productRepository) SetAvailability(ctx context.Context, id int, avail bool) error {
	query := `UPDATE products SET avail = $1 WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, avail, id)
	if err != nil {
		return fmt.Errorf("failed to set availability: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrProductNotFound
	}

	return nil
}

func (r *productRepository) GetAvailable(ctx context.Context) ([]*Product, error) {
	query := `
		SELECT id, slug, name, desc, tag, label, price, avail, items, recipe, custom
		FROM products
		WHERE avail = true
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get available products: %w", err)
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		product, err := r.scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return products, nil
}

func (r *productRepository) GetByTag(ctx context.Context, tag string) ([]*Product, error) {
	query := `
		SELECT id, slug, name, desc, tag, label, price, avail, items, recipe, custom
		FROM products
		WHERE tag = $1
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to get products by tag: %w", err)
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		product, err := r.scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return products, nil
}

func (r *productRepository) GetByLabel(ctx context.Context, label string) ([]*Product, error) {
	query := `
		SELECT id, slug, name, desc, tag, label, price, avail, items, recipe, custom
		FROM products
		WHERE label = $1
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query, label)
	if err != nil {
		return nil, fmt.Errorf("failed to get products by label: %w", err)
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		product, err := r.scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return products, nil
}

func (r *productRepository) GetBundles(ctx context.Context) ([]*Product, error) {
	query := `
		SELECT id, slug, name, desc, tag, label, price, avail, items, recipe, custom
		FROM products
		WHERE items IS NOT NULL
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get bundles: %w", err)
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		product, err := r.scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return products, nil
}

func (r *productRepository) GetWithRecipe(ctx context.Context) ([]*Product, error) {
	query := `
		SELECT id, slug, name, desc, tag, label, price, avail, items, recipe, custom
		FROM products
		WHERE recipe IS NOT NULL
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get products with recipe: %w", err)
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		product, err := r.scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return products, nil
}

func (r *productRepository) Search(ctx context.Context, query string) ([]*Product, error) {
	searchQuery := `
		SELECT id, slug, name, desc, tag, label, price, avail, items, recipe, custom
		FROM products
		WHERE name ILIKE $1 OR desc ILIKE $1 OR tag ILIKE $1
		ORDER BY name
	`

	searchPattern := "%" + query + "%"
	rows, err := r.db.QueryContext(ctx, searchQuery, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search products: %w", err)
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		product, err := r.scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return products, nil
}

func (r *productRepository) UpdatePrice(ctx context.Context, id int, price int64) error {
	query := `UPDATE products SET price = $1 WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, price, id)
	if err != nil {
		return fmt.Errorf("failed to update price: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrProductNotFound
	}

	return nil
}

func (r *productRepository) GetByPriceRange(ctx context.Context, minPrice, maxPrice int64) ([]*Product, error) {
	query := `
		SELECT id, slug, name, desc, tag, label, price, avail, items, recipe, custom
		FROM products
		WHERE price >= $1 AND price <= $2
		ORDER BY price
	`

	rows, err := r.db.QueryContext(ctx, query, minPrice, maxPrice)
	if err != nil {
		return nil, fmt.Errorf("failed to get products by price range: %w", err)
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		product, err := r.scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return products, nil
}

// Helper methods

func (r *productRepository) scanProduct(scanner interface {
	Scan(dest ...any) error
}) (*Product, error) {
	product := &Product{}
	var itemsJSON, recipeJSON, customJSON []byte

	err := scanner.Scan(
		&product.Id, &product.Slug, &product.Name, &product.Desc,
		&product.Tag, &product.Label, &product.Price, &product.Avail,
		&itemsJSON, &recipeJSON, &customJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan product: %w", err)
	}

	if err := r.unmarshalProductData(product, itemsJSON, recipeJSON, customJSON); err != nil {
		return nil, err
	}

	return product, nil
}

func (r *productRepository) unmarshalProductData(product *Product, itemsJSON, recipeJSON, customJSON []byte) error {
	if len(itemsJSON) > 0 {
		var items []string
		if err := json.Unmarshal(itemsJSON, &items); err != nil {
			return fmt.Errorf("failed to unmarshal items: %w", err)
		}
		product.Items = &items
	}

	if len(recipeJSON) > 0 {
		var recipe map[string]int
		if err := json.Unmarshal(recipeJSON, &recipe); err != nil {
			return fmt.Errorf("failed to unmarshal recipe: %w", err)
		}
		product.Recipe = &recipe
	}

	if len(customJSON) > 0 {
		if err := json.Unmarshal(customJSON, &product.Custom); err != nil {
			return fmt.Errorf("failed to unmarshal custom data: %w", err)
		}
	}

	return nil
}

func (r *productRepository) marshalNullableSlice(items *[]string) ([]byte, error) {
	if items == nil {
		return nil, nil
	}
	return json.Marshal(items)
}

func (r *productRepository) marshalNullableMap(recipe *map[string]int) ([]byte, error) {
	if recipe == nil {
		return nil, nil
	}
	return json.Marshal(recipe)
}

// Helper function to check for duplicate key violations
// This is PostgreSQL-specific; adjust for your database
func isDuplicateKeyError(err error) bool {
	// You might want to import "github.com/lib/pq" and check:
	// if pqErr, ok := err.(*pq.Error); ok {
	//     return pqErr.Code == "23505"
	// }
	return false
}
