package user

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidUserInput   = errors.New("invalid user input")
	ErrDuplicateUsername  = errors.New("username already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id int) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id int) error
	List(ctx context.Context, opts UserListOptions) ([]*User, error)
	UpdatePassword(ctx context.Context, id int, hash string) error
	UpdateSettings(ctx context.Context, id int, settings map[string]any) error
	SetActive(ctx context.Context, id int, active bool) error
	GetByRole(ctx context.Context, role string) ([]*User, error)
	Search(ctx context.Context, query string) ([]*User, error)
	Count(ctx context.Context) (int, error)
}

type UserListOptions struct {
	Role      string
	Active    *bool // pointer so we can distinguish between false and not set
	Limit     int
	Offset    int
	SortBy    string // username, display_name, id
	SortOrder string // asc, desc
}

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *User) error {
	if user.Username == "" || user.Hash == "" {
		return ErrInvalidUserInput
	}

	settingJSON, err := json.Marshal(user.Setting)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	customJSON, err := json.Marshal(user.Custom)
	if err != nil {
		return fmt.Errorf("failed to marshal custom data: %w", err)
	}

	query := `
		INSERT INTO users (username, display_name, hash, role, active, setting, custom)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	err = r.db.QueryRowContext(
		ctx, query,
		user.Username, user.DisplayName, user.Hash, user.Role, user.Active, settingJSON, customJSON,
	).Scan(&user.Id)

	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrDuplicateUsername
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id int) (*User, error) {
	query := `
		SELECT id, username, display_name, hash, role, active, setting, custom
		FROM users
		WHERE id = $1
	`

	user := &User{}
	var settingJSON, customJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.Id, &user.Username, &user.DisplayName, &user.Hash,
		&user.Role, &user.Active, &settingJSON, &customJSON,
	)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if err := r.unmarshalUserData(user, settingJSON, customJSON); err != nil {
		return nil, err
	}

	return user, nil
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	query := `
		SELECT id, username, display_name, hash, role, active, setting, custom
		FROM users
		WHERE username = $1
	`

	user := &User{}
	var settingJSON, customJSON []byte

	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&user.Id, &user.Username, &user.DisplayName, &user.Hash,
		&user.Role, &user.Active, &settingJSON, &customJSON,
	)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if err := r.unmarshalUserData(user, settingJSON, customJSON); err != nil {
		return nil, err
	}

	return user, nil
}

func (r *userRepository) Update(ctx context.Context, user *User) error {
	if user.Id == 0 {
		return ErrInvalidUserInput
	}

	settingJSON, err := json.Marshal(user.Setting)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	customJSON, err := json.Marshal(user.Custom)
	if err != nil {
		return fmt.Errorf("failed to marshal custom data: %w", err)
	}

	query := `
		UPDATE users
		SET username = $1, display_name = $2, hash = $3, role = $4, 
		    active = $5, setting = $6, custom = $7
		WHERE id = $8
	`

	result, err := r.db.ExecContext(
		ctx, query,
		user.Username, user.DisplayName, user.Hash, user.Role,
		user.Active, settingJSON, customJSON, user.Id,
	)

	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrDuplicateUsername
		}
		return fmt.Errorf("failed to update user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *userRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *userRepository) List(ctx context.Context, opts UserListOptions) ([]*User, error) {
	query := `
		SELECT id, username, display_name, hash, role, active, setting, custom
		FROM users
		WHERE 1=1
	`
	args := []any{}
	argPos := 1

	if opts.Role != "" {
		query += fmt.Sprintf(" AND role = $%d", argPos)
		args = append(args, opts.Role)
		argPos++
	}

	if opts.Active != nil {
		query += fmt.Sprintf(" AND active = $%d", argPos)
		args = append(args, *opts.Active)
		argPos++
	}

	// Sorting
	sortBy := "id"
	if opts.SortBy != "" {
		switch opts.SortBy {
		case "username", "display_name", "id":
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
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user, err := r.scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return users, nil
}

func (r *userRepository) UpdatePassword(ctx context.Context, id int, hash string) error {
	query := `UPDATE users SET hash = $1 WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, hash, id)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *userRepository) UpdateSettings(ctx context.Context, id int, settings map[string]any) error {
	settingJSON, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	query := `UPDATE users SET setting = $1 WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, settingJSON, id)
	if err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *userRepository) SetActive(ctx context.Context, id int, active bool) error {
	query := `UPDATE users SET active = $1 WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, active, id)
	if err != nil {
		return fmt.Errorf("failed to set active status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *userRepository) GetByRole(ctx context.Context, role string) ([]*User, error) {
	query := `
		SELECT id, username, display_name, hash, role, active, setting, custom
		FROM users
		WHERE role = $1
		ORDER BY username
	`

	rows, err := r.db.QueryContext(ctx, query, role)
	if err != nil {
		return nil, fmt.Errorf("failed to get users by role: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user, err := r.scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return users, nil
}

func (r *userRepository) Search(ctx context.Context, query string) ([]*User, error) {
	searchQuery := `
		SELECT id, username, display_name, hash, role, active, setting, custom
		FROM users
		WHERE username ILIKE $1 OR display_name ILIKE $1
		ORDER BY username
	`

	searchPattern := "%" + query + "%"
	rows, err := r.db.QueryContext(ctx, searchQuery, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user, err := r.scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return users, nil
}

func (r *userRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM users`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}

	return count, nil
}

// Helper methods

func (r *userRepository) scanUser(scanner interface {
	Scan(dest ...any) error
}) (*User, error) {
	user := &User{}
	var settingJSON, customJSON []byte

	err := scanner.Scan(
		&user.Id, &user.Username, &user.DisplayName, &user.Hash,
		&user.Role, &user.Active, &settingJSON, &customJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}

	if err := r.unmarshalUserData(user, settingJSON, customJSON); err != nil {
		return nil, err
	}

	return user, nil
}

func (r *userRepository) unmarshalUserData(user *User, settingJSON, customJSON []byte) error {
	if len(settingJSON) > 0 {
		if err := json.Unmarshal(settingJSON, &user.Setting); err != nil {
			return fmt.Errorf("failed to unmarshal settings: %w", err)
		}
	}

	if len(customJSON) > 0 {
		if err := json.Unmarshal(customJSON, &user.Custom); err != nil {
			return fmt.Errorf("failed to unmarshal custom data: %w", err)
		}
	}

	return nil
}

func isDuplicateKeyError(_ error) bool {
	return false
}
