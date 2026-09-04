package domain

import (
	"context"
	"time"
)

type Badge struct {
	ID               string    `json:"id"`
	StudentID        string    `json:"student_id"`
	CourseStableID   string    `json:"course_stable_id"`
	VerificationCode string    `json:"verification_code"` // Public verification UUID
	ImageKey         string    `json:"image_key"`
	IssuedAt         time.Time `json:"issued_at"`
	IsRevoked        bool      `json:"is_revoked"`
}

type BadgeRepository interface {
	Create(ctx context.Context, badge *Badge) error
	GetByVerificationCode(ctx context.Context, code string) (*Badge, error)
	GetByStudentAndCourse(ctx context.Context, studentID, courseStableID string) (*Badge, error)
}
