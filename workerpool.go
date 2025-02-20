package workerpool

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/errgroup"
)

// Producer is a function that produces work items of type T using the provided add function.
// It returns an error if production fails.
type Producer[T any] func(ctx context.Context, add AddFunc[T]) error

// Consumer is a function that processes a work item of type T.
// It returns an error if processing fails.
type Consumer[T any] func(ctx context.Context, v T) error

// AddFunc is a function that adds a work item of type T to the work queue.
type AddFunc[T any] func(v T)

// Workerpool is a worker pool that manages a set of producers and consumers.
// It can be configured with arbitrary numbers of producers and consumers that work on objects of
// type T. The worker pool has a buffer size that limits the number of items that can be queued.
type Workerpool[T any] struct {
	producers []Producer[T]
	consumers []Consumer[T]
	buffer    int
}

func New[T any](buffer int) *Workerpool[T] {
	return &Workerpool[T]{
		buffer: buffer,
	}
}

// AddProducers adds count instances of producerFn to the worker pool.
func (p *Workerpool[T]) AddProducers(count int, producerFn Producer[T]) *Workerpool[T] {
	for i := 0; i < count; i++ {
		p.producers = append(p.producers, producerFn)
	}
	return p
}

// AddConsumers adds count instances of consumerFn to the worker pool.
func (p *Workerpool[T]) AddConsumers(count int, consumerFn Consumer[T]) *Workerpool[T] {
	for i := 0; i < count; i++ {
		p.consumers = append(p.consumers, consumerFn)
	}
	return p
}

// Run runs the worker pool. It returns an error if any producer or consumer fails.
func (p *Workerpool[T]) Run(ctx context.Context) error {
	if len(p.producers) == 0 {
		return fmt.Errorf("must specify at least 1 producers. 0 provided")
	}

	if len(p.consumers) == 0 {
		return fmt.Errorf("must specify at least 1 consumers. 0 provided")
	}

	workCh := make(chan T, p.buffer)

	// create a context that will be canceled if any producer or consumer fails to stop
	// all workers
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	// Launch producers
	gProd, pCtx := errgroup.WithContext(ctx)

	addFn := func(v T) {
		_ = PushOnChannel(pCtx, workCh, v)
	}

	for _, fn := range p.producers {
		gProd.Go(func() error {
			if err := fn(pCtx, addFn); err != nil {
				return fmt.Errorf("producer error: %w", err)
			}
			return nil
		})
	}

	// Launch consumers.
	gCons, gCtx := errgroup.WithContext(ctx)
	for _, fn := range p.consumers {
		gCons.Go(func() error {
			for {
				select {
				case <-gCtx.Done():
					return gCtx.Err()
				case v, ok := <-workCh:
					if !ok {
						return nil // channel closed; exit loop
					}
					if err := fn(gCtx, v); err != nil {
						return fmt.Errorf("consumer error: %w", err)
					}
				}
			}
		})
	}

	wg := sync.WaitGroup{}
	wg.Add(2)

	// Wait for producers and consumers to finish.
	// abort if any producer or consumer fails.
	errs := new(multierror.Error)
	go func() {
		defer wg.Done()
		defer close(workCh) // close work channel when producers are done
		if err := gProd.Wait(); err != nil {
			cancel(fmt.Errorf("producer aborted"))
			errs = multierror.Append(errs, err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := gCons.Wait(); err != nil {
			cancel(fmt.Errorf("consumer aborted"))
			errs = multierror.Append(errs, err)
		}
	}()

	wg.Wait()

	return errs.ErrorOrNil()
}

func PushOnChannel[T any](ctx context.Context, ch chan<- T, v T) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case ch <- v:
		return nil
	}
}
