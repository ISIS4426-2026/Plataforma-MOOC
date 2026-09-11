package domain

import (
	"context"
	"time"
)

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
// Nothing here is ever updated or deleted once written; that is what makes the
// trail worth having. Enforcing it at the database level is the subject of
// issue #18 and is not done yet, see the notes on AuditRepository.
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

// AuditRepository appends entries to the audit trail.
//
// # Contract for issue #18
//
// This port is intentionally write-only and single-method. Issue #18 owns
// making the trail genuinely immutable and queryable, and should extend this
// rather than replace it, so the callers already recording entries keep
// working. What that issue still has to provide:
//
//   - Immutability at the database level. Today nothing stops an UPDATE or a
//     DELETE on audit_logs; a trail that can be edited is not a trail. The usual
//     shape is revoking those grants from the application role and adding a rule
//     or trigger that rejects them.
//   - A read side. Listing and filtering by actor, action, target and time range,
//     with the cursor pagination the rest of the API uses.
//   - Retention and export, per section 11 of the statement.
//
// # Contract for callers
//
// Record must not be called on its own for an action that changes state.
// "Every action is reflected in the audit trail" is only true if the change and
// its entry commit together; a caller that writes the change first and the
// entry afterwards loses the entry whenever the second write fails. The
// repositories that mutate state therefore take the entry and write both inside
// one transaction, and Record exists for the actions that have nothing to
// commit alongside them.
type AuditRepository interface {
	Record(ctx context.Context, entry *AuditEntry) error
}
