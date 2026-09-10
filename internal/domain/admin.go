package domain

import "context"

// DefaultUserPageSize is used when a listing request does not ask for a size.
const DefaultUserPageSize = 20

// MaxUserPageSize caps how much a single request can pull, so a client cannot
// turn the listing into a full table scan.
const MaxUserPageSize = 100

// UserFilter narrows and pages an administrative user listing.
type UserFilter struct {
	// Role and Status are optional; nil means "any".
	Role   *Role
	Status *UserStatus

	// Cursor is an opaque value from a previous page. Pagination is by cursor
	// rather than by offset because an offset shifts whenever a row is inserted
	// or removed, which makes a client silently skip or repeat users while it
	// pages through them.
	Cursor string
	Limit  int
}

// UserPage is one page of an administrative listing.
type UserPage struct {
	Users []*User
	// NextCursor is empty when there is nothing more to fetch.
	NextCursor string
	HasMore    bool
}

// AdminRepository backs the administrative management of accounts.
//
// ChangeRole and ChangeStatus take the audit entry rather than leaving the
// caller to record it separately, because "every action is reflected in the
// audit trail" is a guarantee, not a convention: the change and its entry are
// written in one transaction, so neither can exist without the other.
//
// Both also enforce the last-administrator rule inside that transaction. A
// caller that counted administrators first and then applied the change would
// race with itself: two concurrent requests could each observe two active
// administrators and each remove one, leaving the platform with none.
type AdminRepository interface {
	// ListUsers returns one page of accounts, newest first.
	ListUsers(ctx context.Context, filter UserFilter) (*UserPage, error)

	// ChangeRole assigns a new role.
	//
	// Returns ErrNotFound when the user does not exist, and
	// ErrLastAdminProtected when the change would demote the only remaining
	// active administrator.
	ChangeRole(ctx context.Context, userID string, role Role, entry *AuditEntry) (*User, error)

	// ChangeStatus suspends or reactivates an account.
	//
	// Returns ErrNotFound when the user does not exist, and
	// ErrLastAdminProtected when the change would suspend the only remaining
	// active administrator.
	ChangeStatus(ctx context.Context, userID string, status UserStatus, entry *AuditEntry) (*User, error)
}
