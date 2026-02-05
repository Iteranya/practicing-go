package main

import (
	"context"
	"errors"
)

type UserService interface {
	RegisterUser(ctx context.Context, input UserInput) (*User, error)
	GetUser(ctx context.Context, idOrUsername any) (*User, error)
	UpdateUser(ctx context.Context, id int, input UserInput) error
	DeleteUser(ctx context.Context, id int) error
	ListUsers(ctx context.Context, params UserServiceListParams) ([]*User, error)

	// Specific Actions
	ChangePassword(ctx context.Context, id int, newPassword string) error
	UpdateSettings(ctx context.Context, id int, settings map[string]any) error
	ToggleActive(ctx context.Context, id int, active bool) error
}

// UserInput separates the API request shape from the Database Model
type UserInput struct {
	Username    string         `json:"username"`
	Password    string         `json:"password"` // Raw password
	DisplayName string         `json:"display_name"`
	Role        string         `json:"role"`
	Setting     map[string]any `json:"setting"`
	Custom      map[string]any `json:"custom"`
}

type UserServiceListParams struct {
	Role   string
	Query  string // Username or Display Name search
	Active *bool
	Limit  int
	Page   int
}

type userService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) RegisterUser(ctx context.Context, input UserInput) (*User, error) {
	// Validation
	if input.Username == "" || input.Password == "" {
		return nil, ErrInvalidUserInput
	}

	// Default role if empty
	if input.Role == "" {
		input.Role = "staff"
	}

	// TODO: Replace this with proper hashing (e.g., bcrypt.GenerateFromPassword)
	hashedPassword := "hashed_" + input.Password

	newUser := &User{
		Username:    input.Username,
		Hash:        hashedPassword,
		DisplayName: input.DisplayName,
		Role:        input.Role,
		Active:      true, // Active by default
		Setting:     input.Setting,
		Custom:      input.Custom,
	}

	err := s.repo.Create(ctx, newUser)
	if err != nil {
		return nil, err
	}

	return newUser, nil
}

func (s *userService) GetUser(ctx context.Context, idOrUsername any) (*User, error) {
	switch v := idOrUsername.(type) {
	case int:
		return s.repo.GetByID(ctx, v)
	case string:
		return s.repo.GetByUsername(ctx, v)
	default:
		return nil, ErrInvalidUserInput
	}
}

func (s *userService) UpdateUser(ctx context.Context, id int, input UserInput) error {
	if id == 0 {
		return ErrInvalidUserInput
	}

	// Fetch existing to preserve fields like Hash, Active if not provided
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Update fields
	if input.Username != "" {
		existing.Username = input.Username
	}
	if input.DisplayName != "" {
		existing.DisplayName = input.DisplayName
	}
	if input.Role != "" {
		existing.Role = input.Role
	}
	if input.Setting != nil {
		existing.Setting = input.Setting
	}
	if input.Custom != nil {
		existing.Custom = input.Custom
	}

	return s.repo.Update(ctx, existing)
}

func (s *userService) DeleteUser(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

func (s *userService) ListUsers(ctx context.Context, params UserServiceListParams) ([]*User, error) {
	if params.Query != "" {
		return s.repo.Search(ctx, params.Query)
	}

	offset := 0
	if params.Page > 1 {
		offset = (params.Page - 1) * params.Limit
	}

	repoOpts := UserListOptions{
		Role:   params.Role,
		Active: params.Active,
		Limit:  params.Limit,
		Offset: offset,
		SortBy: "username",
	}

	return s.repo.List(ctx, repoOpts)
}

func (s *userService) ChangePassword(ctx context.Context, id int, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("password too short")
	}

	// TODO: Replace with real hashing
	newHash := "hashed_" + newPassword

	return s.repo.UpdatePassword(ctx, id, newHash)
}

func (s *userService) UpdateSettings(ctx context.Context, id int, settings map[string]any) error {
	return s.repo.UpdateSettings(ctx, id, settings)
}

func (s *userService) ToggleActive(ctx context.Context, id int, active bool) error {
	return s.repo.SetActive(ctx, id, active)
}
