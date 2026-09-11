package domain

import (
	"context"
	"time"
)

// RecordedResponse is the answer a request produced, kept so a retry carrying
// the same Idempotency-Key can be given the same answer instead of running the
// operation again.
//
// Only what a client needs to reconstruct the reply is stored. Headers the
// server regenerates per request, such as the correlation id or the rate limit
// counters, are deliberately left out: replaying them would attribute one
// request's identifiers to another.
type RecordedResponse struct {
	Status int    `json:"status"`
	Body   []byte `json:"body"`
	// ContentType is kept because a replay has to be decodable the same way the
	// original was.
	ContentType string `json:"content_type"`

	// RequestFingerprint is a digest of the request that produced this
	// response. Reusing a key with a different payload is a client defect, and
	// silently returning the first answer would hide it.
	RequestFingerprint string `json:"request_fingerprint"`
}

// IdempotencyStore remembers the outcome of a request against its key.
//
// It is separate from the worker's idempotency store, which answers "was this
// task already processed" for asynq jobs. This one has to return the original
// response, not just a boolean, so a retry is indistinguishable from the first
// call.
type IdempotencyStore interface {
	// Reserve claims a key for a request that is about to run.
	//
	// It returns the stored response when the key was already used, and
	// inProgress when another request holds the key but has not finished yet.
	// The claim must be atomic: two concurrent retries of the same key must not
	// both be told to proceed, or the operation runs twice, which is precisely
	// what the key exists to prevent.
	Reserve(ctx context.Context, key string, fingerprint string, ttl time.Duration) (recorded *RecordedResponse, inProgress bool, err error)

	// Store attaches the outcome to a key claimed by Reserve.
	Store(ctx context.Context, key string, response *RecordedResponse, ttl time.Duration) error

	// Release drops a claim without recording an outcome, so a request that
	// failed before producing a reusable answer does not leave the key blocked
	// until it expires.
	Release(ctx context.Context, key string) error
}
