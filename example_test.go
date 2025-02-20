package workerpool_test

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/peterargue/workerpool"
)

func ExampleWorkerpool() {
	var mu sync.Mutex
	var results []int

	// Create a new worker pool with a channel buffer size of 10.
	// Configure the worker pool with one producer and two consumers.
	wp := workerpool.New[int](10).
		AddProducers(1, func(ctx context.Context, add workerpool.AddFunc[int]) error {
			for i := 0; i < 10; i++ {
				add(i)
				time.Sleep(10 * time.Millisecond) // simulate some work
			}
			return nil
		}).
		AddConsumers(2, func(ctx context.Context, v int) error {
			mu.Lock()
			defer mu.Unlock()
			results = append(results, v)
			return nil
		})

	// Run the worker pool.
	if err := wp.Run(context.Background()); err != nil {
		fmt.Printf("Workerpool error: %v\n", err)
		return
	}

	// Sort the results to display them in order.
	sort.Ints(results)

	fmt.Println("Processed items:", results)
	// Output:
	// Processed items: [0 1 2 3 4 5 6 7 8 9]
}
