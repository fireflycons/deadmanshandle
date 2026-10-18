package adapters

import "testing"

// The rate is written by Terraform and by dmh, and read back for the
// owner's notice, so both directions must agree
func TestRateExpressions(t *testing.T) {
	for _, minutes := range []int{1, 2, 15, 1440} {
		expression := rateExpression(minutes)
		got, err := parseRate(&expression)
		if err != nil || got != minutes {
			t.Errorf("%d minutes: %q parsed as %d, %v", minutes, expression, got, err)
		}
	}
	if got := rateExpression(1); got != "rate(1 minute)" {
		t.Errorf("Expected the singular for one minute, got %q", got)
	}

	for _, bad := range []string{"rate(1 hour)", "cron(0 2 * * ? *)", "rate(minutes)"} {
		if _, err := parseRate(&bad); err == nil {
			t.Errorf("Expected %q refused", bad)
		}
	}
	if _, err := parseRate(nil); err == nil {
		t.Error("Expected a rule without a schedule refused")
	}
}
