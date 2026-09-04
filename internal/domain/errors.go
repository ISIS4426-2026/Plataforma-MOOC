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
)
