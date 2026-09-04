package domain

import (
	"context"
	"time"
)

type QuestionOption struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	IsCorrect bool   `json:"-"` // Omitted from client serialization for security
}

type Question struct {
	ID       string           `json:"id"`
	QuizID   string           `json:"quiz_id"`
	Prompt   string           `json:"prompt"`
	Position int              `json:"position"`
	Options  []QuestionOption `json:"options"`
}

type Quiz struct {
	ID           string     `json:"id"`
	ResourceID   string     `json:"resource_id"`
	Title        string     `json:"title"`
	PassingScore int        `json:"passing_score"` // Percentage, e.g., 70
	Questions    []Question `json:"questions,omitempty"`
}

type QuizSubmission struct {
	ID          string            `json:"id"`
	QuizID      string            `json:"quiz_id"`
	StudentID   string            `json:"student_id"`
	Answers     map[string]string `json:"answers"` // QuestionID -> SelectedOptionID
	Score       int               `json:"score"`
	Passed      bool              `json:"passed"`
	SubmittedAt time.Time         `json:"submitted_at"`
}

type QuizRepository interface {
	GetByResourceID(ctx context.Context, resourceID string) (*Quiz, error)
	SaveSubmission(ctx context.Context, sub *QuizSubmission) error
	GetLatestSubmission(ctx context.Context, studentID, quizID string) (*QuizSubmission, error)
}
