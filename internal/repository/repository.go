package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"

	"github.com/fedutinova/smartheart/internal/database"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Repository struct {
	db *database.DB
}

func New(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateRequest(ctx context.Context, req *models.Request) error {
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}

	query := `
		INSERT INTO requests (id, user_id, text_query, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`

	var textQuery sql.NullString
	if req.TextQuery != nil {
		textQuery = sql.NullString{String: *req.TextQuery, Valid: true}
	}

	_, err := r.db.Pool().Exec(ctx, query, req.ID, req.UserID, textQuery, req.Status)
	return err
}

func (r *Repository) CreateFile(ctx context.Context, file *models.File) error {
	if file.ID == uuid.Nil {
		file.ID = uuid.New()
	}

	query := `
		INSERT INTO files (id, request_id, original_filename, file_type, file_size, s3_bucket, s3_key, s3_url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
	`

	_, err := r.db.Pool().Exec(ctx, query,
		file.ID,
		file.RequestID,
		file.OriginalFilename,
		file.FileType,
		file.FileSize,
		file.S3Bucket,
		file.S3Key,
		file.S3URL,
	)
	return err
}

func (r *Repository) GetRequestByID(ctx context.Context, id uuid.UUID) (*models.Request, error) {
	query := `
		SELECT id, user_id, text_query, status, created_at, updated_at
		FROM requests
		WHERE id = $1
	`

	var req models.Request
	var textQuery sql.NullString

	err := r.db.Pool().QueryRow(ctx, query, id).Scan(
		&req.ID,
		&req.UserID,
		&textQuery,
		&req.Status,
		&req.CreatedAt,
		&req.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if textQuery.Valid {
		req.TextQuery = &textQuery.String
	}

	files, err := r.GetFilesByRequestID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get files: %w", err)
	}
	req.Files = files

	response, err := r.GetResponseByRequestID(ctx, id)
	if err != nil && err.Error() != "no rows in result set" {
		return nil, fmt.Errorf("failed to get response: %w", err)
	}
	req.Response = response

	return &req, nil
}

func (r *Repository) GetFilesByRequestID(ctx context.Context, requestID uuid.UUID) ([]models.File, error) {
	query := `
		SELECT id, request_id, original_filename, file_type, file_size, s3_bucket, s3_key, s3_url, created_at
		FROM files
		WHERE request_id = $1
		ORDER BY created_at
	`

	rows, err := r.db.Pool().Query(ctx, query, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []models.File
	for rows.Next() {
		var file models.File
		err := rows.Scan(
			&file.ID,
			&file.RequestID,
			&file.OriginalFilename,
			&file.FileType,
			&file.FileSize,
			&file.S3Bucket,
			&file.S3Key,
			&file.S3URL,
			&file.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}

	return files, rows.Err()
}

func (r *Repository) GetResponseByRequestID(ctx context.Context, requestID uuid.UUID) (*models.Response, error) {
	query := `
		SELECT id, request_id, content, model, tokens_used, processing_time_ms, created_at
		FROM responses
		WHERE request_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var resp models.Response
	err := r.db.Pool().QueryRow(ctx, query, requestID).Scan(
		&resp.ID,
		&resp.RequestID,
		&resp.Content,
		&resp.Model,
		&resp.TokensUsed,
		&resp.ProcessingTimeMs,
		&resp.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (r *Repository) CreateUser(ctx context.Context, user *models.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	query := `
		INSERT INTO users (id, username, email, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`

	_, err := r.db.Pool().Exec(ctx, query, user.ID, user.Username, user.Email, user.PasswordHash)
	return err
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	var user models.User
	err := r.db.Pool().QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	roles, err := r.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}
	user.Roles = roles

	return &user, nil
}

func (r *Repository) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err := r.db.Pool().QueryRow(ctx, query, userID).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	roles, err := r.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}
	user.Roles = roles

	return &user, nil
}

func (r *Repository) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]models.Role, error) {
	query := `
		SELECT r.id, r.name, r.description, r.created_at
		FROM roles r
		INNER JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY r.name
	`

	rows, err := r.db.Pool().Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var role models.Role
		err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt)
		if err != nil {
			return nil, err
		}

		permissions, err := r.GetRolePermissions(ctx, role.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get role permissions: %w", err)
		}
		role.Permissions = permissions

		roles = append(roles, role)
	}

	return roles, rows.Err()
}

func (r *Repository) GetRolePermissions(ctx context.Context, roleID int) ([]models.Permission, error) {
	query := `
		SELECT p.id, p.name, p.resource, p.action, p.description, p.created_at
		FROM permissions p
		INNER JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		ORDER BY p.name
	`

	rows, err := r.db.Pool().Query(ctx, query, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []models.Permission
	for rows.Next() {
		var perm models.Permission
		err := rows.Scan(&perm.ID, &perm.Name, &perm.Resource, &perm.Action, &perm.Description, &perm.CreatedAt)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, perm)
	}

	return permissions, rows.Err()
}

func (r *Repository) AssignRoleToUser(ctx context.Context, userID uuid.UUID, roleName string) error {
	query := `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name = $2
		ON CONFLICT (user_id, role_id) DO NOTHING
	`

	_, err := r.db.Pool().Exec(ctx, query, userID, roleName)
	return err
}

func (r *Repository) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func (r *Repository) CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (r *Repository) CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}

	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`

	_, err := r.db.Pool().Exec(ctx, query, token.ID, token.UserID, token.TokenHash, token.ExpiresAt)
	return err
}

func (r *Repository) GetRefreshToken(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, created_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1 AND expires_at > NOW() AND revoked_at IS NULL
	`

	var token models.RefreshToken
	err := r.db.Pool().QueryRow(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.CreatedAt,
		&token.RevokedAt,
	)
	if err != nil {
		return nil, err
	}

	return &token, nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE token_hash = $1
	`

	_, err := r.db.Pool().Exec(ctx, query, tokenHash)
	return err
}

func (r *Repository) HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", hash)
}

func (r *Repository) GetRequestsByUserID(ctx context.Context, userID uuid.UUID) ([]models.Request, error) {
	query := `
		SELECT id, user_id, text_query, status, created_at, updated_at
		FROM requests
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool().Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []models.Request
	for rows.Next() {
		var req models.Request
		var textQuery sql.NullString

		err := rows.Scan(
			&req.ID,
			&req.UserID,
			&textQuery,
			&req.Status,
			&req.CreatedAt,
			&req.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if textQuery.Valid {
			req.TextQuery = &textQuery.String
		}

		requests = append(requests, req)
	}

	return requests, rows.Err()
}
