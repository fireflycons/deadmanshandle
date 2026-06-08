package adapters

import (
	"context"
	"crypto/subtle"
)

// SimpleAPIKeyValidator implements basic API key validation
type SimpleAPIKeyValidator struct{}

// NewSimpleAPIKeyValidator creates a new API key validator
func NewSimpleAPIKeyValidator() *SimpleAPIKeyValidator {
	return &SimpleAPIKeyValidator{}
}

// ValidateAPIKey performs constant-time comparison of API keys
func (v *SimpleAPIKeyValidator) ValidateAPIKey(ctx context.Context, providedKey string, expectedKey string) bool {
	return subtle.ConstantTimeCompare([]byte(providedKey), []byte(expectedKey)) == 1
}
