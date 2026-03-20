package dakera

import "fmt"

// ErrorCode represents a typed server error code from the Dakera API.
type ErrorCode string

const (
	ErrorCodeNamespaceNotFound      ErrorCode = "NAMESPACE_NOT_FOUND"
	ErrorCodeVectorNotFound         ErrorCode = "VECTOR_NOT_FOUND"
	ErrorCodeDimensionMismatch      ErrorCode = "DIMENSION_MISMATCH"
	ErrorCodeEmptyVector            ErrorCode = "EMPTY_VECTOR"
	ErrorCodeInvalidRequest         ErrorCode = "INVALID_REQUEST"
	ErrorCodeStorageError           ErrorCode = "STORAGE_ERROR"
	ErrorCodeInternalError          ErrorCode = "INTERNAL_ERROR"
	ErrorCodeQuotaExceeded          ErrorCode = "QUOTA_EXCEEDED"
	ErrorCodeServiceUnavailable     ErrorCode = "SERVICE_UNAVAILABLE"
	ErrorCodeAuthenticationRequired ErrorCode = "AUTHENTICATION_REQUIRED"
	ErrorCodeInvalidApiKey          ErrorCode = "INVALID_API_KEY"
	ErrorCodeApiKeyExpired          ErrorCode = "API_KEY_EXPIRED"
	ErrorCodeInsufficientScope      ErrorCode = "INSUFFICIENT_SCOPE"
	ErrorCodeNamespaceAccessDenied  ErrorCode = "NAMESPACE_ACCESS_DENIED"
	ErrorCodeUnknown                ErrorCode = "UNKNOWN"
)

// DakeraError is the base error type for all Dakera errors.
type DakeraError struct {
	Message      string
	StatusCode   int
	Code         ErrorCode
	ResponseBody interface{}
}

func (e *DakeraError) Error() string {
	if e.Code != "" && e.Code != ErrorCodeUnknown {
		if e.StatusCode > 0 {
			return fmt.Sprintf("DakeraError: %s (status: %d, code: %s)", e.Message, e.StatusCode, e.Code)
		}
		return fmt.Sprintf("DakeraError: %s (code: %s)", e.Message, e.Code)
	}
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

func NewNotFoundError(message string, statusCode int, body interface{}, code ErrorCode) *NotFoundError {
	return &NotFoundError{
		DakeraError: DakeraError{
			Message:      message,
			StatusCode:   statusCode,
			Code:         code,
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

func NewValidationError(message string, statusCode int, body interface{}, code ErrorCode) *ValidationError {
	return &ValidationError{
		DakeraError: DakeraError{
			Message:      message,
			StatusCode:   statusCode,
			Code:         code,
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

func NewRateLimitError(message string, statusCode int, body interface{}, code ErrorCode, retryAfter int) *RateLimitError {
	return &RateLimitError{
		DakeraError: DakeraError{
			Message:      message,
			StatusCode:   statusCode,
			Code:         code,
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

func NewServerError(message string, statusCode int, body interface{}, code ErrorCode) *ServerError {
	return &ServerError{
		DakeraError: DakeraError{
			Message:      message,
			StatusCode:   statusCode,
			Code:         code,
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

func NewAuthenticationError(message string, statusCode int, body interface{}, code ErrorCode) *AuthenticationError {
	return &AuthenticationError{
		DakeraError: DakeraError{
			Message:      message,
			StatusCode:   statusCode,
			Code:         code,
			ResponseBody: body,
		},
	}
}

func (e *AuthenticationError) Error() string {
	return fmt.Sprintf("AuthenticationError: %s", e.Message)
}

// AuthorizationError is raised when the server returns a 403 Forbidden response.
type AuthorizationError struct {
	DakeraError
}

func NewAuthorizationError(message string, statusCode int, code ErrorCode, body interface{}) *AuthorizationError {
	return &AuthorizationError{
		DakeraError: DakeraError{
			Message:      message,
			StatusCode:   statusCode,
			Code:         code,
			ResponseBody: body,
		},
	}
}

func (e *AuthorizationError) Error() string {
	return fmt.Sprintf("AuthorizationError: %s", e.Message)
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

// IsAuthorizationError checks if an error is an AuthorizationError.
func IsAuthorizationError(err error) bool {
	_, ok := err.(*AuthorizationError)
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
