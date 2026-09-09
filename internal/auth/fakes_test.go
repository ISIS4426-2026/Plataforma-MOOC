package auth_test

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// In-memory doubles for the domain ports. They are intentionally simple but
// preserve the behaviour the service relies on: uniqueness of emails and atomic
// single-use token consumption.

type fakeUserRepo struct {
	mu    sync.Mutex
	users map[string]*domain.User // by id
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: make(map[string]*domain.User)}
}

func (f *fakeUserRepo) Create(_ context.Context, user *domain.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, existing := range f.users {
		if strings.EqualFold(existing.Email, user.Email) {
			return domain.ErrConflict
		}
	}

	user.ID = uuid.NewString()
	user.CreatedAt = time.Now()
	user.UpdatedAt = user.CreatedAt

	stored := *user
	f.users[user.ID] = &stored
	return nil
}

func (f *fakeUserRepo) GetByID(_ context.Context, id string) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	user, ok := f.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copied := *user
	return &copied, nil
}

func (f *fakeUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, user := range f.users {
		if user.Email == email {
			copied := *user
			return &copied, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeUserRepo) Update(_ context.Context, user *domain.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.users[user.ID]; !ok {
		return domain.ErrNotFound
	}

	user.UpdatedAt = time.Now()
	stored := *user
	f.users[user.ID] = &stored
	return nil
}

func (f *fakeUserRepo) CountActiveAdmins(context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	count := 0
	for _, user := range f.users {
		if user.Role == domain.RoleAdmin && user.Status == domain.UserStatusActive {
			count++
		}
	}
	return count, nil
}

type fakeTokenRepo struct {
	mu     sync.Mutex
	tokens map[string]*domain.VerificationToken // by token hash
}

func newFakeTokenRepo() *fakeTokenRepo {
	return &fakeTokenRepo{tokens: make(map[string]*domain.VerificationToken)}
}

func (f *fakeTokenRepo) Create(_ context.Context, token *domain.VerificationToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	token.ID = uuid.NewString()
	token.CreatedAt = time.Now()

	stored := *token
	f.tokens[token.TokenHash] = &stored
	return nil
}

// Consume mirrors the atomic UPDATE of the real repository: the check and the
// write happen while holding the lock, so a replay cannot win a race.
func (f *fakeTokenRepo) Consume(_ context.Context, tokenHash string, now time.Time) (*domain.VerificationToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	token, ok := f.tokens[tokenHash]
	if !ok || token.ConsumedAt != nil || !now.Before(token.ExpiresAt) {
		return nil, domain.ErrNotFound
	}

	consumed := now
	token.ConsumedAt = &consumed

	copied := *token
	return &copied, nil
}

func (f *fakeTokenRepo) InvalidateForUser(_ context.Context, userID string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, token := range f.tokens {
		if token.UserID == userID && token.ConsumedAt == nil {
			consumed := now
			token.ConsumedAt = &consumed
		}
	}
	return nil
}

type sentMessage struct {
	To      string
	Subject string
	Body    string
}

type fakeMailer struct {
	mu       sync.Mutex
	messages []sentMessage
	err      error
}

func newFakeMailer() *fakeMailer { return &fakeMailer{} }

func (f *fakeMailer) Send(_ context.Context, to, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return f.err
	}

	f.messages = append(f.messages, sentMessage{To: to, Subject: subject, Body: body})
	return nil
}

func (f *fakeMailer) last() (sentMessage, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.messages) == 0 {
		return sentMessage{}, false
	}
	return f.messages[len(f.messages)-1], true
}
