package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/models"
)

// CreateUser creates a new user.
func (r *Repository) CreateUser(ctx context.Context, user *models.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	query := `
		INSERT INTO users (id, username, email, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`

	_, err := r.querier.Exec(ctx, query, user.ID, user.Username, user.Email, user.PasswordHash)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("user with this email already exists: %w", apperr.ErrConflict)
		}
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetUserByEmail retrieves a user by email with roles in a single query.
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT u.id, u.username, u.email, u.password_hash, u.created_at, u.updated_at,
		       r.id, r.name, r.description, r.created_at
		FROM users u
		LEFT JOIN user_roles ur ON u.id = ur.user_id
		LEFT JOIN roles r ON ur.role_id = r.id
		WHERE u.email = $1
		ORDER BY r.name
	`
	return r.scanUserWithRoles(ctx, query, email)
}

// GetUserByID retrieves a user by ID with roles in a single query.
func (r *Repository) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	query := `
		SELECT u.id, u.username, u.email, u.password_hash, u.created_at, u.updated_at,
		       r.id, r.name, r.description, r.created_at
		FROM users u
		LEFT JOIN user_roles ur ON u.id = ur.user_id
		LEFT JOIN roles r ON ur.role_id = r.id
		WHERE u.id = $1
		ORDER BY r.name
	`
	return r.scanUserWithRoles(ctx, query, userID)
}

// scanUserWithRoles executes a user+roles query and assembles the result.
func (r *Repository) scanUserWithRoles(ctx context.Context, query string, arg any) (*models.User, error) {
	rows, err := r.querier.Query(ctx, query, arg)
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}
	defer rows.Close()

	var user *models.User
	for rows.Next() {
		var u models.User
		var roleID *int
		var roleName, roleDesc *string
		var roleCreated *time.Time

		if err := rows.Scan(
			&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt,
			&roleID, &roleName, &roleDesc, &roleCreated,
		); err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}

		if user == nil {
			user = &u
		}
		if roleID != nil {
			user.Roles = append(user.Roles, models.Role{
				ID:          *roleID,
				Name:        *roleName,
				Description: *roleDesc,
				CreatedAt:   *roleCreated,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate user rows: %w", err)
	}
	if user == nil {
		return nil, apperr.ErrUserNotFound
	}
	return user, nil
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

	rows, err := r.querier.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get user roles: %w", err)
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
			return nil, fmt.Errorf("scan user role row: %w", err)
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user role rows: %w", err)
	}
	return roles, nil
}

// AssignRoleToUser assigns a role to a user.
// Returns an error if the role does not exist.
func (r *Repository) AssignRoleToUser(ctx context.Context, userID uuid.UUID, roleName string) error {
	query := `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name = $2
		ON CONFLICT (user_id, role_id) DO NOTHING
	`

	tag, err := r.querier.Exec(ctx, query, userID, roleName)
	if err != nil {
		return fmt.Errorf("failed to assign role %q: %w", roleName, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("role %q does not exist", roleName)
	}
	return nil
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

	rows, err := r.querier.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to load role permissions: %w", err)
	}
	defer rows.Close()

	mapping := make(map[string][]string)
	for rows.Next() {
		var roleName, permName string
		if err := rows.Scan(&roleName, &permName); err != nil {
			return nil, fmt.Errorf("scan role permission row: %w", err)
		}
		mapping[roleName] = append(mapping[roleName], permName)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate role permission rows: %w", err)
	}
	return mapping, nil
}

// isUniqueViolation checks if the error is a unique constraint violation (PostgreSQL code 23505)
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
