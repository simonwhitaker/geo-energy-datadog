package energy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// parseJWTExpiry extracts the expiration time from a JWT token
func parseJWTExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid JWT format")
	}
	
	// Decode the payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to decode JWT payload: %w", err)
	}
	
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse JWT claims: %w", err)
	}
	
	// Extract exp claim
	exp, ok := claims["exp"]
	if !ok {
		return time.Time{}, fmt.Errorf("no exp claim in JWT")
	}
	
	// Convert to int64 (Unix timestamp)
	var expTime int64
	switch v := exp.(type) {
	case float64:
		expTime = int64(v)
	case int64:
		expTime = v
	default:
		return time.Time{}, fmt.Errorf("unexpected type for exp claim: %T", exp)
	}
	
	return time.Unix(expTime, 0), nil
}