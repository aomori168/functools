package functools

import (
	"context"
	"sync"
)

func ReadOnly[T any](data []T) <-chan T {
	dataCh := make(chan T)
	go func() {
		defer close(dataCh)
		for _, d := range data {
			dataCh <- d
		}
	}()
	return dataCh
}

func ReadOnlyWithCancel[T any](data []T) (<-chan T, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	dataCh := make(chan T)
	go func() {
		defer close(dataCh)
		for _, d := range data {
			select {
			case <-ctx.Done():
				return
			case dataCh <- d:
			}
		}
	}()
	return dataCh, cancel
}

func WorkWithCancel[T any](dataCh <-chan T, work func(t T)) (<-chan struct{}, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	wait := make(chan struct{})
	go func() {
		defer close(wait)
		for {
			select {
			case d, ok := <-dataCh:
				if !ok {
					return
				}
				work(d)
			case <-ctx.Done():
				return
			}
		}
	}()
	return wait, cancel
}

type Result[T any] struct {
	Error error
	Value T
}

func WorkWithResult[T any, R any](dataCh <-chan T, work func(t T) (R, error), numWorkers int) (<-chan Result[R], context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan Result[R])

	var wg sync.WaitGroup
	if numWorkers < 1 {
		numWorkers = 1
	}
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case d, ok := <-dataCh:
					if !ok {
						return
					}
					val, err := work(d)
					result := Result[R]{Error: err, Value: val}
					select {
					case resultCh <- result:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	return resultCh, cancel
}

func Or(done ...<-chan struct{}) <-chan struct{} {
	switch len(done) {
	case 0:
		return nil
	case 1:
		return done[0]
	}

	orDone := make(chan struct{})
	go func() {
		defer close(orDone)

		switch len(done) {
		case 2:
			select {
			case <-done[0]:
			case <-done[1]:
			}
		default:
			m := len(done) / 2
			select {
			case <-Or(done[:m]...):
			case <-Or(done[m:]...):
			}
		}
	}()
	return orDone
}

func OrDone[T any](done <-chan struct{}, dataCh <-chan T) <-chan T {
	ch := make(chan T)
	go func() {
		defer close(ch)
		for {
			select {
			case <-done:
				return
			case d, ok := <-dataCh:
				if !ok {
					return
				}
				select {
				case ch <- d:
				case <-done:
				}
			}
		}
	}()
	return ch
}

func TeeChan[T any](done <-chan struct{}, in <-chan T) (<-chan T, <-chan T) {
	ch1 := make(chan T)
	ch2 := make(chan T)

	go func() {
		defer close(ch1)
		defer close(ch2)

		for val := range OrDone(done, in) {
			var ch1, ch2 = ch1, ch2
			for ch1 != nil || ch2 != nil {
				select {
				case <-done:
					return
				case ch1 <- val:
					ch1 = nil
				case ch2 <- val:
					ch2 = nil
				}
			}
		}
	}()

	return ch1, ch2
}

func Bridge[T any](done <-chan struct{}, chanStream <-chan (<-chan T)) <-chan T {
	ch := make(chan T)
	go func() {
		defer close(ch)
		for {
			var stream <-chan T
			select {
			case cs, ok := <-chanStream:
				if !ok {
					return
				}
				stream = cs
			case <-done:
				return
			}
			for val := range OrDone(done, stream) {
				select {
				case ch <- val:
				case <-done:
				}
			}
		}
	}()
	return ch
}

func todo() {

}
