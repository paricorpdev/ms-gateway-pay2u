package webhook

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBackoffAttempt1 = 15 * time.Second
	DefaultBackoffAttempt2 = 60 * time.Second
	MaxRetryAfterDuration  = 5 * time.Minute
	MinRetryAfterDuration  = 1 * time.Second
)

// IsSuccess returns true for any HTTP 2xx status code.
func IsSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode <= 299
}

// IsRetryable determines whether a failed HTTP response or error is eligible for retry.
func IsRetryable(statusCode int, err error) bool {
	if err != nil {
		return true // Network error, connection refused, DNS error, timeout
	}

	switch statusCode {
	case http.StatusRequestTimeout: // 408
		return true
	case http.StatusTooManyRequests: // 429
		return true
	case http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:       // 504
		return true
	}

	// Non-retryable 4xx (400, 401, 403, 404, 405, 410, 422, etc.)
	if statusCode >= 400 && statusCode < 500 {
		return false
	}

	// All other 5xx errors are retryable
	return statusCode >= 500
}

// ParseRetryAfter parses the HTTP Retry-After header which can be either seconds or an HTTP-date.
func ParseRetryAfter(headerValue string) (time.Duration, bool) {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		return 0, false
	}

	// Try parsing as integer seconds
	if seconds, err := strconv.Atoi(headerValue); err == nil && seconds >= 0 {
		dur := time.Duration(seconds) * time.Second
		if dur < MinRetryAfterDuration {
			dur = MinRetryAfterDuration
		}
		if dur > MaxRetryAfterDuration {
			dur = MaxRetryAfterDuration
		}
		return dur, true
	}

	// Try parsing as HTTP-date (RFC1123, RFC850, ANSIC)
	formats := []string{
		http.TimeFormat,
		time.RFC850,
		time.ANSIC,
	}
	for _, format := range formats {
		if t, err := time.Parse(format, headerValue); err == nil {
			dur := time.Until(t)
			if dur <= 0 {
				return MinRetryAfterDuration, true
			}
			if dur > MaxRetryAfterDuration {
				dur = MaxRetryAfterDuration
			}
			return dur, true
		}
	}

	return 0, false
}

// CalculateBackoff calculates the next retry duration based on attempt number and optional Retry-After header.
func CalculateBackoff(attempt int, retryAfterHeader string) time.Duration {
	if dur, ok := ParseRetryAfter(retryAfterHeader); ok {
		return dur
	}

	switch attempt {
	case 1:
		return DefaultBackoffAttempt1 // 15s
	case 2:
		return DefaultBackoffAttempt2 // 60s
	default:
		return DefaultBackoffAttempt2
	}
}
