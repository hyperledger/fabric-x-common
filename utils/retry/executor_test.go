/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package retry

import (
	"bytes"
	"context"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
	"github.com/jackc/puddle/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yugabyte/pgx/v5/pgconn"
)

func TestNewBackoff(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                        string
		profile                     *Profile
		expectedInitialInterval     time.Duration
		expectedRandomizationFactor float64
		expectedMultiplier          float64
		expectedMaxInterval         time.Duration
		expectedMaxElapsedTime      time.Duration
	}{
		{
			name:                        "default",
			profile:                     nil,
			expectedInitialInterval:     defaultInitialInterval,
			expectedRandomizationFactor: defaultRandomizationFactor,
			expectedMultiplier:          defaultMultiplier,
			expectedMaxInterval:         defaultMaxInterval,
			expectedMaxElapsedTime:      defaultMaxElapsedTime,
		},
		{
			name: "custom",
			profile: &Profile{
				InitialInterval:     10 * time.Millisecond,
				RandomizationFactor: 0.2,
				Multiplier:          2.0,
				MaxInterval:         50 * time.Millisecond,
				MaxElapsedTime:      new(100 * time.Millisecond),
			},
			expectedInitialInterval:     10 * time.Millisecond,
			expectedRandomizationFactor: 0.2,
			expectedMultiplier:          2.0,
			expectedMaxInterval:         50 * time.Millisecond,
			expectedMaxElapsedTime:      100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := tt.profile.WithDefaults()
			assert.InEpsilon(t, tt.expectedInitialInterval, p.InitialInterval, 0)
			assert.InEpsilon(t, tt.expectedRandomizationFactor, p.RandomizationFactor, 0)
			assert.InEpsilon(t, tt.expectedMultiplier, p.Multiplier, 0)
			assert.Equal(t, tt.expectedMaxInterval, p.MaxInterval)
			assert.Equal(t, tt.expectedMaxElapsedTime, *p.MaxElapsedTime)

			b := tt.profile.NewBackoff()
			assert.InEpsilon(t, tt.expectedInitialInterval, b.InitialInterval, 0)
			assert.InEpsilon(t, tt.expectedRandomizationFactor, b.RandomizationFactor, 0)
			assert.InEpsilon(t, tt.expectedMultiplier, b.Multiplier, 0)
			assert.Equal(t, tt.expectedMaxInterval, b.MaxInterval)
		})
	}
}

func TestExecute(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name                   string
		profile                *Profile
		failUntil              int // parameter for makeOp: negative means always fail
		expectedCallCount      int // expected number of calls if the op eventually succeeds;
		expectError            bool
		expectedErrorSubstring string
	}

	tests := []testCase{
		{
			name: "Success",
			profile: &Profile{
				InitialInterval: 1 * time.Millisecond,
				MaxInterval:     100 * time.Millisecond,
				MaxElapsedTime:  new(1 * time.Second),
			},
			failUntil:         3, // op fails until the third call, then succeeds.
			expectedCallCount: 3,
			expectError:       false,
		},
		{
			name: "Failure",
			profile: &Profile{
				InitialInterval: 10 * time.Millisecond,
				MaxInterval:     500 * time.Millisecond,
				MaxElapsedTime:  new(5 * time.Second),
			},
			failUntil:              -1, // op always fails.
			expectError:            true,
			expectedErrorSubstring: "error",
		},
		{
			name:              "Nil Profile",
			profile:           nil,
			failUntil:         0, // op succeeds immediately.
			expectedCallCount: 1,
			expectError:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			op, callCount := makeOp(tc.failUntil)
			err := Execute(t.Context(), tc.profile, op)
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrorSubstring)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedCallCount, *callCount)
			}
		})
	}
}

