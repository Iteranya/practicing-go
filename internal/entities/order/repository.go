package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrOrderNotFound     = errors.New("order not found")
	ErrInvalidOrderInput = errors.New("invalid order input")
	ErrInvalidPayment    = errors.New("invalid payment amount")
)

type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	GetByID(ctx context.Context, id int) (*Order, error)
	Update(ctx context.Context, order *Order) error
	Delete(ctx context.Context, id int) error
	List(ctx context.Context, opts OrderListOptions) ([]*Order, error)
	GetByClerk(ctx context.Context, clerkId int) ([]*Order, error)
	GetByDateRange(ctx context.Context, start, end time.Time) ([]*Order, error)
	UpdatePayment(ctx context.Context, id int, paid int64) error
	GetTotalSales(ctx context.Context, start, end time.Time) (int64, error)
	GetClerkSales(ctx context.Context, clerkId int, start, end time.Time) (int64, error)
	GetAverageOrderValue(ctx context.Context, start, end time.Time) (float64, error)
	Count(ctx context.Context) (int, error)
	GetRecentOrders(ctx context.Context, limit int) ([]*Order, error)
}

type OrderListOptions struct {
	ClerkId   int
	MinTotal  int64
	MaxTotal  int64
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
	SortBy    string // id, total, created_at
	SortOrder string // asc, desc
}

type orderRepository struct {
	db *sql.DB
}

func NewOrderRepository(db *sql.DB) OrderRepository {
	return &orderRepository{db: db}
}

func (r *orderRepository) Create(ctx context.Context, order *Order) error {
	if len(order.Items) == 0 || order.ClerkId == 0 {
		return ErrInvalidOrderInput
	}

	if order.Paid < 0 || order.Total < 0 {
		return ErrInvalidPayment
	}

	// Calculate change if not already set
	if order.Change == 0 && order.Paid > 0 {
		order.Change = order.Paid - order.Total
	}

	itemsJSON, err := json.Marshal(order.Items)
	if err != nil {
		return fmt.Errorf("failed to marshal items: %w", err)
	}

	customJSON, err := json.Marshal(order.Custom)
	if err != nil {
		return fmt.Errorf("failed to marshal custom data: %w", err)
	}

	query := `
		INSERT INTO orders (items, clerk_id, total, paid, change, custom, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	err = r.db.QueryRowContext(
		ctx, query,
		itemsJSON, order.ClerkId, order.Total, order.Paid, order.Change, customJSON, time.Now(),
	).Scan(&order.Id)

	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	return nil
}

func (r *orderRepository) GetByID(ctx context.Context, id int) (*Order, error) {
	query := `
		SELECT id, items, clerk_id, total, paid, change, custom
		FROM orders
		WHERE id = $1
	`

	order := &Order{}
	var itemsJSON, customJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&order.Id, &itemsJSON, &order.ClerkId,
		&order.Total, &order.Paid, &order.Change, &customJSON,
	)

	if err == sql.ErrNoRows {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	if err := r.unmarshalOrderData(order, itemsJSON, customJSON); err != nil {
		return nil, err
	}

	return order, nil
}

func (r *orderRepository) Update(ctx context.Context, order *Order) error {
	if order.Id == 0 {
		return ErrInvalidOrderInput
	}

	itemsJSON, err := json.Marshal(order.Items)
	if err != nil {
		return fmt.Errorf("failed to marshal items: %w", err)
	}

	customJSON, err := json.Marshal(order.Custom)
	if err != nil {
		return fmt.Errorf("failed to marshal custom data: %w", err)
	}

	query := `
		UPDATE orders
		SET items = $1, clerk_id = $2, total = $3, paid = $4, change = $5, custom = $6
		WHERE id = $7
	`

	result, err := r.db.ExecContext(
		ctx, query,
		itemsJSON, order.ClerkId, order.Total, order.Paid, order.Change, customJSON, order.Id,
	)

	if err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrOrderNotFound
	}

	return nil
}

func (r *orderRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM orders WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete order: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrOrderNotFound
	}

	return nil
}

func (r *orderRepository) List(ctx context.Context, opts OrderListOptions) ([]*Order, error) {
	query := `
		SELECT id, items, clerk_id, total, paid, change, custom
		FROM orders
		WHERE 1=1
	`
	args := []any{}
	argPos := 1

	if opts.ClerkId > 0 {
		query += fmt.Sprintf(" AND clerk_id = $%d", argPos)
		args = append(args, opts.ClerkId)
		argPos++
	}

	if opts.MinTotal > 0 {
		query += fmt.Sprintf(" AND total >= $%d", argPos)
		args = append(args, opts.MinTotal)
		argPos++
	}

	if opts.MaxTotal > 0 {
		query += fmt.Sprintf(" AND total <= $%d", argPos)
		args = append(args, opts.MaxTotal)
		argPos++
	}

	if opts.StartDate != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argPos)
		args = append(args, *opts.StartDate)
		argPos++
	}

	if opts.EndDate != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argPos)
		args = append(args, *opts.EndDate)
		argPos++
	}

	// Sorting
	sortBy := "id"
	if opts.SortBy != "" {
		switch opts.SortBy {
		case "total", "created_at", "id":
			sortBy = opts.SortBy
		}
	}

	sortOrder := "DESC" // Most recent first by default
	if opts.SortOrder == "asc" {
		sortOrder = "ASC"
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
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		order, err := r.scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return orders, nil
}

func (r *orderRepository) GetByClerk(ctx context.Context, clerkId int) ([]*Order, error) {
	query := `
		SELECT id, items, clerk_id, total, paid, change, custom
		FROM orders
		WHERE clerk_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, clerkId)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders by clerk: %w", err)
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		order, err := r.scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return orders, nil
}

