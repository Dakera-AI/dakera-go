package dakera

import (
	"errors"
	"testing"
)

func TestDakeraError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *DakeraError
		want string
	}{
		{
			name: "code and status",
			err:  &DakeraError{Message: "not found", StatusCode: 404, Code: ErrorCodeNamespaceNotFound},
			want: "DakeraError: not found (status: 404, code: NAMESPACE_NOT_FOUND)",
		},
		{
			name: "code only no status",
			err:  &DakeraError{Message: "dimension mismatch", Code: ErrorCodeDimensionMismatch},
			want: "DakeraError: dimension mismatch (code: DIMENSION_MISMATCH)",
		},
		{
			name: "status only no code",
			err:  &DakeraError{Message: "server error", StatusCode: 500},
			want: "DakeraError: server error (status: 500)",
		},
		{
			name: "unknown code treated as no code",
			err:  &DakeraError{Message: "unknown", StatusCode: 0, Code: ErrorCodeUnknown},
			want: "DakeraError: unknown",
		},
		{
			name: "unknown code with status",
			err:  &DakeraError{Message: "unknown", StatusCode: 503, Code: ErrorCodeUnknown},
			want: "DakeraError: unknown (status: 503)",
		},
		{
			name: "message only",
			err:  &DakeraError{Message: "something went wrong"},
			want: "DakeraError: something went wrong",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestConnectionError(t *testing.T) {
	err := NewConnectionError("cannot reach server")
	if err.Error() != "ConnectionError: cannot reach server" {
		t.Errorf("unexpected: %s", err.Error())
	}
	if err.Message != "cannot reach server" {
		t.Errorf("Message field wrong: %s", err.Message)
	}
}

func TestNotFoundError(t *testing.T) {
	err := NewNotFoundError("namespace missing", 404, nil, ErrorCodeNamespaceNotFound)
	if err.Error() != "NotFoundError: namespace missing" {
		t.Errorf("unexpected: %s", err.Error())
	}
	if err.StatusCode != 404 {
		t.Errorf("StatusCode wrong: %d", err.StatusCode)
	}
	if err.Code != ErrorCodeNamespaceNotFound {
		t.Errorf("Code wrong: %s", err.Code)
	}
}

func TestValidationError(t *testing.T) {
	err := NewValidationError("empty vector", 400, nil, ErrorCodeEmptyVector)
	if err.Error() != "ValidationError: empty vector" {
		t.Errorf("unexpected: %s", err.Error())
	}
	if err.Code != ErrorCodeEmptyVector {
		t.Errorf("Code wrong: %s", err.Code)
	}
}

func TestRateLimitError(t *testing.T) {
	t.Run("with retry-after", func(t *testing.T) {
		err := NewRateLimitError("rate limit exceeded", 429, nil, ErrorCodeQuotaExceeded, 30)
		if err.Error() != "RateLimitError: rate limit exceeded (retry after 30 seconds)" {
			t.Errorf("unexpected: %s", err.Error())
		}
		if err.RetryAfter != 30 {
			t.Errorf("RetryAfter wrong: %d", err.RetryAfter)
		}
	})
	t.Run("without retry-after", func(t *testing.T) {
		err := NewRateLimitError("rate limit exceeded", 429, nil, ErrorCodeQuotaExceeded, 0)
		if err.Error() != "RateLimitError: rate limit exceeded" {
			t.Errorf("unexpected: %s", err.Error())
		}
	})
}

func TestServerError(t *testing.T) {
	err := NewServerError("internal error", 500, nil, ErrorCodeInternalError)
	if err.Error() != "ServerError: internal error (status: 500)" {
		t.Errorf("unexpected: %s", err.Error())
	}
}

func TestAuthenticationError(t *testing.T) {
	err := NewAuthenticationError("invalid api key", 401, nil, ErrorCodeInvalidApiKey)
	if err.Error() != "AuthenticationError: invalid api key" {
		t.Errorf("unexpected: %s", err.Error())
	}
}

func TestAuthorizationError(t *testing.T) {
	err := NewAuthorizationError("namespace access denied", 403, ErrorCodeNamespaceAccessDenied, nil)
	if err.Error() != "AuthorizationError: namespace access denied" {
		t.Errorf("unexpected: %s", err.Error())
	}
}

func TestTimeoutError(t *testing.T) {
	err := NewTimeoutError("request timed out")
	if err.Error() != "TimeoutError: request timed out" {
		t.Errorf("unexpected: %s", err.Error())
	}
}

func TestIsErrorTypeGuards(t *testing.T) {
	notFound := NewNotFoundError("x", 404, nil, ErrorCodeVectorNotFound)
	validation := NewValidationError("y", 400, nil, ErrorCodeInvalidRequest)
	rateLimit := NewRateLimitError("z", 429, nil, ErrorCodeQuotaExceeded, 0)
	server := NewServerError("s", 500, nil, ErrorCodeInternalError)
	auth := NewAuthenticationError("a", 401, nil, ErrorCodeAuthenticationRequired)
	authz := NewAuthorizationError("b", 403, ErrorCodeNamespaceAccessDenied, nil)
	timeout := NewTimeoutError("t")
	conn := NewConnectionError("c")
	other := errors.New("plain error")

	if !IsNotFoundError(notFound) {
		t.Error("IsNotFoundError should be true")
	}
	if IsNotFoundError(validation) {
		t.Error("IsNotFoundError should be false for ValidationError")
	}
	if !IsValidationError(validation) {
		t.Error("IsValidationError should be true")
	}
	if !IsRateLimitError(rateLimit) {
		t.Error("IsRateLimitError should be true")
	}
	if !IsServerError(server) {
		t.Error("IsServerError should be true")
	}
	if !IsAuthenticationError(auth) {
		t.Error("IsAuthenticationError should be true")
	}
	if !IsAuthorizationError(authz) {
		t.Error("IsAuthorizationError should be true")
	}
	if !IsTimeoutError(timeout) {
		t.Error("IsTimeoutError should be true")
	}
	if !IsConnectionError(conn) {
		t.Error("IsConnectionError should be true")
	}
	if IsNotFoundError(other) || IsServerError(other) || IsTimeoutError(other) {
		t.Error("type guards should return false for plain error")
	}
}

func TestErrorCodeConstants(t *testing.T) {
	codes := []struct {
		code ErrorCode
		str  string
	}{
		{ErrorCodeNamespaceNotFound, "NAMESPACE_NOT_FOUND"},
		{ErrorCodeVectorNotFound, "VECTOR_NOT_FOUND"},
		{ErrorCodeDimensionMismatch, "DIMENSION_MISMATCH"},
		{ErrorCodeEmptyVector, "EMPTY_VECTOR"},
		{ErrorCodeInvalidRequest, "INVALID_REQUEST"},
		{ErrorCodeStorageError, "STORAGE_ERROR"},
		{ErrorCodeInternalError, "INTERNAL_ERROR"},
		{ErrorCodeQuotaExceeded, "QUOTA_EXCEEDED"},
		{ErrorCodeServiceUnavailable, "SERVICE_UNAVAILABLE"},
		{ErrorCodeAuthenticationRequired, "AUTHENTICATION_REQUIRED"},
		{ErrorCodeInvalidApiKey, "INVALID_API_KEY"},
		{ErrorCodeApiKeyExpired, "API_KEY_EXPIRED"},
		{ErrorCodeInsufficientScope, "INSUFFICIENT_SCOPE"},
		{ErrorCodeNamespaceAccessDenied, "NAMESPACE_ACCESS_DENIED"},
		{ErrorCodeUnknown, "UNKNOWN"},
	}
	for _, tc := range codes {
		if string(tc.code) != tc.str {
			t.Errorf("ErrorCode %s: got %q, want %q", tc.str, tc.code, tc.str)
		}
	}
}
