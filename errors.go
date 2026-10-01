package dakera

import (
	"errors"
	"fmt"
	"strings"
)

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

	// Codes added by server v0.12.0 (every error body is JSON there).

	// ErrorCodeConflict is a 409: the request conflicts with the server's state
	// (for example deleting an attachment a memory still references).
	ErrorCodeConflict ErrorCode = "CONFLICT"
	// ErrorCodePayloadTooLarge is a 413 where the REQUEST is over a configured
	// limit (attachment size, record size). A namespace that is full answers
	// 413 with ErrorCodeQuotaExceeded instead.
	ErrorCodePayloadTooLarge ErrorCode = "PAYLOAD_TOO_LARGE"
	// ErrorCodeNotImplemented is a 501: the configured backend cannot do this.
	ErrorCodeNotImplemented ErrorCode = "NOT_IMPLEMENTED"
	// ErrorCodeFeatureDisabled is a 501: the route exists but its feature flag
	// is off (Details names the environment variable that turns it on).
	ErrorCodeFeatureDisabled ErrorCode = "FEATURE_DISABLED"
	// ErrorCodeRateLimitExceeded is a 429.
	ErrorCodeRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"
	// ErrorCodeQueryTimeout is a 504: a query ran past query_timeout_ms.
	ErrorCodeQueryTimeout ErrorCode = "QUERY_TIMEOUT"
	// ErrorCodeRequestTimeout is a 408: the request did not complete within DAKERA_REQUEST_TIMEOUT.
	ErrorCodeRequestTimeout ErrorCode = "REQUEST_TIMEOUT"
	// ErrorCodeRouteNotFound is a 404 for a path no route matches.
	ErrorCodeRouteNotFound ErrorCode = "ROUTE_NOT_FOUND"
	// ErrorCodeMethodNotAllowed is a 405.
	ErrorCodeMethodNotAllowed ErrorCode = "METHOD_NOT_ALLOWED"
	// ErrorCodeUnsupportedMediaType is a 415.
	ErrorCodeUnsupportedMediaType ErrorCode = "UNSUPPORTED_MEDIA_TYPE"
	// ErrorCodeApiKeyNotFound is a 404 for an unknown API key id.
	ErrorCodeApiKeyNotFound ErrorCode = "API_KEY_NOT_FOUND"
	// ErrorCodeJobNotFound is a 404 for an unknown job id (jobs live in memory:
	// after a server restart the id is unknown; look up the memory the job stored).
	ErrorCodeJobNotFound ErrorCode = "JOB_NOT_FOUND"
	// ErrorCodeCrossOriginRequestRefused is a 403 for a state-changing request from another origin.
	ErrorCodeCrossOriginRequestRefused ErrorCode = "CROSS_ORIGIN_REQUEST_REFUSED"
)

