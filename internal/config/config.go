package config

import (
	"encoding/json"
	"time"
)

// Config represents the application configuration stored in Parameter Store
type Config struct {
	Owner      string    `json:"owner"`
	Recipients []string  `json:"recipients"`
	ResetDays  int       `json:"resetDays"`
	WarnDays   int       `json:"warnDays"`
	Timeout    time.Time `json:"timeout"`
	APIKey     string    `json:"apiKey"`

	// Delivery state, written once the timeout has passed so that later
	// scheduled runs do not resend. Both are cleared by a check-in.
	SentTo        []string `json:"sentTo,omitempty"`        // Recipients already sent the document
	OwnerNotified bool     `json:"ownerNotified,omitempty"` // Owner told that the document was sent
}

// ParseConfig parses JSON configuration
func ParseConfig(data []byte) (*Config, error) {
	var cfg Config
	err := json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ToJSON converts config to JSON bytes
func (c *Config) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// CalculateNewTimeout calculates the new timeout based on reset days
func (c *Config) CalculateNewTimeout(now time.Time, resetDays int) time.Time {
	return now.AddDate(0, 0, resetDays)
}
