package energy

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestParseJWTExpiry(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		want    time.Time
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid JWT with exp claim",
			token:   createTestJWT(t, map[string]interface{}{"exp": float64(1700000000)}),
			want:    time.Unix(1700000000, 0),
			wantErr: false,
		},
		{
			name:    "valid JWT with exp as int64",
			token:   createTestJWTWithInt64Exp(t, 1700000000),
			want:    time.Unix(1700000000, 0),
			wantErr: false,
		},
		{
			name:    "JWT without exp claim",
			token:   createTestJWT(t, map[string]interface{}{"sub": "user123"}),
			wantErr: true,
			errMsg:  "no exp claim in JWT",
		},
		{
			name:    "invalid JWT format - missing parts",
			token:   "header.payload",
			wantErr: true,
			errMsg:  "invalid JWT format",
		},
		{
			name:    "invalid JWT format - too many parts",
			token:   "header.payload.signature.extra",
			wantErr: true,
			errMsg:  "invalid JWT format",
		},
		{
			name:    "invalid base64 in payload",
			token:   "header.!!!invalid-base64!!!.signature",
			wantErr: true,
			errMsg:  "failed to decode JWT payload",
		},
		{
			name:    "invalid JSON in payload",
			token:   "header." + base64.RawURLEncoding.EncodeToString([]byte("not json")) + ".signature",
			wantErr: true,
			errMsg:  "failed to parse JWT claims",
		},
		{
			name:    "exp claim with wrong type",
			token:   createTestJWT(t, map[string]interface{}{"exp": "not-a-number"}),
			wantErr: true,
			errMsg:  "unexpected type for exp claim",
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
			errMsg:  "invalid JWT format",
		},
		{
			name:    "realistic JWT with multiple claims",
			token: createTestJWT(t, map[string]interface{}{
				"exp":   float64(1700000000),
				"iat":   float64(1699999000),
				"sub":   "user123",
				"email": "user@example.com",
				"role":  "admin",
			}),
			want:    time.Unix(1700000000, 0),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseJWTExpiry(tt.token)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("parseJWTExpiry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg && !contains(err.Error(), tt.errMsg) {
					t.Errorf("parseJWTExpiry() error = %v, want error containing %v", err.Error(), tt.errMsg)
				}
				return
			}
			
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("parseJWTExpiry() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to create a test JWT with custom claims
func createTestJWT(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	
	header := map[string]interface{}{
		"alg": "HS256",
		"typ": "JWT",
	}
	
	headerBytes, _ := json.Marshal(header)
	claimsBytes, _ := json.Marshal(claims)
	
	encodedHeader := base64.RawURLEncoding.EncodeToString(headerBytes)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsBytes)
	
	return encodedHeader + "." + encodedClaims + ".signature"
}

// Special helper to create JWT with int64 exp (to test type handling)
func createTestJWTWithInt64Exp(t *testing.T, exp int64) string {
	t.Helper()
	
	header := map[string]interface{}{
		"alg": "HS256",
		"typ": "JWT",
	}
	
	// Manually construct JSON to ensure exp is treated as int64
	headerBytes, _ := json.Marshal(header)
	claimsJSON := `{"exp":` + string(mustMarshalJSON(t, exp)) + `}`
	
	encodedHeader := base64.RawURLEncoding.EncodeToString(headerBytes)
	encodedClaims := base64.RawURLEncoding.EncodeToString([]byte(claimsJSON))
	
	return encodedHeader + "." + encodedClaims + ".signature"
}

func mustMarshalJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	return b
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || len(substr) > 0 && len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestParseJWTExpiryIntegration(t *testing.T) {
	// Test with a realistic JWT that might come from an actual auth service
	futureTime := time.Now().Add(1 * time.Hour).Unix()
	
	token := createTestJWT(t, map[string]interface{}{
		"exp": float64(futureTime),
		"iat": float64(time.Now().Unix()),
		"sub": "test-user",
	})
	
	expiry, err := parseJWTExpiry(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Check that the expiry is in the future
	if !expiry.After(time.Now()) {
		t.Errorf("expected expiry to be in the future, got %v", expiry)
	}
	
	// Check that the expiry matches what we set
	if expiry.Unix() != futureTime {
		t.Errorf("expected expiry Unix time %d, got %d", futureTime, expiry.Unix())
	}
}