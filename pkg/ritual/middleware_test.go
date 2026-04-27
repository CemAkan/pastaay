package ritual

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMatchHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-Test-User", "true")
	req.Header.Set("X-Device", "ios")

	tests := []struct {
		name     string
		required map[string]string
		expected bool
	}{
		{
			name:     "No requirements should match",
			required: nil,
			expected: true,
		},
		{
			name:     "Single matching requirement",
			required: map[string]string{"X-Test-User": "true"},
			expected: true,
		},
		{
			name:     "Mismatching requirement",
			required: map[string]string{"X-Test-User": "false"},
			expected: false,
		},
		{
			name:     "Missing required header",
			required: map[string]string{"X-Version": "v2"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchHeaders(req, tt.required); got != tt.expected {
				t.Errorf("matchHeaders() = %v, want %v", got, tt.expected)
			}
		})
	}
}
