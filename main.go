package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"
)

type LifeType struct {
	Id         int64
	SomeError  bool
	TaskResult bool
	Start      time.Time
}

const FR = 250
const TIMEOUT = 3

func generator(ctx context.Context, wg *sync.WaitGroup) <-chan LifeType {
	var cnt int64
	ch := make(chan LifeType, 10)

	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				close(ch)
				return
			default:
				atomic.AddInt64(&cnt, 1)
				t := time.Now()
				se := false
				if t.Nanosecond()%2 > 0 {
					se = true
				}
				ch <- LifeType{Start: time.Now(), Id: cnt, SomeError: se}
			}
		}
	}()

	return ch
}

func taskWorker(w LifeType) LifeType {
	if w.Start.After(time.Now().Add(-20 * time.Second)) {
		w.TaskResult = true
	}

	time.Sleep(FR * time.Millisecond)

	return w
}

func main() {
	wg := sync.WaitGroup{}

	ctxN, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ctx, cancel := context.WithTimeout(ctxN, time.Second*TIMEOUT)
	defer cancel()

	doneTasks := make(chan LifeType)
	undoneTasks := make(chan error)
	superChann := generator(ctx, &wg)

	wg.Add(1)
	go func(context.Context) {
		for {
			select {
			case <-ctx.Done():
				wg.Done()
				return
			case tc := <-superChann:

				if tc.SomeError {
					undoneTasks <- fmt.Errorf("Task id %d time %s, error %s", tc.Id, tc.Start, "Some error occured")
				} else {
					t := taskWorker(tc)

					if t.TaskResult {
						doneTasks <- t
					} else {
						undoneTasks <- fmt.Errorf("Task id %d time %s, error %s", t.Id, t.Start, "something went wrong")
					}
				}
			}
		}
	}(ctx)

	result := map[int64]LifeType{}
	err := []error{}
	mu := sync.Mutex{}

	go func() {
		for r := range doneTasks {
			mu.Lock()
			id := r.Id
			result[id] = r
			mu.Unlock()
		}
	}()

	go func() {
		for r := range undoneTasks {
			err = append(err, r)
		}
	}()

	wg.Wait()

	close(doneTasks)
	close(undoneTasks)

	println("Errors:")
	for r := range err {
		fmt.Println(r)
	}

	println("Done tasks:")
	for _, v := range result {
		fmt.Printf("task %d has been successed %s \n", v.Id, v.Start)
	}
}
