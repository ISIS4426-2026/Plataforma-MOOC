package domain

import (
	"context"
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

type CourseRepository interface {
	Create(ctx context.Context, course *Course) error
	GetByID(ctx context.Context, id string) (*Course, error)
	GetByStableIDAndVersion(ctx context.Context, stableID string, version int) (*Course, error)
	ListPublished(ctx context.Context, cursor string, limit int) ([]*Course, string, error)
	Update(ctx context.Context, course *Course) error
}
