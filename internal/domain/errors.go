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

	// ErrPreconditionFailed reports that the caller's If-Match no longer matches
	// the resource: somebody else changed it in between, and applying the write
	// would silently overwrite their change.
	ErrPreconditionFailed = errors.New("the resource changed since the version the caller holds")
)
