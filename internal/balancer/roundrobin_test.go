package balancer

import (
	"errors"
	"fmt"
	"net/url"
	"sync"
	"testing"

	"github.com/Ebukamee/load-balancer/internal/backend"
)

// makeBackends returns n backends with URLs http://b1, http://b2, ...
func makeBackends(n int) []*backend.Backend {
	bs := make([]*backend.Backend, n)
	for i := range bs {
		bs[i] = &backend.Backend{URL: &url.URL{Scheme: "http", Host: fmt.Sprintf("b%d", i+1)}}
	}
	return bs
}

func TestRoundRobinCyclesInOrder(t *testing.T) {
	rr := NewRoundRobin(makeBackends(3))

	want := []string{"b1", "b2", "b3", "b1", "b2", "b3", "b1"}
	for i, w := range want {
		b, err := rr.Next()
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i+1, err)
		}
		if b.URL.Host != w {
			t.Fatalf("call %d: got %s, want %s", i+1, b.URL.Host, w)
		}
	}
}

func TestRoundRobinNoBackends(t *testing.T) {
	rr := NewRoundRobin(nil)

	b, err := rr.Next()
	if !errors.Is(err, ErrNoBackends) {
		t.Fatalf("err = %v, want ErrNoBackends", err)
	}
	if b != nil {
		t.Fatalf("backend = %v, want nil", b)
	}
}

// 100 goroutines each ask for 30 backends at the same time, like 100
// simultaneous clients. With a correct (atomic) counter every backend gets
// exactly 1000 picks. A plain int counter loses updates and the counts
// drift; `go test -race` (run in CI) reports the race directly.
func TestRoundRobinConcurrentIsEven(t *testing.T) {
	const goroutines, perGoroutine = 100, 30
	rr := NewRoundRobin(makeBackends(3))

	var mu sync.Mutex
	counts := map[string]int{}
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perGoroutine {
				b, err := rr.Next()
				if err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				counts[b.URL.Host]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	for _, host := range []string{"b1", "b2", "b3"} {
		if counts[host] != 1000 {
			t.Errorf("%s picked %d times, want 1000 (all counts: %v)", host, counts[host], counts)
		}
	}
}
