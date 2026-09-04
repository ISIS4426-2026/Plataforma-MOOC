package domain

import (
	"context"
	"time"
)

type ProgressEvent struct {
	ID               string    `json:"id"`
	StudentID        string    `json:"student_id"`
	CourseStableID   string    `json:"course_stable_id"`
	ResourceStableID string    `json:"resource_stable_id"`
	DwellTimeSeconds int       `json:"dwell_time_seconds"`
	Completed        bool      `json:"completed"`
	Timestamp        time.Time `json:"timestamp"`
}

type StudentProgress struct {
	StudentID          string    `json:"student_id"`
	CourseStableID     string    `json:"course_stable_id"`
	CompletedResources []string  `json:"completed_resources"` // Array of ResourceStableIDs
	PercentCompleted   float64   `json:"percent_completed"`
	IsApproved         bool      `json:"is_approved"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type ProgressRepository interface {
	RecordEvent(ctx context.Context, event *ProgressEvent) error
	GetStudentProgress(ctx context.Context, studentID, courseStableID string) (*StudentProgress, error)
}