func (r *orderRepository) GetByDateRange(ctx context.Context, start, end time.Time) ([]*Order, error) {
	query := `
		SELECT id, items, clerk_id, total, paid, change, custom
		FROM orders
		WHERE created_at >= $1 AND created_at <= $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders by date range: %w", err)
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		order, err := r.scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return orders, nil
}

func (r *orderRepository) UpdatePayment(ctx context.Context, id int, paid int64) error {
	if paid < 0 {
		return ErrInvalidPayment
	}

	// Get current order to calculate new change
	query := `SELECT total FROM orders WHERE id = $1`
	var total int64
	err := r.db.QueryRowContext(ctx, query, id).Scan(&total)
	if err == sql.ErrNoRows {
		return ErrOrderNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to get order total: %w", err)
	}

	change := paid - total

	updateQuery := `UPDATE orders SET paid = $1, change = $2 WHERE id = $3`
	result, err := r.db.ExecContext(ctx, updateQuery, paid, change, id)
	if err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrOrderNotFound
	}

	return nil
}

func (r *orderRepository) GetTotalSales(ctx context.Context, start, end time.Time) (int64, error) {
	query := `
		SELECT COALESCE(SUM(total), 0)
		FROM orders
		WHERE created_at >= $1 AND created_at <= $2
	`

	var total int64
	err := r.db.QueryRowContext(ctx, query, start, end).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to get total sales: %w", err)
	}

	return total, nil
}

func (r *orderRepository) GetClerkSales(ctx context.Context, clerkId int, start, end time.Time) (int64, error) {
	query := `
		SELECT COALESCE(SUM(total), 0)
		FROM orders
		WHERE clerk_id = $1 AND created_at >= $2 AND created_at <= $3
	`

	var total int64
	err := r.db.QueryRowContext(ctx, query, clerkId, start, end).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to get clerk sales: %w", err)
	}

	return total, nil
}

func (r *orderRepository) GetAverageOrderValue(ctx context.Context, start, end time.Time) (float64, error) {
	query := `
		SELECT COALESCE(AVG(total), 0)
		FROM orders
		WHERE created_at >= $1 AND created_at <= $2
	`

	var avg float64
	err := r.db.QueryRowContext(ctx, query, start, end).Scan(&avg)
	if err != nil {
		return 0, fmt.Errorf("failed to get average order value: %w", err)
	}

	return avg, nil
}

func (r *orderRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM orders`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count orders: %w", err)
	}

	return count, nil
}

func (r *orderRepository) GetRecentOrders(ctx context.Context, limit int) ([]*Order, error) {
	query := `
		SELECT id, items, clerk_id, total, paid, change, custom
		FROM orders
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent orders: %w", err)
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		order, err := r.scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return orders, nil
}

// Helper methods

func (r *orderRepository) scanOrder(scanner interface {
	Scan(dest ...any) error
}) (*Order, error) {
	order := &Order{}
	var itemsJSON, customJSON []byte

	err := scanner.Scan(
		&order.Id, &itemsJSON, &order.ClerkId,
		&order.Total, &order.Paid, &order.Change, &customJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan order: %w", err)
	}

	if err := r.unmarshalOrderData(order, itemsJSON, customJSON); err != nil {
		return nil, err
	}

	return order, nil
}

func (r *orderRepository) unmarshalOrderData(order *Order, itemsJSON, customJSON []byte) error {
	if len(itemsJSON) > 0 {
		if err := json.Unmarshal(itemsJSON, &order.Items); err != nil {
			return fmt.Errorf("failed to unmarshal items: %w", err)
		}
	}

	if len(customJSON) > 0 {
		if err := json.Unmarshal(customJSON, &order.Custom); err != nil {
			return fmt.Errorf("failed to unmarshal custom data: %w", err)
		}
	}

	return nil
}
