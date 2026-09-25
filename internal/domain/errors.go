package domain

import "errors"

// Standard domain errors decoupled from transport/HTTP layer.
var (
	ErrNotFound           = errors.New("resource not found")
	ErrUnauthorized       = errors.New("unauthorized access")
	ErrForbidden          = errors.New("action forbidden for current role or owner")
	ErrInvalidInput       = errors.New("invalid input data")
	ErrConflict           = errors.New("resource conflict or duplicate entry")
	ErrInternal           = errors.New("internal domain error")
	ErrLastAdminProtected = errors.New("the last active administrator cannot be modified or deleted")
	ErrCourseImmutable    = errors.New("published course version is immutable")

	// ErrObjectNotFound reports that the bucket holds nothing at the key. It is
	// what separates "the upload never landed" from a provider outage, so the
	// confirmation endpoint can answer 409 instead of 500.
	ErrObjectNotFound = errors.New("object not found in storage")

	// ErrTaskAlreadyQueued reports that a job with this idempotency key is
	// already queued or was just processed. For an idempotent caller that is
	// success, not failure: the work it asked for exists.
	ErrTaskAlreadyQueued = errors.New("a task with this idempotency key is already queued")

	// ErrPreconditionFailed reports that the caller's If-Match no longer matches
	// the resource: somebody else changed it in between, and applying the write
	// would silently overwrite their change.
	ErrPreconditionFailed = errors.New("the resource changed since the version the caller holds")
)
