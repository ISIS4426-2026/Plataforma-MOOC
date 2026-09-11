package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type Role string

const (
	RoleAdmin     Role = "administrador"
	RoleProfessor Role = "profesor"
	RoleStudent   Role = "estudiante"
)

type UserStatus string

const (
	UserStatusPendingVerification UserStatus = "pending_verification"
	UserStatusActive              UserStatus = "active"
	UserStatusSuspended           UserStatus = "suspended"
)

// User represents a system actor (Student, Professor, or Admin).
type User struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	FullName     string     `json:"full_name"`
	Role         Role       `json:"role"`
	Status       UserStatus `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// ETag returns the entity tag of this representation.
//
// It is derived from the identifier and updated_at, which the database bumps on
// every write, so the tag changes exactly when the resource does. Hashing rather
// than exposing the timestamp keeps clients from inferring an ordering or a
// write schedule from a value they are only meant to compare.
//
// Defining it here rather than in the HTTP layer is deliberate: the handler
// compares the tag a client sent and the repository verifies it inside the
// transaction that applies the change, and the two must never disagree on how
// it is computed.
func (u *User) ETag() string {
	sum := sha256.Sum256([]byte(u.ID + "|" + u.UpdatedAt.UTC().Format(time.RFC3339Nano)))
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

// UserRepository interface defines persistence capabilities for users.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	CountActiveAdmins(ctx context.Context) (int, error)
}
