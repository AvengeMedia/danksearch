package errdefs

import (
	"errors"
	"testing"
)

func TestCustomError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *CustomError
		expected string
	}{
		{
			name:     "error without wrapped error",
			err:      &CustomError{Type: ErrTypeIndexNotFound, Message: "test message"},
			expected: "test message",
		},
		{
			name:     "error with wrapped error",
			err:      &CustomError{Type: ErrTypeIndexNotFound, Message: "test message", Err: errors.New("wrapped")},
			expected: "test message: wrapped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCustomError_Unwrap(t *testing.T) {
	wrappedErr := errors.New("wrapped error")
	err := &CustomError{
		Type:    ErrTypeIndexNotFound,
		Message: "test",
		Err:     wrappedErr,
	}

	if unwrapped := err.Unwrap(); unwrapped != wrappedErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, wrappedErr)
	}
}

func TestNewCustomError(t *testing.T) {
	wrappedErr := errors.New("wrapped")
	err := NewCustomError(ErrTypeIndexNotFound, "test message", wrappedErr)

	customErr, ok := err.(*CustomError)
	if !ok {
		t.Fatal("expected *CustomError")
	}

	if customErr.Type != ErrTypeIndexNotFound {
		t.Errorf("Type = %v, want %v", customErr.Type, ErrTypeIndexNotFound)
	}

	if customErr.Message != "test message" {
		t.Errorf("Message = %v, want %v", customErr.Message, "test message")
	}

	if customErr.Err != wrappedErr {
		t.Errorf("Err = %v, want %v", customErr.Err, wrappedErr)
	}
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		errType ErrorType
		message string
	}{
		{"ErrIndexNotFound", ErrIndexNotFound, ErrTypeIndexNotFound, "index not found"},
		{"ErrIndexCorrupted", ErrIndexCorrupted, ErrTypeIndexCorrupted, "index corrupted"},
		{"ErrIndexingFailed", ErrIndexingFailed, ErrTypeIndexingFailed, "indexing failed"},
		{"ErrSearchFailed", ErrSearchFailed, ErrTypeSearchFailed, "search failed"},
		{"ErrWatcherFailed", ErrWatcherFailed, ErrTypeWatcherFailed, "watcher failed"},
		{"ErrInvalidConfig", ErrInvalidConfig, ErrTypeInvalidConfig, "invalid config"},
		{"ErrFileAccessDenied", ErrFileAccessDenied, ErrTypeFileAccessDenied, "file access denied"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			customErr, ok := tt.err.(*CustomError)
			if !ok {
				t.Fatal("expected *CustomError")
			}

			if customErr.Type != tt.errType {
				t.Errorf("Type = %v, want %v", customErr.Type, tt.errType)
			}

			if customErr.Message != tt.message {
				t.Errorf("Message = %v, want %v", customErr.Message, tt.message)
			}
		})
	}
}
