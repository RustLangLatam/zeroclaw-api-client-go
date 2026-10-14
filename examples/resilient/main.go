// Command resilient shows a client tuned for an unreliable network: a short
// answer timeout, five retries with jittered backoff, at most four requests
// in flight, ten requests per second, a circuit breaker that stops calling a
// gateway that keeps failing, and a middleware of its own that times every
// request.
//
//	ZEROCLAW_URL=… ZEROCLAW_TOKEN=… go run ./examples/resilient
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	zeroclaw "github.com/RustLangLatam/zeroclaw-api-client-go"
	"github.com/sony/gobreaker/v2"
	"golang.org/x/time/rate"
)

// timed is the outermost middleware: it sees each call once, retries
// included.
func timed(next http.RoundTripper) http.RoundTripper {
	return zeroclaw.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		started := time.Now()
		resp, err := next.RoundTrip(r)
		fmt.Fprintf(os.Stderr, "  %s %s took %v\n", r.Method, r.URL.Path, time.Since(started))
		return resp, err
	})
}

func main() {
	client, err := zeroclaw.New(gatewayURL(),
		zeroclaw.WithToken(os.Getenv("ZEROCLAW_TOKEN")),
		zeroclaw.WithTimeout(10*time.Second),
		zeroclaw.WithRetry(zeroclaw.RetryPolicy{MaxRetries: 5, BaseDelay: 250 * time.Millisecond, MaxDelay: 8 * time.Second, Jitter: true}),
		zeroclaw.WithConcurrencyLimit(4),
		zeroclaw.WithRateLimit(rate.Limit(10), 10),
		zeroclaw.WithCircuitBreaker(gobreaker.Settings{
			Timeout: 15 * time.Second,
			OnStateChange: func(_ string, from, to gobreaker.State) {
				fmt.Fprintf(os.Stderr, "  circuit %s -> %s\n", from, to)
			},
		}),
		zeroclaw.WithMiddleware(timed),
	)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	status, err := client.System().Status(ctx, "")
	switch {
	case err == nil:
		fmt.Printf("ZeroClaw %s up %ds\n", status.Version, status.UptimeSeconds)
	case errors.Is(err, zeroclaw.ErrCircuitOpen):
		fmt.Println("the gateway keeps failing; not calling it for now:", err)
	case errors.Is(err, zeroclaw.ErrTimeout):
		fmt.Println("no answer even after retries:", err)
	case zeroclaw.IsTransient(err):
		fmt.Println("still unavailable:", err)
	default:
		log.Fatal(err)
	}

	// The client is safe for concurrent use: eight calls share the
	// connection pool, and at most four wait for an answer at once.
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			channels, err := client.Channels().List(ctx)
			if err != nil {
				fmt.Println("channels:", err)
				return
			}
			fmt.Printf("%d channels\n", len(channels))
		}()
	}
	wg.Wait()
}

func gatewayURL() string {
	if url := os.Getenv("ZEROCLAW_URL"); url != "" {
		return url
	}
	return "http://127.0.0.1:42617"
}