// DakeraError is the base error type for all Dakera errors.
type DakeraError struct {
	Message      string
	StatusCode   int
	Code         ErrorCode
	ResponseBody interface{}
	// Details is the server's "details" field, when it sent one.
	Details string
	// Resource is what a 404 did not find ("namespace", "memory", "job",
	// "attachment", ...). Server v0.12+; empty otherwise.
	Resource string
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

// UnsupportedCapabilityError is returned *before* a request is sent when the
// server's advertised capabilities (GET /v1/capabilities) do not include what
// was asked for (R9 / DAK-10004). Kind names the registry, Requested is the
// wire string that was rejected and Supported is what the server does accept,
// so the message is actionable on its own. Matches errors.As for both
// *UnsupportedCapabilityError and (via embedding) the ValidationError message.
type UnsupportedCapabilityError struct {
	ValidationError
	Kind          CapabilityKind
	Requested     string
	Supported     []string
	ServerVersion string
}

// NewUnsupportedCapabilityError builds the error with a message that names the
// supported values.
func NewUnsupportedCapabilityError(kind CapabilityKind, requested string, supported []string, serverVersion string) *UnsupportedCapabilityError {
	server := "this Dakera server"
	if serverVersion != "" {
		server = "Dakera server v" + serverVersion
	}
	accepted := "(none advertised)"
	if len(supported) > 0 {
		accepted = strings.Join(supported, ", ")
	}
	msg := fmt.Sprintf("%s '%s' is not supported by %s; supported %s values: %s", kind, requested, server, kind, accepted)
	return &UnsupportedCapabilityError{
		ValidationError: ValidationError{
			DakeraError: DakeraError{
				Message: msg,
				Code:    ErrorCodeInvalidRequest,
			},
		},
		Kind:          kind,
		Requested:     requested,
		Supported:     append([]string(nil), supported...),
		ServerVersion: serverVersion,
	}
}

func (e *UnsupportedCapabilityError) Error() string {
	return fmt.Sprintf("UnsupportedCapabilityError: %s", e.Message)
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
	// RetryAfter is the Retry-After header in seconds, or -1 when absent. Every
	// 503 of a v0.12 server carries one; the client already honours it when it
	// retries.
	RetryAfter int
}

func NewServerError(message string, statusCode int, body interface{}, code ErrorCode) *ServerError {
	return &ServerError{
		DakeraError: DakeraError{
			Message:      message,
			StatusCode:   statusCode,
			Code:         code,
			ResponseBody: body,
		},
		RetryAfter: -1,
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

// ConflictError is raised on a 409: the request conflicts with the server's
// current state (for example deleting an attachment a memory still references).
type ConflictError struct {
	DakeraError
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("ConflictError: %s", e.Message)
}

// PayloadTooLargeError is raised on a 413. Two different things answer 413:
// a namespace over its hard quota (Code QUOTA_EXCEEDED, see IsQuota) and a
// request over a configured size limit (Code PAYLOAD_TOO_LARGE: an attachment
// over DAKERA_ATTACHMENT_MAX_BYTES, a record over the record limits). Neither
// is retried.
type PayloadTooLargeError struct {
	DakeraError
}

// IsQuota reports whether the 413 is a namespace quota (as opposed to an
// oversize request).
func (e *PayloadTooLargeError) IsQuota() bool {
	return e.Code == ErrorCodeQuotaExceeded
}

func (e *PayloadTooLargeError) Error() string {
	if e.IsQuota() {
		return fmt.Sprintf("QuotaExceededError: %s", e.Message)
	}
	return fmt.Sprintf("PayloadTooLargeError: %s", e.Message)
}

// FeatureDisabledError is raised on a 501 FEATURE_DISABLED: the route exists
// but the server was started without the feature (attachments, records,
// vision). Details names the environment variable that turns it on. Check
// Capabilities() first to avoid the round trip.
type FeatureDisabledError struct {
	DakeraError
}

func (e *FeatureDisabledError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("FeatureDisabledError: %s (%s)", e.Message, e.Details)
	}
	return fmt.Sprintf("FeatureDisabledError: %s", e.Message)
}

// NotImplementedError is raised on any other 501: the configured backend
// cannot perform the operation. Details says what configuration would.
type NotImplementedError struct {
	DakeraError
}

func (e *NotImplementedError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("NotImplementedError: %s (%s)", e.Message, e.Details)
	}
	return fmt.Sprintf("NotImplementedError: %s", e.Message)
}

// IsConflictError checks if an error is (or wraps) a ConflictError.
func IsConflictError(err error) bool {
	var target *ConflictError
	return errors.As(err, &target)
}

// IsPayloadTooLargeError checks if an error is (or wraps) a 413 of either
// kind (quota or oversize request).
func IsPayloadTooLargeError(err error) bool {
	var target *PayloadTooLargeError
	return errors.As(err, &target)
}

// IsQuotaExceededError checks if an error is a 413 caused by a namespace quota.
func IsQuotaExceededError(err error) bool {
	var target *PayloadTooLargeError
	return errors.As(err, &target) && target.IsQuota()
}

// IsFeatureDisabledError checks if an error is a 501 FEATURE_DISABLED.
func IsFeatureDisabledError(err error) bool {
	var target *FeatureDisabledError
	return errors.As(err, &target)
}

// IsNotImplementedError checks if an error is a 501 that is not FEATURE_DISABLED.
func IsNotImplementedError(err error) bool {
	var target *NotImplementedError
	return errors.As(err, &target)
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
