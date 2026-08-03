package certificate

import (
	"strings"
	"testing"
)

func TestSanitizeDomain(t *testing.T) {
	tests := []struct {
		input string
		// unsafe is true if the output must NOT contain path separators or ..
		unsafe bool
	}{
		{"example.com", false},
		{"*.example.com", false},
		{"../../../etc/passwd", true},  // critical: path traversal must be neutralized
		{"..\\..\\windows", true},      // windows-style traversal must be neutralized
		{"normal-domain.com", false},
		{"a/b/c", true},                // path separators must be removed
		{"..", true},
		{".", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeDomain(tt.input)
			// Security assertion: no path separators or traversal sequences
			if strings.Contains(got, "/") || strings.Contains(got, "\\") || strings.Contains(got, "..") {
				t.Errorf("sanitizeDomain(%q) = %q contains dangerous characters (/, \\, or ..)", tt.input, got)
			}
			// Ensure output is not empty
			if got == "" {
				t.Errorf("sanitizeDomain(%q) returned empty string", tt.input)
			}
		})
	}
}
