package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system.
type User struct {
	ID                    uuid.UUID  `json:"id"                                db:"id"`
	Username              string     `json:"username"                          db:"username"`
	Email                 string     `json:"email"                             db:"email"`
	PasswordHash          string     `json:"-"                                 db:"password_hash"`
	CreatedAt             time.Time  `json:"created_at"                        db:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"                        db:"updated_at"`
	Roles                 []Role     `json:"roles,omitempty"`
	SubscriptionExpiresAt *time.Time `json:"subscription_expires_at,omitempty" db:"subscription_expires_at"`
	// ConsentGivenAt / ConsentVersion record the user's consent to personal-data
	// processing (152-ФЗ): when it was given and which policy version was agreed.
	ConsentGivenAt *time.Time `json:"consent_given_at,omitempty" db:"consent_given_at"`
	ConsentVersion string     `json:"consent_version,omitempty"  db:"consent_version"`
}

// Role represents a user role.
type Role struct {
	ID          int          `json:"id"                    db:"id"`
	Name        string       `json:"name"                  db:"name"`
	Description string       `json:"description,omitempty" db:"description"`
	CreatedAt   time.Time    `json:"created_at"            db:"created_at"`
	Permissions []Permission `json:"permissions,omitempty"`
}

// Permission represents a permission in the RBAC system.
type Permission struct {
	ID          int       `json:"id"                    db:"id"`
	Name        string    `json:"name"                  db:"name"`
	Resource    string    `json:"resource"              db:"resource"`
	Action      string    `json:"action"                db:"action"`
	Description string    `json:"description,omitempty" db:"description"`
	CreatedAt   time.Time `json:"created_at"            db:"created_at"`
}

type PasswordResetToken struct {
	ID        uuid.UUID  `json:"id"                db:"id"`
	UserID    uuid.UUID  `json:"user_id"           db:"user_id"`
	TokenHash string     `json:"-"                 db:"token_hash"`
	ExpiresAt time.Time  `json:"expires_at"        db:"expires_at"`
	CreatedAt time.Time  `json:"created_at"        db:"created_at"`
	UsedAt    *time.Time `json:"used_at,omitempty" db:"used_at"`
}

// RefreshToken represents a refresh token for JWT authentication.
type RefreshToken struct {
	ID        uuid.UUID  `json:"id"                   db:"id"`
	UserID    uuid.UUID  `json:"user_id"              db:"user_id"`
	TokenHash string     `json:"-"                    db:"token_hash"`
	ExpiresAt time.Time  `json:"expires_at"           db:"expires_at"`
	CreatedAt time.Time  `json:"created_at"           db:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
}
