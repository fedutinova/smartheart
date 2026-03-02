package repository

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/fedutinova/smartheart/internal/common"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

// CreateUser creates a new user
func (r *Repository) CreateUser(ctx context.Context, user *models.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	query := `
		INSERT INTO users (id, username, email, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`

	_, err := r.q.Exec(ctx, query, user.ID, user.Username, user.Email, user.PasswordHash)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("user with this email or username %w", common.ErrConflict)
		}
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetUserByEmail retrieves a user by email
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	var user models.User
	err := r.q.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, common.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	roles, err := r.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}
	user.Roles = roles

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (r *Repository) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, username, email, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err := r.q.QueryRow(ctx, query, userID).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, common.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	roles, err := r.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}
	user.Roles = roles

	return &user, nil
}

// GetUserRoles retrieves all roles with permissions for a user in a single query.
func (r *Repository) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]models.Role, error) {
	query := `
		SELECT r.id, r.name, r.description, r.created_at,
		       p.id, p.name, p.resource, p.action, p.description, p.created_at
		FROM roles r
		INNER JOIN user_roles ur ON r.id = ur.role_id
		LEFT JOIN role_permissions rp ON r.id = rp.role_id
		LEFT JOIN permissions p ON rp.permission_id = p.id
		WHERE ur.user_id = $1
		ORDER BY r.name, p.name
	`

	rows, err := r.q.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []models.Role
	var lastID int

	for rows.Next() {
		var roleID int
		var roleName, roleDesc string
		var roleCreated time.Time
		var permID *int
		var permName, permResource, permAction, permDesc *string
		var permCreated *time.Time

		if err := rows.Scan(
			&roleID, &roleName, &roleDesc, &roleCreated,
			&permID, &permName, &permResource, &permAction, &permDesc, &permCreated,
		); err != nil {
			return nil, err
		}

		if len(roles) == 0 || roleID != lastID {
			roles = append(roles, models.Role{
				ID:          roleID,
				Name:        roleName,
				Description: roleDesc,
				CreatedAt:   roleCreated,
			})
			lastID = roleID
		}

		if permID != nil {
			cur := &roles[len(roles)-1]
			cur.Permissions = append(cur.Permissions, models.Permission{
				ID:          *permID,
				Name:        *permName,
				Resource:    *permResource,
				Action:      *permAction,
				Description: *permDesc,
				CreatedAt:   *permCreated,
			})
		}
	}

	return roles, rows.Err()
}

// AssignRoleToUser assigns a role to a user
func (r *Repository) AssignRoleToUser(ctx context.Context, userID uuid.UUID, roleName string) error {
	query := `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name = $2
		ON CONFLICT (user_id, role_id) DO NOTHING
	`

	_, err := r.q.Exec(ctx, query, userID, roleName)
	return err
}

// HashPassword hashes a password using bcrypt
func (r *Repository) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword verifies a password against a hash
func (r *Repository) CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// HashRefreshToken creates a hash of the refresh token
func (r *Repository) HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", hash)
}

// LoadRolePermissions returns the role->permissions mapping from the database,
// suitable for passing to auth.InitPermsFromDB.
func (r *Repository) LoadRolePermissions(ctx context.Context) (map[string][]string, error) {
	query := `
		SELECT r.name, p.name
		FROM roles r
		INNER JOIN role_permissions rp ON r.id = rp.role_id
		INNER JOIN permissions p ON rp.permission_id = p.id
		ORDER BY r.name, p.name
	`

	rows, err := r.q.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to load role permissions: %w", err)
	}
	defer rows.Close()

	mapping := make(map[string][]string)
	for rows.Next() {
		var roleName, permName string
		if err := rows.Scan(&roleName, &permName); err != nil {
			return nil, err
		}
		mapping[roleName] = append(mapping[roleName], permName)
	}
	return mapping, rows.Err()
}

// isUniqueViolation checks if the error is a unique constraint violation (PostgreSQL code 23505)
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

