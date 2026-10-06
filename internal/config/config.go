package config

import (
	"encoding/json"
	"errors"
	"strings"
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

// ParseConfig parses and validates JSON configuration
func ParseConfig(data []byte) (*Config, error) {
	var cfg Config
	err := json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate reports every problem with the config. The same rules are
// checked against the seed file in terraform/parameter_store.tf.
func (c *Config) Validate() error {
	var errs []error
	if strings.TrimSpace(c.Owner) == "" {
		errs = append(errs, errors.New("owner is empty"))
	}
	if len(c.Recipients) == 0 {
		errs = append(errs, errors.New("recipients is empty"))
	}
	for _, r := range c.Recipients {
		if strings.TrimSpace(r) == "" {
			errs = append(errs, errors.New("recipients contains a blank entry"))
			break
		}
	}
	if c.ResetDays <= 0 {
		errs = append(errs, errors.New("resetDays must be greater than 0"))
	}
	if c.WarnDays < 0 || c.WarnDays >= c.ResetDays {
		errs = append(errs, errors.New("warnDays must be at least 0 and less than resetDays"))
	}
	if c.Timeout.IsZero() {
		errs = append(errs, errors.New("timeout is missing"))
	}
	if strings.TrimSpace(c.APIKey) == "" {
		errs = append(errs, errors.New("apiKey is empty"))
	}
	if len(errs) > 0 {
		return errors.Join(append([]error{errors.New("invalid config")}, errs...)...)
	}
	return nil
}

// ToJSON converts config to JSON bytes
func (c *Config) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// CalculateNewTimeout calculates the new timeout based on reset days
func (c *Config) CalculateNewTimeout(now time.Time, resetDays int) time.Time {
	return now.AddDate(0, 0, resetDays)
}
