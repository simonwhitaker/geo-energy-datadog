package energy

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// withFastRetries shortens the retry tuning for the duration of a test.
func withFastRetries(t *testing.T) {
	t.Helper()

	timeout, attempts, wait := apiCallTimeout, apiMaxAttempts, apiRetryBaseWait
	t.Cleanup(func() {
		apiCallTimeout, apiMaxAttempts, apiRetryBaseWait = timeout, attempts, wait
	})

	apiCallTimeout = 100 * time.Millisecond
	apiMaxAttempts = 3
	apiRetryBaseWait = time.Millisecond
}

func TestCallAPIRecoversFromPanic(t *testing.T) {
	withFastRetries(t)

	calls := 0
	_, err := callAPI("panicky", func() (string, error) {
		calls++
		var resp *struct{ Body string }
		return resp.Body, nil // nil dereference, as in the upstream client
	})

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "panicked") {
		t.Fatalf("expected a panic error, got %v", err)
	}
	if calls != apiMaxAttempts {
		t.Fatalf("expected %d attempts, got %d", apiMaxAttempts, calls)
	}
}

func TestCallAPIRetriesUntilSuccess(t *testing.T) {
	withFastRetries(t)

	calls := 0
	got, err := callAPI("flaky", func() (string, error) {
		calls++
		if calls < 2 {
			return "", errors.New("Response Code: 502")
		}
		return "ok", nil
	})

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if got != "ok" {
		t.Fatalf("expected %q, got %q", "ok", got)
	}
	if calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", calls)
	}
}

func TestCallAPITimesOut(t *testing.T) {
	withFastRetries(t)

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	_, err := callAPI("hung", func() (string, error) {
		<-release // never returns while the test is running
		return "", nil
	})

	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected a timeout error, got %v", err)
	}
}

func TestCallAPIDoesNotRetryAuthFailures(t *testing.T) {
	withFastRetries(t)

	calls := 0
	_, err := callAPI("login", func() (string, error) {
		calls++
		return "", errors.New("Response Code: 401")
	})

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if calls != 1 {
		t.Fatalf("expected 1 attempt for an auth failure, got %d", calls)
	}
}
