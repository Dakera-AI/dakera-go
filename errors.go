package dakera

import "fmt"

// DakeraError is the base error type for all Dakera errors.
type DakeraError struct {
	Message      string
	StatusCode   int
	ResponseBody interface{}
}

func (e *DakeraError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("DakeraError: %s (status: %d)", e.Message, e.StatusCode)
	}
	return fmt.Sprintf("DakeraError: %s", e.Message)
}

// ConnectionError is raised when unable to connect to Dakera server.
type ConnectionError struct {
	DakeraError
}

func NewConnectionError(message string) *ConnectionError {
	return &ConnectionError{
		DakeraError: DakeraError{Message: message},
	}
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("ConnectionError: %s", e.Message)
}

// NotFoundError is raised when a requested resource is not found.
type NotFoundError struct {
	DakeraError
}

func NewNotFoundError(message string, statusCode int, body interface{}) *NotFoundError {
	return &NotFoundError{
		DakeraError: DakeraError{
			Message:      message,
			StatusCode:   statusCode,
			ResponseBody: body,
		},
	}
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("NotFoundError: %s", e.Message)
}

// ValidationError is raised when request validation fails.
type ValidationError struct {
	DakeraError
}

func NewValidationError(message string, statusCode int, body interface{}) *ValidationError {
	return &ValidationError{
		DakeraError: DakeraError{
			Message:      message,
			StatusCode:   statusCode,
			ResponseBody: body,
		},
	}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("ValidationError: %s", e.Message)
}

// RateLimitError is raised when rate limit is exceeded.
type RateLimitError struct {
	DakeraError
	RetryAfter int
}

func NewRateLimitError(message string, statusCode int, body interface{}, retryAfter int) *RateLimitError {
	return &RateLimitError{
		DakeraError: DakeraError{
			Message:      message,
			StatusCode:   statusCode,
			ResponseBody: body,
		},
		RetryAfter: retryAfter,
	}
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("RateLimitError: %s (retry after %d seconds)", e.Message, e.RetryAfter)
	}
	return fmt.Sprintf("RateLimitError: %s", e.Message)
}

// ServerError is raised when the server returns a 5xx error.
type ServerError struct {
	DakeraError
}

func NewServerError(message string, statusCode int, body interface{}) *ServerError {
	return &ServerError{
		DakeraError: DakeraError{
			Message:      message,
			StatusCode:   statusCode,
			ResponseBody: body,
		},
	}
}

func (e *ServerError) Error() string {
	return fmt.Sprintf("ServerError: %s (status: %d)", e.Message, e.StatusCode)
}

// AuthenticationError is raised when authentication fails.
type AuthenticationError struct {
	DakeraError
}

func NewAuthenticationError(message string, statusCode int, body interface{}) *AuthenticationError {
	return &AuthenticationError{
		DakeraError: DakeraError{
			Message:      message,
			StatusCode:   statusCode,
			ResponseBody: body,
		},
	}
}

func (e *AuthenticationError) Error() string {
	return fmt.Sprintf("AuthenticationError: %s", e.Message)
}

// TimeoutError is raised when a request times out.
type TimeoutError struct {
	DakeraError
}

func NewTimeoutError(message string) *TimeoutError {
	return &TimeoutError{
		DakeraError: DakeraError{Message: message},
	}
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("TimeoutError: %s", e.Message)
}

// IsNotFoundError checks if an error is a NotFoundError.
func IsNotFoundError(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}

// IsValidationError checks if an error is a ValidationError.
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// IsRateLimitError checks if an error is a RateLimitError.
func IsRateLimitError(err error) bool {
	_, ok := err.(*RateLimitError)
	return ok
}

// IsServerError checks if an error is a ServerError.
func IsServerError(err error) bool {
	_, ok := err.(*ServerError)
	return ok
}

// IsAuthenticationError checks if an error is an AuthenticationError.
func IsAuthenticationError(err error) bool {
	_, ok := err.(*AuthenticationError)
	return ok
}

// IsTimeoutError checks if an error is a TimeoutError.
func IsTimeoutError(err error) bool {
	_, ok := err.(*TimeoutError)
	return ok
}

// IsConnectionError checks if an error is a ConnectionError.
func IsConnectionError(err error) bool {
	_, ok := err.(*ConnectionError)
	return ok
}