// TestExecuteLogLevel is used to manually verify the log output is using the correct
// method name when logging.
//
//nolint:paralleltest // We cannot run in parallel because we modify the logger.
func TestExecuteLogLevel(t *testing.T) {
	var b bytes.Buffer
	flogging.SetWriter(&b)
	t.Cleanup(func() {
		flogging.SetWriter(os.Stderr)
	})

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)
	err := Execute(ctx, nil, func() error {
		time.Sleep(10 * time.Millisecond)
		return errors.New("Execute error")
	})
	require.Error(t, err)

	ctx, cancel = context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)
	_, err = ExecuteWithResult(ctx, nil, func() (any, error) {
		time.Sleep(10 * time.Millisecond)
		return nil, errors.New("ExecuteWithResult error")
	})
	require.Error(t, err)

	ctx, cancel = context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)
	res := WaitForCondition(ctx, nil, func() bool {
		time.Sleep(10 * time.Millisecond)
		return false
	})
	require.False(t, res)

	ctx, cancel = context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)
	err = Sustain(ctx, nil, func() error {
		time.Sleep(10 * time.Millisecond)
		return errors.Wrap(ErrNonRetryable, "Sustain error")
	})
	require.Error(t, err)

	// Regain ownership over the buffer.
	flogging.SetWriter(os.Stderr)

	output := b.String()
	t.Log(output)

	// Remove the color codes from the log output.
	output = regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(output, "")
	require.Contains(t, output, "[retry] TestExecuteLogLevel -> Execute error")
	require.Contains(t, output, "[retry] TestExecuteLogLevel -> ExecuteWithResult error")
	require.Contains(t, output, "[retry] TestExecuteLogLevel -> condition not satisfied")
	require.Contains(t, output, "[retry] TestExecuteLogLevel -> Sustain error")
}

// TestExecuteWithResult verifies that ExecuteWithResult returns the typed result of the
// operation on success and propagates both the operation's last result and its error on failure.
func TestExecuteWithResult(t *testing.T) {
	t.Parallel()

	// Success cases: the eventual result value and the call count are asserted.
	for _, tc := range []struct {
		name      string
		profile   *Profile
		failUntil int
		value     int
		wantCalls int
	}{
		{
			name: "returns result on first success", profile: fastProfile(time.Second),
			failUntil: 1, value: 42, wantCalls: 1,
		},
		{name: "returns result after retries", profile: fastProfile(time.Second), failUntil: 3, value: 7, wantCalls: 3},
		{name: "nil profile returns result", profile: nil, failUntil: 1, value: 99, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			op, callCount := makeResultOp(tc.failUntil, tc.value)
			got, err := ExecuteWithResult(t.Context(), tc.profile, op)
			require.NoError(t, err)
			require.Equal(t, tc.value, got)
			require.Equal(t, tc.wantCalls, *callCount)
		})
	}

	// Failure cases: the last result (-1) is propagated alongside the error.
	for _, tc := range []struct {
		name       string
		profile    *Profile
		ctxTimeout time.Duration
		wantErr    string
	}{
		{
			name:    "returns error and last result when budget elapses",
			profile: fastProfile(80 * time.Millisecond),
			wantErr: "boom",
		},
		{
			name:       "returns context error and last result when context is cancelled",
			profile:    fastProfile(5 * time.Second),
			ctxTimeout: 60 * time.Millisecond,
			wantErr:    "context deadline exceeded",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			if tc.ctxTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tc.ctxTimeout)
				t.Cleanup(cancel)
			}
			op, callCount := makeResultOp(-1, 0)
			got, err := ExecuteWithResult(ctx, tc.profile, op)
			require.ErrorContains(t, err, tc.wantErr)
			require.Equal(t, -1, got)
			require.GreaterOrEqual(t, *callCount, 2)
		})
	}
}

// TestWaitForCondition verifies the boolean return value of WaitForCondition for a condition
// that becomes true (success) and one that never does (timeout / context cancellation).
func TestWaitForCondition(t *testing.T) {
	t.Parallel()

	// Success cases: the condition eventually returns true.
	for _, tc := range []struct {
		name      string
		profile   *Profile
		trueAfter int
		wantCalls int
	}{
		{name: "true on first evaluation", profile: fastProfile(time.Second), trueAfter: 1, wantCalls: 1},
		{name: "true after retries", profile: fastProfile(time.Second), trueAfter: 3, wantCalls: 3},
		{name: "nil profile", profile: nil, trueAfter: 1, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls int
			ok := WaitForCondition(t.Context(), tc.profile, func() bool {
				calls++
				return calls >= tc.trueAfter
			})
			require.True(t, ok)
			require.Equal(t, tc.wantCalls, calls)
		})
	}

	// Failure cases: the condition never returns true.
	for _, tc := range []struct {
		name       string
		profile    *Profile
		ctxTimeout time.Duration
	}{
		{name: "false when budget elapses", profile: fastProfile(80 * time.Millisecond)},
		{
			name: "false when context is cancelled", profile: fastProfile(5 * time.Second),
			ctxTimeout: 60 * time.Millisecond,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			if tc.ctxTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tc.ctxTimeout)
				t.Cleanup(cancel)
			}
			var calls int
			ok := WaitForCondition(ctx, tc.profile, func() bool {
				calls++
				return false
			})
			require.False(t, ok)
			require.GreaterOrEqual(t, calls, 2)
		})
	}
}

