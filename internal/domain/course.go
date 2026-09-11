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

// The ten values match the CHECK constraint on resources.type in
// migrations/000001_init_schema.up.sql; adding a resource type means updating
// both.
const (
	ResourceTypeRichText     ResourceType = "text"
	ResourceTypeImage        ResourceType = "image"
	ResourceTypeVideo        ResourceType = "video"
	ResourceTypeAudio        ResourceType = "audio"
	ResourceTypePDF          ResourceType = "pdf"
	ResourceTypePresentation ResourceType = "presentation"
	ResourceTypeDownloadable ResourceType = "downloadable"
	ResourceTypeIframe       ResourceType = "iframe"
	ResourceTypeExternalLink ResourceType = "external_link"
	ResourceTypeQuiz         ResourceType = "quiz"
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
	ID          string       `json:"id"`
	StableID    string       `json:"stable_id"`
	UnitID      string       `json:"unit_id"`
	Title       string       `json:"title"`
	Type        ResourceType `json:"type"`
	Position    int          `json:"position"`
	IsVisible   bool         `json:"is_visible"`
	IsMandatory bool         `json:"is_mandatory"`
	// AllowDownload is independent of Type: a video or PDF can be
	// streamed/viewed-only or downloadable, section 3 of the spec lists it
	// alongside visibility and mandatoriness as a per-resource property.
	AllowDownload    bool      `json:"allow_download"`
	ContentText      string    `json:"content_text,omitempty"` // Extended canonical markdown
	ObjectKey        string    `json:"object_key,omitempty"`   // Reference in MinIO/S3
	ProcessingStatus string    `json:"processing_status,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
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

// The structural repositories below back issue #19: CRUD for Module, Unit
// and Resource, respecting the Curso -> Módulo -> Unidad -> Recurso
// hierarchy.
//
// Three conventions carry over unchanged from CourseRepository:
//
//   - The caller supplies ID and StableID, generated client-side, so the
//     audit entry recorded in the same transaction as the insert can name
//     the resource it just created.
//   - Every write takes the audit entry and commits it in the same
//     transaction as the change, per the #18 contract.
//   - Every write returns ErrCourseImmutable if the owning course is
//     currently published: the whole structure freezes with it, not only
//     the course row itself, so a published course can't grow a new module
//     any more than it can change its title.
//
// One is new here: Create returns ErrNotFound if the parent (course for a
// module, module for a unit, unit for a resource) does not exist -- issue
// #19's integrity rule that a unit can't be created without a module, nor a
// resource without a unit. The parent is locked (SELECT ... FOR UPDATE)
// inside the same transaction as the insert, which is what makes that check
// race-free against a concurrent delete of the same parent, and what makes
// it safe to read the parent's current sibling count to assign the new
// item's position -- always at the end, the trivial case of the
// InsertAt/Reposition algorithm in internal/domain/ordering.go, since
// nothing about creating an item requires moving any of its siblings.
type ModuleRepository interface {
	// Create inserts a new module at the end of course courseID.
	Create(ctx context.Context, module *Module, entry *AuditEntry) error

	GetByID(ctx context.Context, id string) (*Module, error)

	// ListByCourse returns every module of a course, ordered by position.
	ListByCourse(ctx context.Context, courseID string) ([]*Module, error)

	// Update renames a module.
	Update(ctx context.Context, moduleID, title string, entry *AuditEntry) (*Module, error)

	// Delete removes a module and closes the position gap it leaves behind
	// in its siblings, so positions stay a dense 0..n-1 sequence. Units and
	// resources beneath it are removed by the database's ON DELETE CASCADE.
	Delete(ctx context.Context, moduleID string, entry *AuditEntry) error
}

type UnitRepository interface {
	// Create inserts a new unit at the end of module moduleID.
	Create(ctx context.Context, unit *Unit, entry *AuditEntry) error

	GetByID(ctx context.Context, id string) (*Unit, error)

	// ListByModule returns every unit of a module, ordered by position.
	ListByModule(ctx context.Context, moduleID string) ([]*Unit, error)

	Update(ctx context.Context, unitID, title string, entry *AuditEntry) (*Unit, error)

	Delete(ctx context.Context, unitID string, entry *AuditEntry) error
}

// ResourceUpdate carries the fields of a resource a caller may change after
// creation. Type is not among them: changing what kind of resource a row is
// after the fact is a replace, not an edit. ProcessingStatus is not among
// them either: only the media-processing worker moves a resource through
// pending/processing/completed/failed, never a client request.
type ResourceUpdate struct {
	Title         string
	IsVisible     bool
	IsMandatory   bool
	AllowDownload bool
}

type ResourceRepository interface {
	// Create inserts a new resource at the end of unit unitID.
	Create(ctx context.Context, resource *Resource, entry *AuditEntry) error

	GetByID(ctx context.Context, id string) (*Resource, error)

	// ListByUnit returns every resource of a unit, ordered by position.
	ListByUnit(ctx context.Context, unitID string) ([]*Resource, error)

	Update(ctx context.Context, resourceID string, fields ResourceUpdate, entry *AuditEntry) (*Resource, error)

	Delete(ctx context.Context, resourceID string, entry *AuditEntry) error
}
