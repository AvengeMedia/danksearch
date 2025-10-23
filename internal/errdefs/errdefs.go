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

var (
	ErrIndexNotFound    = &CustomError{Type: ErrTypeIndexNotFound, Message: "index not found"}
	ErrIndexCorrupted   = &CustomError{Type: ErrTypeIndexCorrupted, Message: "index corrupted"}
	ErrIndexingFailed   = &CustomError{Type: ErrTypeIndexingFailed, Message: "indexing failed"}
	ErrSearchFailed     = &CustomError{Type: ErrTypeSearchFailed, Message: "search failed"}
	ErrWatcherFailed    = &CustomError{Type: ErrTypeWatcherFailed, Message: "watcher failed"}
	ErrInvalidConfig    = &CustomError{Type: ErrTypeInvalidConfig, Message: "invalid config"}
	ErrFileAccessDenied = &CustomError{Type: ErrTypeFileAccessDenied, Message: "file access denied"}
)
