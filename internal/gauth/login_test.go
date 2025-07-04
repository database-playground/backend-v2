package gauth

import (
	"net/http"
	"net/url"
	"testing"
)

func TestBuildCallbackURL(t *testing.T) {
	tests := []struct {
		name         string
		requestURL   string
		callbackPath string
		expectedURL  string
	}{
		{
			name:         "simple callback path",
			requestURL:   "https://example.com/auth/google/login",
			callbackPath: "callback",
			expectedURL:  "https://example.com/auth/google/callback",
		},
		{
			name:         "callback path with double dots",
			requestURL:   "https://example.com/auth/google/login",
			callbackPath: "../callback",
			expectedURL:  "https://example.com/auth/callback",
		},
		{
			name:         "callback path with leading slash",
			requestURL:   "https://example.com/auth/google/login",
			callbackPath: "/callback",
			expectedURL:  "https://example.com/auth/google/callback",
		},
		{
			name:         "http scheme",
			requestURL:   "http://localhost:8080/auth/google/login",
			callbackPath: "./callback",
			expectedURL:  "http://localhost:8080/auth/google/callback",
		},
		{
			name:         "custom port",
			requestURL:   "https://example.com:8443/auth/google/login",
			callbackPath: "./callback",
			expectedURL:  "https://example.com:8443/auth/google/callback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.requestURL)
			if err != nil {
				t.Fatalf("failed to parse URL %q: %v", tt.requestURL, err)
			}

			req := &http.Request{
				URL: parsedURL,
			}

			got := BuildCallbackURL(req, tt.callbackPath)
			if got != tt.expectedURL {
				t.Errorf("BuildCallbackURL() = %v, want %v", got, tt.expectedURL)
			}
		})
	}
}
