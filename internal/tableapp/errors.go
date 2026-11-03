package tableapp

import "fmt"

type Error struct {
	Code      string
	Message   string
	Retryable bool
	Details   map[string]any
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func actorError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func actorErrorWithDetails(code, message string, details map[string]any) *Error {
	return &Error{Code: code, Message: message, Details: details}
}
