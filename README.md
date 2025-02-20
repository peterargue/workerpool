# Workerpool

Workerpool is a generic, concurrent worker pool module written in Go. It leverages Go generics, the `errgroup` package, and context cancellation to provide a flexible and robust solution for coordinating concurrent producers and consumers.

## Features

- **Generic Support:** Process work items of any type.
- **Concurrent Producers and Consumers:** Easily configure multiple producers and consumers.
- **Graceful Shutdown:** Ensures all work is processed even during shutdown.
- **Error Propagation:** Cancels all workers if any producer or consumer fails, aggregating errors.
- **Fluent API:** Build your worker pool using a fluent builder-style API.

## Installation

To install Workerpool, use `go get`:

```bash
go get github.com/peterargue/workerpool
```

## Usage

Workerpool supports one-to-many, many-to-one, and many-to-many concurrent produce/consumer patterns. Below is a basic example demonstrating how to create and run a worker pool.

# Example
```golang
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/yourusername/workerpool"
)

func main() {
	// Create a new worker pool with a channel buffer size of 10 and
	// configure the worker pool with one producer and three consumers.
	wp := workerpool.New[int](10).
		AddProducers(1, func(ctx context.Context, add workerpool.AddFunc[int]) error {
            for i := 0; i < 100; i++ {
                add(i)
            }
            return nil
        }).
	    AddConsumers(3, func(ctx context.Context, v int) error {
            fmt.Printf("Processed: %d\n", v)
            return nil
        })

	// Run the worker pool.
	if err := wp.Run(context.Background()); err != nil {
		fmt.Printf("Workerpool error: %v\n", err)
	}
}
```

# Use Cases
•	One-to-Many: Use a single producer (e.g., reading from a file) with multiple consumers processing the data concurrently.
•	Many-to-One: Use multiple producers (e.g., concurrent data fetchers) that feed into a single consumer (e.g., a writer).
•	Many-to-Many: Combine multiple producers and consumers for maximum parallelism.

Simply adjust the counts in AddProducers and AddConsumers to suit your application’s requirements.

Note, you can add multiple producer and consumer implementations to the worker pool so long as all work with the same underlying type. For example, you could add a producers that fetch from different data sources.

# Testing

To run the tests, execute:

```bash
go test ./...
```

# Contributing

Contributions are welcome! Please fork the repository and open a pull request with your changes. For significant changes, open an issue first to discuss your ideas.