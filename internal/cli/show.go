package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
)

// Report is what `dmh config show` prints: the stored settings, what
// Terraform fixed, and the state
type Report struct {
	Settings   Settings    `json:"settings"`
	Deployment Deployment  `json:"deployment"`
	State      *StateItems `json:"state,omitempty"` // nil if it could not be read
	// Whether the check-in API is open; empty if it could not be read
	API string `json:"api,omitempty"`
	// Why the settings or state could not be read or are invalid
	Problems []string `json:"problems,omitempty"`
}

// Settings are the stored config's fields, which `dmh config set` changes
type Settings struct {
	Parameter   string   `json:"parameter"`
	Owner       string   `json:"owner"`
	Recipients  []string `json:"recipients"`
	ResetDays   int      `json:"resetDays"`
	WarnDays    int      `json:"warnDays"`
	APIKey      string   `json:"apiKey"` // masked unless secrets are shown
	DocumentKey string   `json:"documentKey"`
}

// Deployment holds what Terraform fixed, from the parameters it keeps up to
// date. Each is empty if not found.
type Deployment struct {
	Endpoint       string `json:"endpoint"`
	DocumentBucket string `json:"documentBucket"`
	StateTable     string `json:"stateTable"`
	SenderEmail    string `json:"senderEmail"`
}

// StateItems is the state item, which the Lambdas own
type StateItems struct {
	Timeout               time.Time  `json:"timeout"`
	CheckIns              int64      `json:"checkIns"`
	SentTo                []string   `json:"sentTo"`
	OwnerNotified         bool       `json:"ownerNotified"`
	DocumentETag          string     `json:"documentETag"`
	DocumentChangedAt     *time.Time `json:"documentChangedAt"` // nil if never changed
	DocumentChangePending bool       `json:"documentChangePending"`
	PushToken             string     `json:"pushToken"` // shortened unless secrets are shown
}

// NewReport builds the report from the stored config's JSON and the state.
// The config is shown even if it is invalid, with its problems, since
// `dmh config set` is how it gets fixed. Secrets are masked unless
// showSecrets is set.
func NewReport(parameterName string, configData []byte, deployment Deployment, state *config.State, showSecrets bool) Report {
	report := Report{
		Settings:   Settings{Parameter: parameterName},
		Deployment: deployment,
	}

	// Parsed without validation, so an invalid config can still be shown
	var cfg config.Config
	if err := json.Unmarshal(configData, &cfg); err != nil {
		report.Problems = append(report.Problems, fmt.Sprintf("%s is not valid JSON: %v", parameterName, err))
	} else {
		if err := cfg.Validate(); err != nil {
			report.Problems = append(report.Problems, strings.ReplaceAll(err.Error(), "\n", "; "))
		}
		report.Settings.Owner = cfg.Owner
		report.Settings.Recipients = cfg.Recipients
		report.Settings.ResetDays = cfg.ResetDays
		report.Settings.WarnDays = cfg.WarnDays
		report.Settings.APIKey = mask(cfg.APIKey, showSecrets)
		report.Settings.DocumentKey = cfg.DocumentKey
	}

	if state != nil {
		items := &StateItems{
			Timeout:               state.Timeout,
			CheckIns:              state.CheckIns,
			SentTo:                state.SentTo,
			OwnerNotified:         state.OwnerNotified,
			DocumentETag:          state.DocumentETag,
			DocumentChangePending: state.DocumentChangePending,
			PushToken:             shorten(state.PushToken, showSecrets),
		}
		if !state.DocumentChangedAt.IsZero() {
			changedAt := state.DocumentChangedAt
			items.DocumentChangedAt = &changedAt
		}
		report.State = items
	}
	return report
}

// mask hides all but the start of a secret, and all of a short one
func mask(secret string, show bool) string {
	switch {
	case show || secret == "":
		return secret
	case len(secret) < 12:
		return "********"
	default:
		return secret[:4] + "…"
	}
}

// shorten cuts a long push token, which is not secret but runs to hundreds of
// characters
func shorten(token string, show bool) string {
	if show || len(token) <= 16 {
		return token
	}
	return token[:16] + "…"
}

// WriteText prints the report for people. now is used to say how long is
// left until the timeout.
func (r Report) WriteText(w io.Writer, now time.Time) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	row := func(name, value string) {
		_, _ = fmt.Fprintf(tw, "  %s\t%s\n", name, value)
	}
	orNone := func(value string) string {
		if value == "" {
			return "(none)"
		}
		return value
	}

	_, _ = fmt.Fprintf(tw, "Settings (Parameter Store %s)\n", r.Settings.Parameter)
	row("owner", orNone(r.Settings.Owner))
	row("recipients", orNone(strings.Join(r.Settings.Recipients, ", ")))
	row("resetDays", strconv.Itoa(r.Settings.ResetDays))
	row("warnDays", strconv.Itoa(r.Settings.WarnDays))
	row("apiKey", orNone(r.Settings.APIKey))
	row("documentKey", orNone(r.Settings.DocumentKey))

	_, _ = fmt.Fprintln(tw, "\nFixed by Terraform")
	row("endpoint", orNone(r.Deployment.Endpoint))
	row("documentBucket", orNone(r.Deployment.DocumentBucket))
	row("stateTable", orNone(r.Deployment.StateTable))
	row("senderEmail", orNone(r.Deployment.SenderEmail))

	if r.API != "" {
		_, _ = fmt.Fprintln(tw, "\nCheck-in API")
		row("status", r.API)
	}

	if s := r.State; s != nil {
		_, _ = fmt.Fprintln(tw, "\nState (DynamoDB)")
		row("timeout", FormatTime(s.Timeout)+", "+untilTimeout(s.Timeout, now))
		row("checkIns", strconv.FormatInt(s.CheckIns, 10))
		row("sentTo", orNone(strings.Join(s.SentTo, ", ")))
		row("ownerNotified", strconv.FormatBool(s.OwnerNotified))
		row("documentETag", orNone(s.DocumentETag))
		changedAt := "(never)"
		if s.DocumentChangedAt != nil {
			changedAt = FormatTime(*s.DocumentChangedAt)
		}
		row("documentChangedAt", changedAt)
		row("documentChangePending", strconv.FormatBool(s.DocumentChangePending))
		row("pushToken", orNone(s.PushToken))
	}

	if len(r.Problems) > 0 {
		_, _ = fmt.Fprintln(tw, "\nProblems")
		for _, problem := range r.Problems {
			_, _ = fmt.Fprintf(tw, "  %s\n", problem)
		}
	}
	return tw.Flush()
}

// untilTimeout says how long is left, in whole days or hours
func untilTimeout(timeout, now time.Time) string {
	left := timeout.Sub(now)
	switch {
	case left <= 0:
		return "passed"
	case left >= 48*time.Hour:
		return "in " + strconv.Itoa(int(left.Hours()/24)) + " days"
	default:
		return "in " + strconv.Itoa(int(left.Hours())) + " hours"
	}
}
