package errdefs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ErrdefsSuite struct {
	suite.Suite
}

func TestErrdefsSuite(t *testing.T) {
	suite.Run(t, new(ErrdefsSuite))
}

func (s *ErrdefsSuite) TestCustomError_Error() {
	tests := []struct {
		name     string
		err      *CustomError
		expected string
	}{
		{
			"error without wrapped error",
			&CustomError{Type: ErrTypeIndexNotFound, Message: "test message"},
			"test message",
		},
		{
			"error with wrapped error",
			&CustomError{Type: ErrTypeIndexNotFound, Message: "test message", Err: errors.New("wrapped")},
			"test message: wrapped",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.expected, tt.err.Error())
		})
	}
}

func (s *ErrdefsSuite) TestCustomError_Unwrap() {
	wrappedErr := errors.New("wrapped error")
	err := &CustomError{
		Type:    ErrTypeIndexNotFound,
		Message: "test",
		Err:     wrappedErr,
	}

	s.Equal(wrappedErr, err.Unwrap())
}

func (s *ErrdefsSuite) TestNewCustomError() {
	wrappedErr := errors.New("wrapped")
	err := NewCustomError(ErrTypeIndexNotFound, "test message", wrappedErr)

	customErr, ok := err.(*CustomError)
	s.Require().True(ok)
	s.Equal(ErrTypeIndexNotFound, customErr.Type)
	s.Equal("test message", customErr.Message)
	s.Equal(wrappedErr, customErr.Err)
}
