// Command worker-pool runs a fixed number of workers over a bounded queue.
//
//	go run ./docs-learn/examples/04-worker-pool
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type job struct {
	ID int
}

func main() {
	const (
		workers = 3
		queue   = 10
		jobs    = 20
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := make(chan job, queue)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range ch {
				select {
				case <-ctx.Done():
					return
				case <-time.After(50 * time.Millisecond):
					fmt.Printf("worker %d handled job %d\n", id, j.ID)
				}
			}
		}(i)
	}

	for i := 0; i < jobs; i++ {
		select {
		case ch <- job{ID: i}:
		case <-ctx.Done():
			fmt.Println("producer stopped:", ctx.Err())
			close(ch)
			wg.Wait()
			return
		}
	}
	close(ch)
	wg.Wait()
}
