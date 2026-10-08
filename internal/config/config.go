package config

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Config represents the application configuration stored in Parameter Store.
// The Lambdas only read it; what changes at run time is in State.
type Config struct {
	Owner      string   `json:"owner"`
	Recipients []string `json:"recipients"`
	ResetDays  int      `json:"resetDays"`
	WarnDays   int      `json:"warnDays"`
	APIKey     string   `json:"apiKey"`
}

// State is the handle's mutable state, kept in DynamoDB so that concurrent
// Lambdas can update it atomically
type State struct {
	Timeout time.Time
	// Incremented by each check-in. Delivery state is only recorded if it is
	// unchanged since the run read it, so a check-in during a run wins.
	CheckIns int64

	// Delivery state, written once the timeout has passed so that later
	// scheduled runs do not resend. Both are cleared by a check-in.
	SentTo        []string // Recipients already sent the document
	OwnerNotified bool     // Owner told that the document was sent

	// ETag of the document last seen, so a change of content can be
	// reported. Empty until a document has been seen.
	DocumentETag string
	// When a change of content was last found; zero if never. Delivery is
	// held for a while after a change.
	DocumentChangedAt time.Time
	// Whether the owner has yet to be told of that change
	DocumentChangePending bool
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
	if strings.TrimSpace(c.APIKey) == "" {
		errs = append(errs, errors.New("apiKey is empty"))
	}
	if len(errs) > 0 {
		return errors.Join(append([]error{errors.New("invalid config")}, errs...)...)
	}
	return nil
}