// TestExecuteSQL verifies ExecuteSQL against a hermetic fake executor, covering a successful
// statement, a retryable error that eventually times out, and a non-retryable (closed pool)
// error that stops retries immediately.
func TestExecuteSQL(t *testing.T) {
	t.Parallel()

	// Success cases: Exec eventually returns a CommandTag with no error.
	for _, tc := range []struct {
		name      string
		profile   *Profile
		exec      *fakeExecutor
		wantCalls int
	}{
		{
			name:      "succeeds on first attempt",
			profile:   fastProfile(time.Second),
			exec:      &fakeExecutor{tag: pgconn.NewCommandTag("INSERT 0 1"), failUntil: 1},
			wantCalls: 1,
		},
		{
			name:    "succeeds after transient retryable errors",
			profile: fastProfile(time.Second),
			exec: &fakeExecutor{
				tag: pgconn.NewCommandTag("UPDATE 2"), err: errors.New("connection reset"), failUntil: 3,
			},
			wantCalls: 3,
		},
		{
			name:      "nil profile succeeds",
			profile:   nil,
			exec:      &fakeExecutor{tag: pgconn.NewCommandTag("SELECT 1"), failUntil: 1},
			wantCalls: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ExecuteSQL(t.Context(), tc.profile, tc.exec, "SELECT 1", 1, "a")
			require.NoError(t, err)
			require.Equal(t, tc.wantCalls, tc.exec.callCount)
		})
	}

	// Failure cases: Exec always returns an error.
	for _, tc := range []struct {
		name         string
		profile      *Profile
		exec         *fakeExecutor
		wantErrIs    error
		wantMinCalls int
		wantMaxCalls int
	}{
		{
			name:         "retryable error times out after multiple attempts",
			profile:      fastProfile(80 * time.Millisecond),
			exec:         &fakeExecutor{err: errors.New("connection reset"), failUntil: -1},
			wantMinCalls: 2,
			wantMaxCalls: -1, // unbounded
		},
		{
			name:         "closed pool error is non-retryable and stops immediately",
			profile:      fastProfile(5 * time.Second),
			exec:         &fakeExecutor{err: puddle.ErrClosedPool, failUntil: -1},
			wantErrIs:    puddle.ErrClosedPool,
			wantMinCalls: 1,
			wantMaxCalls: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ExecuteSQL(t.Context(), tc.profile, tc.exec, "SELECT 1")
			require.ErrorContains(t, err, "failed to execute the SQL statement [SELECT 1]")
			if tc.wantErrIs != nil {
				require.ErrorIs(t, err, tc.wantErrIs)
			}
			require.GreaterOrEqual(t, tc.exec.callCount, tc.wantMinCalls)
			if tc.wantMaxCalls >= 0 {
				require.LessOrEqual(t, tc.exec.callCount, tc.wantMaxCalls)
			}
		})
	}
}

// makeOp returns an operation and a pointer to a call counter.
// If failUntil is negative, the operation always fails.
// Otherwise, the op returns an error until callCount >= failUntil.
func makeOp(failUntil int) (func() error, *int) {
	callCount := 0
	op := func() error {
		callCount++
		if failUntil < 0 || callCount < failUntil {
			return errors.New("error")
		}
		return nil
	}
	return op, &callCount
}

// makeResultOp returns an operation and a pointer to a call counter.
// It fails (returning the sentinel result -1 and an error) until callCount >= failUntil,
// then succeeds returning value. A negative failUntil means the operation always fails.
func makeResultOp(failUntil, value int) (func() (int, error), *int) {
	callCount := 0
	op := func() (int, error) {
		callCount++
		if failUntil < 0 || callCount < failUntil {
			return -1, errors.New("boom")
		}
		return value, nil
	}
	return op, &callCount
}

// fastProfile returns a profile with tiny intervals so retry tests run quickly.
func fastProfile(maxElapsed time.Duration) *Profile {
	return &Profile{
		InitialInterval: time.Millisecond,
		MaxInterval:     10 * time.Millisecond,
		MaxElapsedTime:  new(maxElapsed),
	}
}

// fakeExecutor is a hermetic stand-in for the SQL executor used by ExecuteSQL.
// It records the number of Exec calls and returns a scripted CommandTag/error:
// Exec fails with err until callCount >= failUntil, then returns tag. A negative
// failUntil makes Exec always fail.
type fakeExecutor struct {
	tag       pgconn.CommandTag
	err       error
	failUntil int
	callCount int
}

func (f *fakeExecutor) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	f.callCount++
	if f.failUntil < 0 || f.callCount < f.failUntil {
		return pgconn.CommandTag{}, f.err
	}
	return f.tag, nil
}
