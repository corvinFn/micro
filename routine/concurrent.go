package routine

import (
	"context"
	"sync"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"golang.org/x/sync/errgroup"
)

// Routine is a helper function to parallel operate on slice.
// Example usage:
// ```
//   itemIDs := []int{1, 2, 3}
//   items := make([]*Item, len(itemIDs))
//   routine.Routine(len(items), 10, func(i int) {
//     items[i] = getItemByID(itemIDs[i])
//   })
// ```
func Routine(size, numWorker int, f func(i int) error) error {
	if numWorker == 1 {
		return doSimpleRoutine(size, f)
	}
	return RoutineWithTimeout(size, numWorker, time.Hour*24*365, func(ctx context.Context, i int) error {
		err := f(i)
		return err
	})
}

// RoutineWithTimeout is the same as Routine, except each function need to be executed within timeout.
// Important reminder to handle the context provided:
// It is better to check the `ctx.Err() == nil` before any write operation, because the
// function may be canceled at any momement, and the underlying resource refering may have already been
// discarded.
func RoutineWithTimeout(size, numWorker int, timeout time.Duration, f func(ctx context.Context, i int) error) error {
	return RoutineWithCtxTimeout(context.Background(), size, numWorker, timeout, f)
}

func RoutineWithCtx(baseCtx context.Context, size, numWorker int, f func(ctx context.Context, i int) error) error {
	return RoutineWithCtxTimeout(baseCtx, size, numWorker, time.Hour*24*365, f)
}

func RoutineWithCtxTimeout(baseCtx context.Context, size, numWorker int, timeout time.Duration, f func(ctx context.Context, i int) error) error {
	var allerror error
	ch := make(chan int)
	mutex := sync.Mutex{}
	go func() {
		defer close(ch)
		for i := 0; i < size; i++ {
			ch <- i
		}
	}()

	grp := errgroup.Group{}
	for i := 0; i < numWorker; i++ {
		grp.Go(func() error {
			for idx := range ch {
				ctx, cancel := context.WithTimeout(baseCtx, timeout)
				done := make(chan struct{}, 1)
				go func() {
					err := f(ctx, idx)
					if err != nil {
						mutex.Lock()
						allerror = multierror.Append(err)
						mutex.Unlock()
					}
					done <- struct{}{}
				}()
				select {
				case <-done:
					// Done in time
				case <-ctx.Done():
					// Timed out
					allerror = multierror.Append(allerror, errors.Errorf("Deadline exceeded at %d", idx))
				}

				cancel()
			}
			return nil
		})
	}
	grp.Wait()
	return allerror
}

func doSimpleRoutine(size int, f func(i int) error) error {
	var allerror error
	for i := 0; i < size; i++ {
		err := f(i)
		if err != nil {
			allerror = multierror.Append(allerror, err)
		}
	}
	return allerror
}
