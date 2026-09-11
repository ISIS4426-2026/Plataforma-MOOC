// Package admin implements the administrative management of accounts: listing
// users, changing roles, and suspending or reactivating them (issue #12).
package admin

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// Deps are the ports the service depends on.
type Deps struct {
	Admin        domain.AdminRepository
	Sessions     domain.SessionRepository
	SessionCache domain.SessionCache
}

// Actor is who is performing an administrative action, and from where. It is
// recorded on every audit entry so the trail answers "who did this" rather than
// only "what happened".
type Actor struct {
	ID        string
	IPAddress string
	UserAgent string
}

// Service exposes the administrative operations.
type Service struct {
	deps   Deps
	logger *slog.Logger
}

func NewService(deps Deps, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{deps: deps, logger: logger}
}

// ListUsers returns one page of accounts.
func (s *Service) ListUsers(ctx context.Context, filter domain.UserFilter) (*domain.UserPage, error) {
	if filter.Role != nil && !isKnownRole(*filter.Role) {
		return nil, fmt.Errorf("unknown role %q: %w", *filter.Role, domain.ErrInvalidInput)
	}
	if filter.Status != nil && !isKnownStatus(*filter.Status) {
		return nil, fmt.Errorf("unknown status %q: %w", *filter.Status, domain.ErrInvalidInput)
	}

	return s.deps.Admin.ListUsers(ctx, filter)
}

// ChangeRole assigns a new role to an account.
//
// Refusing to demote the last active administrator is enforced by the
// repository, inside the same transaction as the change, so two concurrent
// requests cannot each remove one and leave the platform with none.
func (s *Service) ChangeRole(ctx context.Context, actor Actor, targetID string, role domain.Role, opts domain.ChangeOptions) (*domain.User, error) {
	if !isKnownRole(role) {
		return nil, fmt.Errorf("unknown role %q: %w", role, domain.ErrInvalidInput)
	}

	entry := newEntry(actor, domain.AuditActionUserRoleChanged, targetID)

	return s.deps.Admin.ChangeRole(ctx, targetID, role, opts, entry)
}

// GetUser returns a single account so a client can read its ETag before
// attempting a conditional write.
func (s *Service) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	return s.deps.Admin.GetUser(ctx, userID)
}

// ChangeStatus suspends or reactivates an account.
//
// Suspending also revokes the account's sessions. Authentication already
// refuses a session whose user is not active, so this is belt and braces, but it
// closes the window in which a cached session would still resolve and it makes
// the intent explicit in the session record rather than implicit in a status
// check somewhere else.
func (s *Service) ChangeStatus(ctx context.Context, actor Actor, targetID string, status domain.UserStatus, opts domain.ChangeOptions) (*domain.User, error) {
	if !isKnownStatus(status) {
		return nil, fmt.Errorf("unknown status %q: %w", status, domain.ErrInvalidInput)
	}

	action := domain.AuditActionUserSuspended
	if status == domain.UserStatusActive {
		action = domain.AuditActionUserReactivated
	}

	entry := newEntry(actor, action, targetID)

	updated, err := s.deps.Admin.ChangeStatus(ctx, targetID, status, opts, entry)
	if err != nil {
		return nil, err
	}

	if status != domain.UserStatusActive {
		s.revokeSessions(ctx, updated.ID)
	}

	return updated, nil
}

// revokeSessions closes every session of an account.
//
// A failure here is logged rather than returned: the status change has already
// committed and is what actually denies access, so reporting an error would
// tell the administrator the suspension failed when it did not.
func (s *Service) revokeSessions(ctx context.Context, userID string) {
	revoked, err := s.deps.Sessions.RevokeAllForUser(ctx, userID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to revoke sessions of a suspended user",
			slog.String("user_id", userID),
			slog.String("error", err.Error()),
		)
		return
	}

	if err := s.deps.SessionCache.DeleteAllForUser(ctx, userID); err != nil {
		s.logger.ErrorContext(ctx, "failed to drop cached sessions of a suspended user",
			slog.String("user_id", userID),
			slog.String("error", err.Error()),
		)
		return
	}

	s.logger.InfoContext(ctx, "sessions revoked after suspension",
		slog.String("user_id", userID),
		slog.Int("revoked", revoked),
	)
}

// newEntry builds the audit entry for an administrative action.
//
// The before and after values are filled in by the repository, which is the
// only place that observes them inside the transaction that applies the change.
func newEntry(actor Actor, action domain.AuditAction, targetID string) *domain.AuditEntry {
	actorID := actor.ID

	return &domain.AuditEntry{
		ActorID:        &actorID,
		Action:         action,
		TargetResource: "user:" + targetID,
		IPAddress:      actor.IPAddress,
		UserAgent:      actor.UserAgent,
	}
}

func isKnownRole(role domain.Role) bool {
	switch role {
	case domain.RoleAdmin, domain.RoleProfessor, domain.RoleStudent:
		return true
	default:
		return false
	}
}

func isKnownStatus(status domain.UserStatus) bool {
	switch status {
	case domain.UserStatusActive, domain.UserStatusSuspended, domain.UserStatusPendingVerification:
		return true
	default:
		return false
	}
}
