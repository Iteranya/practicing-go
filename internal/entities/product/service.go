package main

import (
	"context"
)

type ProductService interface {
	CreateProduct(ctx context.Context, product Product) (*Product, error)
	GetProduct(ctx context.Context, idOrSlug any) (*Product, error)
	UpdateProduct(ctx context.Context, id int, product Product) error
	DeleteProduct(ctx context.Context, id int) error
	ListProducts(ctx context.Context, params ProductServiceListParams) ([]*Product, error)

	// Specific Actions
	SetAvailability(ctx context.Context, id int, available bool) error
	UpdatePrice(ctx context.Context, id int, newPrice int64) error

	// Specialized Lists
	GetBundles(ctx context.Context) ([]*Product, error)
	GetProductsWithRecipes(ctx context.Context) ([]*Product, error)
}

type ProductServiceListParams struct {
	Tag      string
	Label    string
	Query    string // For search
	Avail    *bool
	MinPrice int64
	MaxPrice int64
	Limit    int
	Page     int
	SortBy   string
}

type productService struct {
	repo ProductRepository
}

func NewProductService(repo ProductRepository) ProductService {
	return &productService{repo: repo}
}

func (s *productService) CreateProduct(ctx context.Context, product Product) (*Product, error) {
	// Validation
	if product.Name == "" || product.Slug == "" {
		return nil, ErrInvalidProductInput
	}
	if product.Price < 0 {
		return nil, ErrInvalidProductInput
	}

	// Validate Recipe/Items logic if necessary (e.g. can't be both bundle and recipe?)
	// For now, we allow flexibility.

	err := s.repo.Create(ctx, &product)
	if err != nil {
		return nil, err
	}

	return &product, nil
}

func (s *productService) GetProduct(ctx context.Context, idOrSlug any) (*Product, error) {
	switch v := idOrSlug.(type) {
	case int:
		return s.repo.GetByID(ctx, v)
	case string:
		return s.repo.GetBySlug(ctx, v)
	default:
		return nil, ErrInvalidProductInput
	}
}

func (s *productService) UpdateProduct(ctx context.Context, id int, product Product) error {
	if id == 0 {
		return ErrInvalidProductInput
	}

	// Ensure ID is set on the struct
	product.Id = id

	return s.repo.Update(ctx, &product)
}

func (s *productService) DeleteProduct(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

func (s *productService) ListProducts(ctx context.Context, params ProductServiceListParams) ([]*Product, error) {
	// 1. Handle textual search
	if params.Query != "" {
		return s.repo.Search(ctx, params.Query)
	}

	// 2. Handle Price Range specific query if strict range is needed
	// (Though List() in repo also handles this, strict range methods exist in Repo)
	/*
	   Note: The Repo List method already handles Min/Max price.
	   We only use GetByPriceRange if we want ONLY price filtering without pagination/tags.
	   We will stick to repo.List for general usage.
	*/

	offset := 0
	if params.Page > 1 {
		offset = (params.Page - 1) * params.Limit
	}

	repoOpts := ProductListOptions{
		Tag:      params.Tag,
		Label:    params.Label,
		Avail:    params.Avail,
		MinPrice: params.MinPrice,
		MaxPrice: params.MaxPrice,
		SortBy:   params.SortBy,
		Limit:    params.Limit,
		Offset:   offset,
	}

	return s.repo.List(ctx, repoOpts)
}

func (s *productService) SetAvailability(ctx context.Context, id int, available bool) error {
	return s.repo.SetAvailability(ctx, id, available)
}

func (s *productService) UpdatePrice(ctx context.Context, id int, newPrice int64) error {
	if newPrice < 0 {
		return ErrInvalidProductInput
	}
	return s.repo.UpdatePrice(ctx, id, newPrice)
}

func (s *productService) GetBundles(ctx context.Context) ([]*Product, error) {
	return s.repo.GetBundles(ctx)
}

func (s *productService) GetProductsWithRecipes(ctx context.Context) ([]*Product, error) {
	return s.repo.GetWithRecipe(ctx)
}
