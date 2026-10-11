// Package validation collects field-level validation errors.
package validation

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"
)

// FieldError describes an invalid field. Field uses a JSON path such as
// "sources[2].address".
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Errors is a list of field errors; it implements error.
type Errors []FieldError

func (e Errors) Error() string {
	parts := make([]string, len(e))
	for i, fe := range e {
		parts[i] = fe.Field + ": " + fe.Message
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// Add records an error on field.
func (e *Errors) Add(field, format string, args ...any) {
	*e = append(*e, FieldError{Field: field, Message: fmt.Sprintf(format, args...)})
}

// Err returns nil when no error was recorded, so it can be returned directly.
func (e Errors) Err() error {
	if len(e) == 0 {
		return nil
	}
	return e
}

var hostnameRe = regexp.MustCompile(`^(?i)[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*\.?$`)

// IsHost reports whether s is an IP address or a valid DNS hostname.
func IsHost(s string) bool {
	if _, err := netip.ParseAddr(s); err == nil {
		return true
	}
	return len(s) <= 253 && hostnameRe.MatchString(s)
}
