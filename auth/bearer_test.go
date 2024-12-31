package auth

import (
	"net/http"
	"testing"
)

func TestParseBearerToken(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		expectedToken string
		expectedErr   error
	}{
		{
			name:          "Missing Authorization Header",
			authorization: "",
			expectedToken: "",
			expectedErr:   errMissingAuthorizationHeader,
		},
		{
			name:          "Invalid Authorization Header - No Bearer",
			authorization: "Basic some_token",
			expectedToken: "",
			expectedErr:   errInvalidAuthorizationHeader,
		},
		{
			name:          "Invalid Authorization Header - Malformed Bearer Token",
			authorization: "BearerTokenWithoutSpace",
			expectedToken: "",
			expectedErr:   errInvalidAuthorizationHeader,
		},
		{
			name:          "Valid Bearer Token",
			authorization: "Bearer some_valid_token",
			expectedToken: "some_valid_token",
			expectedErr:   nil,
		},
		{
			name:          "Valid Bearer Token with extra spaces",
			authorization: "Bearer   some_valid_token   ",
			expectedToken: "some_valid_token",
			expectedErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header: http.Header{
					authorizationHeader: []string{tt.authorization},
				},
			}

			token, err := BearerTokenFromRequest(req)
			if token != tt.expectedToken {
				t.Errorf("expected token %q, got %q", tt.expectedToken, token)
			}

			if err != tt.expectedErr {
				t.Errorf("expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}
