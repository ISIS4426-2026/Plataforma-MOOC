package domain

import (
	"context"
	"time"
)

// ProgressEvent is one report of a student's activity on a resource.
//
// The table behind it is an append-only log: every heartbeat is a fact that
// happened and is never rewritten. StudentProgress is the projection derived
// from it, and the one clients read.
type ProgressEvent struct {
	ID               string    `json:"id"`
	StudentID        string    `json:"student_id"`
	CourseStableID   string    `json:"course_stable_id"`
	ResourceStableID string    `json:"resource_stable_id"`
	DwellTimeSeconds int       `json:"dwell_time_seconds"`
	Completed        bool      `json:"completed"`
	Timestamp        time.Time `json:"timestamp"`
}

// MaxHeartbeatDwellSeconds bounds a single report at one hour.
//
// A heartbeat is periodic, so any one of them covers seconds or minutes. The
// cap is what keeps a client -- buggy or dishonest -- from claiming a year of
// study in one request, since dwell time is summed and never recomputed from
// anything the server observed.
const MaxHeartbeatDwellSeconds = 3600

// Heartbeat is the input to recording progress.
//
// It names the resource by its row id, the way every other route addresses
// resources, and lets the server derive the course and the stable ids. A client
// that could name the course itself could claim progress on a course it never
// opened.
type Heartbeat struct {
	StudentID        string
	ResourceID       string
	DwellTimeSeconds int
	Completed        bool
}

// StudentProgress is how far a student has got in a course.
//
// It is keyed by the course's stable id, not a version's row id, which is what
// makes progress survive the publication of a new version.
type StudentProgress struct {
	StudentID      string `json:"student_id"`
	CourseStableID string `json:"course_stable_id"`

	// CompletedResources holds resource stable ids. It is a set: recording the
	// same resource twice leaves it unchanged, which is what keeps repeated
	// heartbeats from inflating the percentage.
	CompletedResources []string `json:"completed_resources"`

	// CompletedCount and TotalCount are the two sides of the percentage, and
	// are reported alongside it so a client can show "3 de 8" without having to
	// invert a rounded number.
	//
	// They count only the resources that are both mandatory and visible: an
	// optional resource should not stand between a student and completing the
	// course, and a hidden one cannot be reached at all.
	CompletedCount int `json:"completed_count"`
	TotalCount     int `json:"total_count"`

	PercentCompleted float64   `json:"percent_completed"`
	IsApproved       bool      `json:"is_approved"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ResourceLocation is where a resource sits, resolved from its row id: the
// course version it belongs to and the two stable identities progress is keyed
// by.
type ResourceLocation struct {
	ResourceStableID string
	CourseID         string
	CourseStableID   string
	CourseStatus     CourseStatus
	CourseAuthorID   string
}

type ProgressRepository interface {
	// Locate resolves a resource row id to the course it belongs to, so the
	// caller can check access before recording anything.
	Locate(ctx context.Context, resourceID string) (*ResourceLocation, error)

	// RecordHeartbeat appends the event and recomputes the projection in one
	// transaction, and issues the course's badge when this heartbeat is the one
	// that completes it.
	//
	// It returns the recomputed progress, and the badge when this call is what
	// earned it -- nil on every later call, since a badge is issued once.
	RecordHeartbeat(ctx context.Context, hb Heartbeat, at *ResourceLocation, entry *AuditEntry) (*StudentProgress, *Badge, error)

	// GetStudentProgress returns the projection, recomputing the counts against
	// the course's current resources rather than trusting stored totals.
	//
	// A student who never reported anything gets a zeroed progress, not
	// ErrNotFound: "how far am I" has an answer before the first heartbeat.
	GetStudentProgress(ctx context.Context, studentID, courseID string) (*StudentProgress, error)
}
