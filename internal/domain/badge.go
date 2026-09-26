package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// Badge is the credential a student earns by completing a course.
//
// UNIQUE (student_id, course_stable_id) in the schema is what makes issuing it
// idempotent: a student earns one badge per course, however many times the
// completing heartbeat is retried.
type Badge struct {
	ID             string `json:"id"`
	StudentID      string `json:"student_id"`
	CourseStableID string `json:"course_stable_id"`

	// VerificationCode is the public half of the credential: an unguessable
	// value the student can hand to anyone, which is what makes the badge
	// verifiable without an account.
	VerificationCode string `json:"verification_code"`

	// ImageKey is where the rendered badge image lives. It is derived from the
	// student and the course rather than stored by a caller, so the same badge
	// always resolves to the same object.
	//
	// Rendering the image is not part of issue #113 and no object exists at this
	// key yet. It is never handed to a client: a storage key is an internal
	// detail, and a public endpoint has no business exposing one.
	ImageKey string `json:"-"`

	IssuedAt  time.Time `json:"issued_at"`
	IsRevoked bool      `json:"is_revoked"`
}

// BadgeCredential is what verification answers: enough to confirm the badge is
// genuine and say what it certifies, and nothing more.
//
// It deliberately does not name the student. api/openapi.yaml calls this
// endpoint "public privacy-preserving badge verification ... without revealing
// student personal identity or email", and that is the contract: what a verifier
// needs is that the code corresponds to a real, unrevoked badge for a named
// course, and whoever handed them the code is the one claiming to have earned
// it. No name, no email, no identifiers, no storage key.
type BadgeCredential struct {
	CourseTitle string    `json:"course_title"`
	IssuedAt    time.Time `json:"issued_at"`
	IsRevoked   bool      `json:"revoked"`
}

// Valid reports whether the credential currently certifies anything.
func (c *BadgeCredential) Valid() bool {
	return c != nil && !c.IsRevoked
}

// ETag returns the entity tag of this badge, for the same reason and in the same
// shape as Course.ETag and User.ETag: computed here rather than in the HTTP
// layer, so the handler and the repository can never disagree on how.
//
// Unlike those two, badges have no updated_at column, so the tag is built from
// the fields that can actually change what a client sees. Only is_revoked ever
// moves after issuance, and including it is what makes a revocation invalidate a
// cached copy instead of leaving a client holding a badge the platform has
// withdrawn.
func (b *Badge) ETag() string {
	sum := sha256.Sum256([]byte(
		b.ID + "|" + b.IssuedAt.UTC().Format(time.RFC3339Nano) + "|" + strconv.FormatBool(b.IsRevoked),
	))
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

type BadgeRepository interface {
	// Verify resolves a verification code, or ErrNotFound when no badge carries
	// it. A revoked badge resolves: "this was issued and then revoked" is the
	// useful answer, and hiding it would be indistinguishable from a forgery.
	Verify(ctx context.Context, code string) (*BadgeCredential, error)

	// GetByID returns one badge, or ErrNotFound. Whether the caller is allowed to
	// see it is decided above this layer.
	GetByID(ctx context.Context, badgeID string) (*Badge, error)

	// GetByStudentAndCourse returns the student's badge for a course, or
	// ErrNotFound when they have not earned it.
	GetByStudentAndCourse(ctx context.Context, studentID, courseStableID string) (*Badge, error)
}

// BadgeImageKey is where a badge's image belongs. Derived, never stored by a
// caller, so it cannot drift between the two.
func BadgeImageKey(studentID, courseStableID string) string {
	return "badges/" + courseStableID + "/" + studentID + ".png"
}

// BadgeVerificationURL is the address a student hands to someone who wants to
// check their badge.
//
// Built against the platform's public origin, the same one activation and
// password-reset links are built against, and following the same convention: it
// has to be an address the recipient's browser can reach, not a container
// hostname. It points at the API's verification endpoint because that is what
// exists and answers; when a frontend is deployed, this is the one place that has
// to learn about it.
func BadgeVerificationURL(appBaseURL, verificationCode string) string {
	if appBaseURL == "" || verificationCode == "" {
		// Better no link than one pointing at a host nobody configured. The code
		// alone is enough to verify a badge.
		return ""
	}
	return strings.TrimRight(appBaseURL, "/") + "/api/v1/badges/verify/" + verificationCode
}
