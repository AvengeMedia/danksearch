package errdefs

type ErrorType int

const (
	ErrTypeIndexNotFound ErrorType = iota
	ErrTypeIndexCorrupted
	ErrTypeIndexingFailed
	ErrTypeSearchFailed
	ErrTypeWatcherFailed
	ErrTypeInvalidConfig
	ErrTypeFileAccessDenied
	ErrTypeIndexBusy
)

type CustomError struct {
	Type    ErrorType
	Message string
	Err     error
}

func (e *CustomError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *CustomError) Unwrap() error {
	return e.Err
}

func NewCustomError(errType ErrorType, message string, err error) error {
	return &CustomError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}
