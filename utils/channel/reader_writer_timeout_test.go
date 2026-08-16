/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package channel_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/utils/channel"
)

func TestMake(t *testing.T) {
	t.Parallel()

	d := &data{val: 42}

	for _, tc := range []struct {
		name string
		size int
	}{
		{name: "buffered channel", size: 1},
		{name: "unbuffered channel", size: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)

			c := channel.Make[*data](ctx, tc.size)
			require.NotNil(t, c)
			require.Equal(t, ctx, c.Context())

			// The returned ReaderWriter round-trips a value.
			go func() {
				assert.True(t, c.Write(d))
			}()
			val, ok := c.Read()
			require.True(t, ok)
			require.Equal(t, d, val)
		})
	}
}

func TestReadWithTimeout(t *testing.T) {
	t.Parallel()

	d := &data{val: 7}

	// Success: a value is returned before the timeout elapses.
	for _, tc := range []struct {
		name string
		size int
	}{
		{name: "value already buffered", size: 1},
		{name: "value delivered by writer", size: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)

			ch := make(chan *data, tc.size)
			go func() {
				ch <- d
			}()

			c := channel.NewReader(ctx, ch)
			val, ok := c.ReadWithTimeout(500 * time.Millisecond)
			require.True(t, ok)
			require.Equal(t, d, val)
		})
	}

	// Failure: the zero value and false are returned.
	for _, tc := range []struct {
		name         string
		inputNil     bool
		closed       bool
		cancelBefore bool
		cancelAfter  time.Duration
		timeout      time.Duration
	}{
		{name: "timeout elapses on empty channel", timeout: 30 * time.Millisecond},
		{name: "context already cancelled", cancelBefore: true, timeout: time.Second},
		{name: "cancelled while waiting", cancelAfter: 20 * time.Millisecond, timeout: time.Second},
		{name: "nil input channel", inputNil: true, timeout: 30 * time.Millisecond},
		{name: "closed channel", closed: true, timeout: time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)

			var c channel.Reader[*data]
			if tc.inputNil {
				c = channel.NewReader[*data](ctx, nil)
			} else {
				ch := make(chan *data)
				if tc.closed {
					close(ch)
				}
				c = channel.NewReader(ctx, ch)
			}

			if tc.cancelBefore {
				cancel()
			}
			cancelAfterDelay(cancel, tc.cancelAfter)

			val, ok := c.ReadWithTimeout(tc.timeout)
			require.False(t, ok)
			require.Nil(t, val)
		})
	}
}

func TestWriteWithTimeout(t *testing.T) {
	t.Parallel()

	d := &data{val: 9}

	// Success: the value is written before the timeout elapses.
	for _, tc := range []struct {
		name string
		size int
	}{
		{name: "buffered channel has free space", size: 1},
		{name: "unbuffered channel with waiting reader", size: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)

			ch := make(chan *data, tc.size)
			received := make(chan *data, 1)
			go func() {
				received <- <-ch
			}()

			c := channel.NewWriter(ctx, ch)
			require.True(t, c.WriteWithTimeout(d, 500*time.Millisecond))
			require.Equal(t, d, <-received)
		})
	}

	// Failure: false is returned and no value is delivered.
	for _, tc := range []struct {
		name         string
		outputNil    bool
		prefill      bool
		cancelBefore bool
		cancelAfter  time.Duration
		timeout      time.Duration
	}{
		{name: "timeout elapses on full channel", prefill: true, timeout: 30 * time.Millisecond},
		{name: "context already cancelled", cancelBefore: true, timeout: time.Second},
		{name: "cancelled while waiting", prefill: true, cancelAfter: 20 * time.Millisecond, timeout: time.Second},
		{name: "nil output channel", outputNil: true, timeout: 30 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)

			var c channel.Writer[*data]
			if tc.outputNil {
				c = channel.NewWriter[*data](ctx, nil)
			} else {
				ch := make(chan *data, 1)
				if tc.prefill {
					ch <- &data{val: 1}
				}
				c = channel.NewWriter(ctx, ch)
			}

			if tc.cancelBefore {
				cancel()
			}
			cancelAfterDelay(cancel, tc.cancelAfter)

			require.False(t, c.WriteWithTimeout(d, tc.timeout))
		})
	}
}

// cancelAfterDelay cancels the context from a background goroutine after the
// given delay, so a blocked read or write observes context cancellation while
// waiting. A non-positive delay is a no-op.
func cancelAfterDelay(cancel context.CancelFunc, delay time.Duration) {
	if delay <= 0 {
		return
	}
	go func() {
		time.Sleep(delay)
		cancel()
	}()
}
