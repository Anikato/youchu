package catalog

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrVersion          = errors.New("version conflict")
	ErrParent           = errors.New("invalid parent")
	ErrCycle            = errors.New("location cycle")
	ErrInUse            = errors.New("location in use")
	ErrCodeTaken        = errors.New("code taken")
	ErrNameTaken        = errors.New("name taken")
	ErrAlreadyCompleted = errors.New("already completed")
	ErrPhotoLimit       = errors.New("photo limit")
	ErrIconInUse        = errors.New("icon in use")
)

type FieldError struct {
	Fields map[string]string
}

func (e *FieldError) Error() string { return "invalid fields" }

func fieldError(m map[string]string) error {
	if len(m) == 0 {
		return nil
	}
	return &FieldError{Fields: m}
}
