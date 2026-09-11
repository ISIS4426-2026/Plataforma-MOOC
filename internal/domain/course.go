package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type CourseStatus string

const (
	CourseStatusDraft       CourseStatus = "draft"
	CourseStatusPublished   CourseStatus = "published"
	CourseStatusUnpublished CourseStatus = "unpublished"
)

type ResourceType string

const (
	ResourceTypeRichText ResourceType = "text"
	ResourceTypeImage    ResourceType = "image"
	ResourceTypeVideo    ResourceType = "video"
	ResourceTypeAudio    ResourceType = "audio"
	ResourceTypePDF      ResourceType = "pdf"
	ResourceTypeQuiz     ResourceType = "quiz"
)

// Course aggregate root.
type Course struct {
	ID          string       `json:"id"`
	StableID    string       `json:"stable_id"` // Persistent identifier across versions
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Version     int          `json:"version"`
	Status      CourseStatus `json:"status"`
	AuthorID    string       `json:"author_id"`
	Modules     []Module     `json:"modules,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// ETag returns the entity tag of this course row, for the same reason and in
// the same shape as User.ETag: derived from the identifier and updated_at, so
// it changes exactly when the row does, and computed here rather than in the
// HTTP layer so the handler and the repository never disagree on how.
func (c *Course) ETag() string {
	sum := sha256.Sum256([]byte(c.ID + "|" + c.UpdatedAt.UTC().Format(time.RFC3339Nano)))
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

type Module struct {
	ID        string    `json:"id"`
	StableID  string    `json:"stable_id"`
	CourseID  string    `json:"course_id"`
	Title     string    `json:"title"`
	Position  int       `json:"position"`
	Units     []Unit    `json:"units,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Unit struct {
	ID        string     `json:"id"`
	StableID  string     `json:"stable_id"`
	ModuleID  string     `json:"module_id"`
	Title     string     `json:"title"`
	Position  int        `json:"position"`
	Resources []Resource `json:"resources,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type Resource struct {
	ID               string       `json:"id"`
	StableID         string       `json:"stable_id"`
	UnitID           string       `json:"unit_id"`
	Title            string       `json:"title"`
	Type             ResourceType `json:"type"`
	Position         int          `json:"position"`
	IsVisible        bool         `json:"is_visible"`
	IsMandatory      bool         `json:"is_mandatory"`
	ContentText      string       `json:"content_text,omitempty"` // Extended canonical markdown
	ObjectKey        string       `json:"object_key,omitempty"`   // Reference in MinIO/S3
	ProcessingStatus string       `json:"processing_status,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
}

// DefaultCoursePageSize and MaxCoursePageSize bound a catalog listing the
// same way DefaultUserPageSize and MaxUserPageSize bound an admin one.
const (
	DefaultCoursePageSize = 20
	MaxCoursePageSize     = 100
)

// CourseFilter narrows and pages the published course catalog.
type CourseFilter struct {
	// Search matches title or description; empty means no filtering by text.
	Search string
	Cursor string
	Limit  int
}

// CoursePage is one page of a catalog listing.
type CoursePage struct {
	Courses []*Course
	// NextCursor is empty when there is nothing more to fetch.
	NextCursor string
	HasMore    bool
}

// CourseRepository backs course authoring: creating a draft, reading it back,
// listing the published catalog, and editing a draft's metadata.
//
// Create and Update take the audit entry rather than leaving the caller to
// record it separately, for the same reason AdminRepository.ChangeRole and
// ChangeStatus do: "every authoring action is reflected in the audit trail"
// (issue #18) is only true if the change and its entry commit together, so
// both are written inside one transaction.
type CourseRepository interface {
	// Create inserts a new course draft. The caller supplies ID and StableID
	// so the audit entry recorded in the same transaction can name the
	// resource it just created; the repository fills in CreatedAt and
	// UpdatedAt, which the schema defaults and no caller could know in advance.
	Create(ctx context.Context, course *Course, entry *AuditEntry) error

	// GetByID returns a single course row — one specific version — so a
	// caller can read its ETag before attempting a conditional write.
	GetByID(ctx context.Context, id string) (*Course, error)

	// GetByStableIDAndVersion returns one version of a course by its stable
	// identity, e.g. to resolve the currently published version.
	GetByStableIDAndVersion(ctx context.Context, stableID string, version int) (*Course, error)

	// ListPublished returns one page of published courses, newest first.
	ListPublished(ctx context.Context, filter CourseFilter) (*CoursePage, error)

	// Update changes a draft's title and description.
	//
	// Returns ErrCourseImmutable if the course's current status is
	// CourseStatusPublished: a published version is never edited in place,
	// only after being unpublished. Returns ErrPreconditionFailed if
	// opts.ExpectedETag no longer matches, checked inside the same
	// transaction that applies the change so a concurrent write can't slip
	// between the check and the update.
	Update(ctx context.Context, courseID string, title, description string, opts ChangeOptions, entry *AuditEntry) (*Course, error)
}
