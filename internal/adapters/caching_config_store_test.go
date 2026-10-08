package adapters

import (
	"context"
	"errors"
	"testing"
	"time"
)

// countingConfigStore returns value (or err) and counts the calls
type countingConfigStore struct {
	value string
	err   error
	calls int
}

func (s *countingConfigStore) GetConfig(ctx context.Context, parameterName string) ([]byte, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return []byte(parameterName + "=" + s.value), nil
}

func TestCachingConfigStore(t *testing.T) {
	const ttl = 5 * time.Minute
	failure := errors.New("ssm unavailable")

	// Each step advances the clock, optionally changes the inner store, then
	// calls GetConfig
	type step struct {
		advance   time.Duration
		name      string
		value     string // inner store's value from this step on
		err       error  // inner store's error from this step on
		want      string
		wantErr   bool
		wantCalls int // inner calls so far
	}

	tests := []struct {
		name  string
		steps []step
	}{
		{"cached within the ttl", []step{
			{name: "p", value: "v1", want: "p=v1", wantCalls: 1},
			{advance: ttl - time.Second, name: "p", value: "v2", want: "p=v1", wantCalls: 1},
		}},
		{"fetched again after the ttl", []step{
			{name: "p", value: "v1", want: "p=v1", wantCalls: 1},
			{advance: ttl, name: "p", value: "v2", want: "p=v2", wantCalls: 2},
			{advance: time.Second, name: "p", want: "p=v2", wantCalls: 2},
		}},
		{"error not cached", []step{
			{name: "p", err: failure, wantErr: true, wantCalls: 1},
			{name: "p", value: "v1", want: "p=v1", wantCalls: 2},
		}},
		{"error after the ttl keeps nothing stale", []step{
			{name: "p", value: "v1", want: "p=v1", wantCalls: 1},
			{advance: ttl, name: "p", err: failure, wantErr: true, wantCalls: 2},
			{name: "p", value: "v2", want: "p=v2", wantCalls: 3},
		}},
		{"other parameter not served from the cache", []step{
			{name: "p", value: "v1", want: "p=v1", wantCalls: 1},
			{name: "q", value: "v1", want: "q=v1", wantCalls: 2},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := &countingConfigStore{}
			now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			cache := NewCachingConfigStore(inner, ttl)
			cache.now = func() time.Time { return now }

			for i, s := range tt.steps {
				now = now.Add(s.advance)
				if s.value != "" {
					inner.value = s.value
				}
				inner.err = s.err

				got, err := cache.GetConfig(context.Background(), s.name)
				if s.wantErr {
					if err == nil {
						t.Errorf("step %d: expected an error, got %q", i, got)
					}
				} else if err != nil {
					t.Errorf("step %d: unexpected error: %v", i, err)
				} else if string(got) != s.want {
					t.Errorf("step %d: expected %q, got %q", i, s.want, got)
				}
				if inner.calls != s.wantCalls {
					t.Errorf("step %d: expected %d inner calls, got %d", i, s.wantCalls, inner.calls)
				}
			}
		})
	}
}
