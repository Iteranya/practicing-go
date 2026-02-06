package inventory

import (
	"context"
)

type InventoryService interface {
	CreateInventory(ctx context.Context, input Inventory) (*Inventory, error)
	GetInventory(ctx context.Context, idOrSlug any) (*Inventory, error)
	UpdateInventory(ctx context.Context, id int, input Inventory) error
	DeleteInventory(ctx context.Context, id int) error
	ListInventory(ctx context.Context, params ListParams) ([]*Inventory, error)
	AdjustStock(ctx context.Context, id int, delta int64) error
}

type ListParams struct {
	Tag   string
	Label string
	Query string // Use this to toggle between List() and Search()
	Limit int
	Page  int
}

type inventoryService struct {
	repo InventoryRepository
}

func NewInventoryService(repo InventoryRepository) InventoryService {
	return &inventoryService{repo: repo}
}

func (s *inventoryService) CreateInventory(ctx context.Context, input Inventory) (*Inventory, error) {
	// Basic validation
	if input.Name == "" || input.Slug == "" {
		return nil, ErrInvalidInput
	}

	// Initialize with 0 stock if not provided, though the DB defaults to 0
	if input.Stock < 0 {
		return nil, ErrInvalidInput
	}

	err := s.repo.Create(ctx, &input)
	if err != nil {
		return nil, err
	}

	return &input, nil
}

func (s *inventoryService) GetInventory(ctx context.Context, idOrSlug any) (*Inventory, error) {
	switch v := idOrSlug.(type) {
	case int:
		return s.repo.GetByID(ctx, v)
	case string:
		return s.repo.GetBySlug(ctx, v)
	default:
		return nil, ErrInvalidInput
	}
}

func (s *inventoryService) UpdateInventory(ctx context.Context, id int, input Inventory) error {
	if id == 0 {
		return ErrInvalidInput
	}

	// Fetch existing to ensure it exists
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Merging logic: Only update fields present in input?
	// For this example, we assume 'input' is the complete new state
	// excluding ID.
	input.Id = existing.Id

	return s.repo.Update(ctx, &input)
}

func (s *inventoryService) DeleteInventory(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

func (s *inventoryService) ListInventory(ctx context.Context, params ListParams) ([]*Inventory, error) {
	// If a search query is provided, use the Search method
	if params.Query != "" {
		return s.repo.Search(ctx, params.Query)
	}

	// Calculate offset
	offset := 0
	if params.Page > 1 {
		offset = (params.Page - 1) * params.Limit
	}

	repoOpts := ListOptions{
		Tag:    params.Tag,
		Label:  params.Label,
		Limit:  params.Limit,
		Offset: offset,
	}

	return s.repo.List(ctx, repoOpts)
}

func (s *inventoryService) AdjustStock(ctx context.Context, id int, delta int64) error {
	if delta == 0 {
		return nil // No op
	}
	return s.repo.UpdateStock(ctx, id, delta)
}
