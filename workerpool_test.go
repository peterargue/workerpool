package workerpool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInvalid(t *testing.T) {
	// No producers specified.
	p := New[int](0).
		AddConsumers(1, func(ctx context.Context, v int) error { return nil })
	err := p.Run(t.Context())
	require.Error(t, err, "expected error when no producers provided")

	// No consumers specified.
	p = New[int](0).
		AddProducers(1, func(ctx context.Context, add AddFunc[int]) error { return nil })
	err = p.Run(t.Context())
	require.Error(t, err, "expected error when no consumers provided")
}

// TestBasic tests a simple single producer-consumer scenario.
func TestBasic(t *testing.T) {
	var mu sync.Mutex
	var results []int
	var expected []int

	wp := New[int](10).
		AddProducers(1, producer(0, 100, func(v int) error {
			expected = append(expected, v)
			return nil
		})).
		AddConsumers(1, func(ctx context.Context, v int) error {
			mu.Lock()
			defer mu.Unlock()
			results = append(results, v)
			return nil
		})

	err := wp.Run(t.Context())
	require.NoError(t, err, "Run returned error")

	assert.Equal(t, expected, results, "expected results to match")
}

// TestConcurrentProducersConsumers tests a scenario with multiple producers and multiple consumers.
func TestConcurrentProducersConsumers(t *testing.T) {
	var count int32

	wp := New[int](20).
		AddProducers(3, producer(0, 50, func(v int) error {
			time.Sleep(1 * time.Millisecond) // simulate work
			return nil
		})).
		AddConsumers(3, func(ctx context.Context, v int) error {
			atomic.AddInt32(&count, 1)
			time.Sleep(2 * time.Millisecond) // simulate processing delay
			return nil
		})

	start := time.Now()
	err := wp.Run(t.Context())
	require.NoError(t, err, "Run returned error")
	duration := time.Since(start)

	expected := int32(150) // 3 producers * 50 items each
	assert.Equal(t, expected, atomic.LoadInt32(&count), "expected count to be %d", expected)

	// If processing were entirely sequential, it would take at least 150*2ms = 300ms.
	// We expect the concurrent pool to complete significantly faster.
	assert.Less(t, duration, 300*time.Millisecond, "run took too long (%v), expected concurrent execution", duration)
}

func TestTimeout(t *testing.T) {
	wp := New[int](5).
		AddProducers(1, producer(0, 1000, func(v int) error {
			time.Sleep(5 * time.Millisecond)
			return nil
		})).
		AddConsumers(1, func(ctx context.Context, v int) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	err := wp.Run(ctx)
	require.Error(t, err, "expected error due to context cancellation")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestContextCancellation(t *testing.T) {
	wp := New[int](5).
		AddProducers(1, producer(0, 1000, func(v int) error {
			time.Sleep(5 * time.Millisecond)
			return nil
		})).
		AddConsumers(1, func(ctx context.Context, v int) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := wp.Run(ctx)
	require.Error(t, err, "expected error due to context cancellation")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestProducerError(t *testing.T) {
	testErr := errors.New("test producer error")

	wp := New[int](10).
		AddProducers(1, func(ctx context.Context, add AddFunc[int]) error {
			return testErr
		}).
		AddConsumers(1, func(ctx context.Context, v int) error {
			return nil
		})

	err := wp.Run(t.Context())
	require.Error(t, err, "expected error from producer")
	assert.ErrorIs(t, err, testErr)
}

func TestConsumerError(t *testing.T) {
	target := 42
	testErr := errors.New("test consumer error")

	wp := New[int](10).
		AddProducers(1, producer(0, 100, nil)).
		AddConsumers(1, func(ctx context.Context, v int) error {
			if v == target {
				return testErr
			}
			return nil
		})

	err := wp.Run(t.Context())
	require.Error(t, err, "expected error from consumer")
	assert.ErrorIs(t, err, testErr)
}

func TestBufferDrain(t *testing.T) {
	var received, expected []int

	wp := New[int](5).
		AddProducers(1, producer(0, 20, func(v int) error {
			expected = append(expected, v)
			return nil
		})).
		AddConsumers(1, func(ctx context.Context, v int) error {
			received = append(received, v)
			time.Sleep(20 * time.Millisecond) // simulate slow processing
			return nil
		})

	err := wp.Run(t.Context())
	require.NoError(t, err)
	assert.Equal(t, expected, received)
}

func TestUnbuffered(t *testing.T) {
	var received, expected []int

	wp := New[int](0).
		AddProducers(1, producer(0, 10, func(v int) error {
			expected = append(expected, v)
			return nil
		})).
		AddConsumers(1, func(ctx context.Context, v int) error {
			received = append(received, v)
			return nil
		})

	err := wp.Run(t.Context())
	require.NoError(t, err)
	assert.Equal(t, expected, received)
}

// producer returns a producer function that generates integers from offset to offset+total.
// If fn is provided, it is called for each generated value. If fn returns an error, the producer
// aborts and returns the error.
func producer(offset, total int, fn func(v int) error) Producer[int] {
	if fn == nil {
		fn = func(v int) error { return nil }
	}
	return func(ctx context.Context, add AddFunc[int]) error {
		for i := 0; i < total; i++ {
			select {
			case <-ctx.Done():
				return nil
			default:
			}

			v := i + offset
			add(v)
			if err := fn(v); err != nil {
				return err
			}
		}
		return nil
	}
}
