package main

import (
	"context"
	"time"
)

type OrderService interface {
	CreateOrder(ctx context.Context, order Order) (*Order, error)
	GetOrder(ctx context.Context, id int) (*Order, error)
	ListOrders(ctx context.Context, params OrderServiceListParams) ([]*Order, error)
	GetOrdersByClerk(ctx context.Context, clerkId int) ([]*Order, error)
	ProcessPayment(ctx context.Context, id int, amountPaid int64) error

	// Analytics
	GetSalesStats(ctx context.Context, start, end time.Time) (SalesStats, error)
	GetClerkPerformance(ctx context.Context, clerkId int, start, end time.Time) (int64, error)
}

// OrderServiceListParams maps incoming request params to repo options
type OrderServiceListParams struct {
	ClerkId   int
	StartDate *time.Time
	EndDate   *time.Time
	MinTotal  int64
	MaxTotal  int64
	Limit     int
	Page      int
}

type SalesStats struct {
	TotalRevenue      int64   `json:"total_revenue"`
	AverageOrderValue float64 `json:"average_order_value"`
	OrderCount        int     `json:"order_count"`
}

type orderService struct {
	repo OrderRepository
}

func NewOrderService(repo OrderRepository) OrderService {
	return &orderService{repo: repo}
}

func (s *orderService) CreateOrder(ctx context.Context, order Order) (*Order, error) {
	// Basic Validation
	if len(order.Items) == 0 {
		return nil, ErrInvalidOrderInput
	}
	if order.ClerkId == 0 {
		return nil, ErrInvalidOrderInput
	}

	// Logic: Calculate Change only if Paid is sufficient
	if order.Paid >= order.Total {
		order.Change = order.Paid - order.Total
	} else if order.Paid > 0 {
		// If they paid some, but not enough, we might want to reject
		// or handle partial payment. For now, let's treat it as valid
		// but change is 0 (or negative indicating debt)
		order.Change = order.Paid - order.Total
	}

	// Logic: Ensure Created time is set (Repo sets it via SQL NOW(),
	// but we might want it in the struct that comes back)
	// The Repo Create method uses RETURNING id, but relies on SQL for timestamp.

	err := s.repo.Create(ctx, &order)
	if err != nil {
		return nil, err
	}

	// Since the DB handles the timestamp, we usually re-fetch or just return the ID.
	// We'll return the input object with the new ID.
	return &order, nil
}

func (s *orderService) GetOrder(ctx context.Context, id int) (*Order, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *orderService) ListOrders(ctx context.Context, params OrderServiceListParams) ([]*Order, error) {
	offset := 0
	if params.Page > 1 {
		offset = (params.Page - 1) * params.Limit
	}

	repoOpts := OrderListOptions{
		ClerkId:   params.ClerkId,
		StartDate: params.StartDate,
		EndDate:   params.EndDate,
		MinTotal:  params.MinTotal,
		MaxTotal:  params.MaxTotal,
		Limit:     params.Limit,
		Offset:    offset,
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	return s.repo.List(ctx, repoOpts)
}

func (s *orderService) GetOrdersByClerk(ctx context.Context, clerkId int) ([]*Order, error) {
	if clerkId == 0 {
		return nil, ErrInvalidOrderInput
	}
	return s.repo.GetByClerk(ctx, clerkId)
}

func (s *orderService) ProcessPayment(ctx context.Context, id int, amountPaid int64) error {
	// This updates the Paid amount and recalculates Change in the Repo
	return s.repo.UpdatePayment(ctx, id, amountPaid)
}

func (s *orderService) GetSalesStats(ctx context.Context, start, end time.Time) (SalesStats, error) {
	total, err := s.repo.GetTotalSales(ctx, start, end)
	if err != nil {
		return SalesStats{}, err
	}

	avg, err := s.repo.GetAverageOrderValue(ctx, start, end)
	if err != nil {
		return SalesStats{}, err
	}

	// We might want count as well, though repo.Count is global.
	// For date range count, we would rely on len(GetByDateRange) or add a specific repo method.
	// For now, let's just return what we have.

	return SalesStats{
		TotalRevenue:      total,
		AverageOrderValue: avg,
	}, nil
}

func (s *orderService) GetClerkPerformance(ctx context.Context, clerkId int, start, end time.Time) (int64, error) {
	return s.repo.GetClerkSales(ctx, clerkId, start, end)
}
