package energy

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// Retry tuning. These are vars rather than consts so tests can shorten them.
var (
	// apiCallTimeout bounds a single upstream call. The geo client builds its
	// own http.Client with no timeout, so without this a stalled connection
	// blocks the polling loop indefinitely.
	apiCallTimeout = 20 * time.Second

	apiMaxAttempts   = 3
	apiRetryBaseWait = 500 * time.Millisecond
)

// callAPI runs fn against the upstream API, retrying transient failures with
// jittered exponential backoff. A panic inside fn (the geo client dereferences
// a nil *http.Response when the transport errors, which crashes the process
// with a SIGSEGV) is recovered and returned as an error.
func callAPI[T any](name string, fn func() (T, error)) (T, error) {
	var zero T
	var lastErr error

	wait := apiRetryBaseWait
	for attempt := 1; attempt <= apiMaxAttempts; attempt++ {
		result, err := callAPIOnce(name, fn)
		if err == nil {
			return result, nil
		}
		lastErr = err

		// Bad credentials won't fix themselves; don't hammer the API.
		if isAuthError(err) {
			break
		}

		if attempt < apiMaxAttempts {
			time.Sleep(jitter(wait))
			wait *= 2
		}
	}

	return zero, fmt.Errorf("%s: %w", name, lastErr)
}

// callAPIOnce makes a single attempt, bounded by apiCallTimeout and shielded
// against panics in the upstream client.
func callAPIOnce[T any](name string, fn func() (T, error)) (T, error) {
	type outcome struct {
		value T
		err   error
	}

	// Buffered so the goroutine can exit even if we've stopped waiting for it.
	done := make(chan outcome, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- outcome{err: fmt.Errorf("upstream client panicked: %v", r)}
			}
		}()

		value, err := fn()
		done <- outcome{value: value, err: err}
	}()

	timer := time.NewTimer(apiCallTimeout)
	defer timer.Stop()

	select {
	case o := <-done:
		return o.value, o.err
	case <-timer.C:
		var zero T
		return zero, fmt.Errorf("timed out after %v", apiCallTimeout)
	}
}

// isAuthError reports whether err came back as an authentication failure. The
// geo client returns these as opaque strings, so we match on what it formats.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Response Code: 401") ||
		strings.Contains(msg, "Response Code: 403") ||
		strings.Contains(msg, "Please check your login details")
}

// jitter spreads retries out so a struggling upstream doesn't get a thundering
// herd of perfectly-spaced retries.
func jitter(d time.Duration) time.Duration {
	return d + time.Duration(rand.Int63n(int64(d/2)+1))
}
