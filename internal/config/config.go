package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
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
	// Object key of the document in the document bucket
	DocumentKey string `json:"documentKey"`
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

	// FCM registration token of the owner's phone, sent with its check-ins.
	// Empty until the app has checked in; owner notices are pushed to it.
	PushToken string
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
	} else if !isEmailAddress(c.Owner) {
		errs = append(errs, fmt.Errorf("owner %q is not an email address", c.Owner))
	}
	if len(c.Recipients) == 0 {
		errs = append(errs, errors.New("recipients is empty"))
	}
	blank := false
	for _, r := range c.Recipients {
		switch {
		case strings.TrimSpace(r) == "":
			blank = true
		case !isEmailAddress(r):
			errs = append(errs, fmt.Errorf("recipient %q is not an email address", r))
		}
	}
	if blank {
		errs = append(errs, errors.New("recipients contains a blank entry"))
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
	if strings.TrimSpace(c.DocumentKey) == "" {
		errs = append(errs, errors.New("documentKey is empty"))
	}
	if len(errs) > 0 {
		return errors.Join(append([]error{errors.New("invalid config")}, errs...)...)
	}
	return nil
}

// emailAddressPattern matches a bare address, such as someone@example.com,
// which is passed to SES as it is: no display name or spaces, a local part of
// dot-separated words and a domain with at least one dot. It must stay the
// same as email_address_pattern in terraform/parameter_store.tf, so that a
// seed file that passes terraform plan passes here too; a test checks this.
var emailAddressPattern = regexp.MustCompile("^[A-Za-z0-9_%+'-]+([.][A-Za-z0-9_%+'-]+)*@[A-Za-z0-9-]+([.][A-Za-z0-9-]+)+$")

func isEmailAddress(s string) bool {
	return emailAddressPattern.MatchString(s)
}
