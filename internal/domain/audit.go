package domain

import (
	"context"
	"time"
)

// DefaultAuditPageSize and MaxAuditPageSize bound a trail listing the same
// way DefaultUserPageSize and MaxUserPageSize bound an admin one.
const (
	DefaultAuditPageSize = 20
	MaxAuditPageSize     = 100
)

// AuditFilter narrows and pages the audit trail. Every field is optional;
// the zero value lists everything, newest first.
type AuditFilter struct {
	// ActorID matches entries recorded for exactly this actor.
	ActorID string

	// ActionPrefix matches the start of the dotted action name, e.g.
	// "course." to find every course-related entry regardless of which one
	// happened — the reason actions are named `<recurso>.<hecho>` in the
	// first place.
	ActionPrefix string

	// TargetResource matches entries recorded against exactly this
	// `<tipo>:<identificador>` string.
	TargetResource string

	// From and To bound CreatedAt, inclusive on both ends. Either may be nil.
	From, To *time.Time

	Cursor string
	Limit  int
}

// AuditPage is one page of a trail listing.
type AuditPage struct {
	Entries []*AuditEntry
	// NextCursor is empty when there is nothing more to fetch.
	NextCursor string
	HasMore    bool
}

// AuditAction names something that happened and is worth keeping a record of.
//
// The values are dotted strings rather than an enum of integers so a log line
// or a database row is readable without a lookup table, and so a new action can
// be added without renumbering anything.
//
// The convention is `<recurso>.<hecho en pasado>`. Keeping the resource first
// makes it possible to filter a whole family with a prefix match.
type AuditAction string

const (
	AuditActionUserRoleChanged  AuditAction = "user.role_changed"
	AuditActionUserSuspended    AuditAction = "user.suspended"
	AuditActionUserReactivated  AuditAction = "user.reactivated"
	AuditActionSessionsRevoked  AuditAction = "user.sessions_revoked"
	AuditActionUserRegistered   AuditAction = "user.registered"
	AuditActionPasswordChanged  AuditAction = "user.password_changed"
	AuditActionEmailVerified    AuditAction = "user.email_verified"
	AuditActionLoginSucceeded   AuditAction = "auth.login_succeeded"
	AuditActionLoginFailed      AuditAction = "auth.login_failed"
	AuditActionCourseNewVersion AuditAction = "course.version_published"
	AuditActionCourseCreated    AuditAction = "course.created"
	AuditActionCourseUpdated    AuditAction = "course.updated"
)

// AuditEntry is one immutable record of an action.
//
// Nothing here is ever updated or deleted once written; that is what makes
// the trail worth having. It is immutable both by convention — nothing in
// this codebase exposes an update or delete path — and, since issue #18
// (migrations/000004_audit_immutability.up.sql), enforced at the database
// level: a trigger rejects any UPDATE or DELETE on audit_logs outright,
// regardless of which role or code path attempts it.
type AuditEntry struct {
	ID string `json:"id"`

	// ActorID is who performed the action, or nil when the platform acted on
	// its own. A nil actor is why the column is nullable: an entry with no
	// actor is meaningful, an entry with an invented one is not.
	ActorID *string `json:"actor_id,omitempty"`

	Action AuditAction `json:"action"`

	// TargetResource identifies what the action was performed on, as
	// `<tipo>:<identificador>`, e.g. "user:3fa85f64-...". A single string keeps
	// the trail usable across resource types without a column per type.
	TargetResource string `json:"target_resource"`

	// Details carries whatever else is worth knowing, such as the previous and
	// new value of a field. It must never hold credentials, tokens or password
	// hashes: the audit log is read by more people than the users table is.
	Details map[string]any `json:"details,omitempty"`

	// IPAddress and UserAgent describe where the action came from. Both are
	// taken from the connection and the request, never from forwarding headers
	// a caller can forge.
	IPAddress string `json:"ip_address,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// AuditRepository appends to and reads from the audit trail.
//
// # Contract for callers
//
// Record must not be called on its own for an action that changes state.
// "Every action is reflected in the audit trail" is only true if the change and
// its entry commit together; a caller that writes the change first and the
// entry afterwards loses the entry whenever the second write fails. The
// repositories that mutate state therefore take the entry and write both inside
// one transaction (see insertAuditEntry in internal/postgres), and Record
// exists for the actions that have nothing to commit alongside them.
//
// List is the read side (issue #18): filtering and cursor pagination for an
// administrator reviewing the trail. It never returns an entry that isn't
// already durably committed, which the database-level immutability guarantee
// makes safe to rely on.
//
// Retention and export (section 11 of the statement) are deliberately not
// part of this port: issue #18's acceptance criteria don't call for either,
// and building a policy nobody has specified yet (how long to keep entries,
// in what export format) would be guessing. Left as a note for whoever picks
// it up next rather than silently dropped.
type AuditRepository interface {
	Record(ctx context.Context, entry *AuditEntry) error

	// List returns one page of the audit trail, newest first, narrowed by
	// filter. Returns ErrInvalidInput if filter.Cursor is set but malformed.
	List(ctx context.Context, filter AuditFilter) (*AuditPage, error)
}
