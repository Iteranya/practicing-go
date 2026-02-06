package user

import (
	"context"
	"errors"
)

var (
	ErrPasswordTooShort = errors.New("password must be at least 6 characters")
)

type UserService interface {
	// Authentication
	RegisterUser(ctx context.Context, input UserInput) (*User, error)
	Login(ctx context.Context, username, password string) (string, *User, error)

	// User Management
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
	Password    string         `json:"password"` // Raw password, only used on Create
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

// RegisterUser handles creation and hashing of the password
func (s *userService) RegisterUser(ctx context.Context, input UserInput) (*User, error) {
	if input.Username == "" || input.Password == "" {
		return nil, ErrInvalidUserInput
	}

	if len(input.Password) < 6 {
		return nil, ErrPasswordTooShort
	}

	if input.Role == "" {
		input.Role = "staff"
	}

	// Create the domain entity
	newUser := &User{
		Username:    input.Username,
		DisplayName: input.DisplayName,
		Role:        input.Role,
		Active:      true, // Active by default on register
		Setting:     input.Setting,
		Custom:      input.Custom,
	}

	// Use domain logic from auth.go to hash password
	if err := newUser.SetPassword(input.Password); err != nil {
		return nil, err
	}

	// Save to DB
	if err := s.repo.Create(ctx, newUser); err != nil {
		return nil, err
	}

	return newUser, nil
}

// Login verifies credentials and returns a JWT token + User Info
func (s *userService) Login(ctx context.Context, username, password string) (string, *User, error) {
	// 1. Find User
	u, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		// Mask specific DB errors for security, just say invalid creds
		return "", nil, ErrInvalidCredentials
	}

	// 2. Check Active Status
	if !u.Active {
		return "", nil, errors.New("user account is inactive")
	}

	// 3. Check Password (domain logic)
	if !u.CheckPassword(password) {
		return "", nil, ErrInvalidCredentials
	}

	// 4. Generate Token (domain logic)
	token, err := GenerateToken(u)
	if err != nil {
		return "", nil, err
	}

	return token, u, nil
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
	// Note: We deliberately do NOT update Password here. Use ChangePassword.

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
		return ErrPasswordTooShort
	}

	// We use a temporary user struct to access the SetPassword logic
	// to avoid rewriting the bcrypt logic here.
	tempUser := &User{}
	if err := tempUser.SetPassword(newPassword); err != nil {
		return err
	}

	// Push the new hash to the repository
	return s.repo.UpdatePassword(ctx, id, tempUser.Hash)
}

func (s *userService) UpdateSettings(ctx context.Context, id int, settings map[string]any) error {
	return s.repo.UpdateSettings(ctx, id, settings)
}

func (s *userService) ToggleActive(ctx context.Context, id int, active bool) error {
	return s.repo.SetActive(ctx, id, active)
}
