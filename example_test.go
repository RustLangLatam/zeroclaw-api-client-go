package zeroclaw_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	zeroclaw "github.com/RustLangLatam/zeroclaw-api-client-go"
)

func Example() {
	client, err := zeroclaw.New("http://127.0.0.1:42617", zeroclaw.WithToken("zc_…"))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	status, err := client.System().Status(ctx, "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("ZeroClaw", status.Version)

	if _, err := client.Config().Set(ctx, "agents.helper.model", "gpt-5.4-mini", ""); err != nil {
		log.Fatal(err)
	}
	chat, err := client.Chat(ctx, "helper", zeroclaw.ChatOptions{})
	if err != nil {
		log.Fatal(err)
	}
	defer chat.Close()
	reply, err := chat.Ask(ctx, "How many orders are pending?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(reply.FullResponse)
}

func ExampleNew_resilience() {
	client, err := zeroclaw.New("http://127.0.0.1:42617",
		zeroclaw.WithToken("zc_…"),
		zeroclaw.WithTimeout(10*time.Second), // wait for an answer at most 10 s
		zeroclaw.WithRetry(zeroclaw.RetryPolicy{MaxRetries: 5, BaseDelay: 250 * time.Millisecond, MaxDelay: 8 * time.Second}),
		zeroclaw.WithConcurrencyLimit(4), // shared by every copy
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleWithMiddleware() {
	caller := func(next http.RoundTripper) http.RoundTripper {
		return zeroclaw.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			r = r.Clone(r.Context())
			r.Header.Set("X-Caller", "billing")
			return next.RoundTrip(r)
		})
	}
	client, err := zeroclaw.New("http://127.0.0.1:42617",
		zeroclaw.WithToken("zc_…"),
		zeroclaw.WithMiddleware(caller))
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleEventsAPI_Subscribe() {
	client, _ := zeroclaw.New("http://127.0.0.1:42617", zeroclaw.WithToken("zc_…"))
	events, err := client.Events().Subscribe(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer events.Close()
	for {
		event, err := events.Next()
		if errors.Is(err, io.EOF) {
			return // the gateway closed the stream; subscribe again to resume
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(event.Kind())
	}
}

func ExampleChatSession_NextFrame() {
	client, _ := zeroclaw.New("http://127.0.0.1:42617", zeroclaw.WithToken("zc_…"))
	ctx := context.Background()
	chat, err := client.Chat(ctx, "helper", zeroclaw.ChatOptions{})
	if err != nil {
		log.Fatal(err)
	}
	defer chat.Close()
	chat.Send(ctx, "Clean the tmp folder")
	for {
		frame, err := chat.NextFrame(ctx)
		if err != nil {
			log.Fatal(err)
		}
		switch frame.Type {
		case zeroclaw.FrameChunk:
			fmt.Print(frame.Content)
		case zeroclaw.FrameApprovalRequest:
			chat.AnswerApproval(ctx, frame.RequestID, "approve")
		case zeroclaw.FrameDone:
			return
		}
	}
}

func ExampleIsTransient() {
	client, _ := zeroclaw.New("http://127.0.0.1:42617", zeroclaw.WithToken("zc_…"))
	_, err := client.Cron().List(context.Background())
	switch {
	case err == nil:
	case zeroclaw.IsUnauthorized(err):
		fmt.Println("pair again:", err)
	case zeroclaw.IsTransient(err):
		fmt.Println("try later:", err) // idempotent calls were already retried
	default:
		var api *zeroclaw.APIError
		if errors.As(err, &api) {
			fmt.Println(api.Status, api.Message, string(api.Body))
		}
	}
}

func ExampleConfigAPI_Patch() {
	client, _ := zeroclaw.New("http://127.0.0.1:42617", zeroclaw.WithToken("zc_…"))
	// All or nothing: the model only changes if it is still the one read.
	_, err := client.Config().Patch(context.Background(), []zeroclaw.PatchOp{
		zeroclaw.TestOp("agents.helper.model", "gpt-5.4-mini"),
		zeroclaw.ReplaceOp("agents.helper.model", "gpt-5.5"),
		zeroclaw.CommentOp("agents.helper.model", "upgraded"),
	}, false)
	if err != nil {
		log.Fatal(err)
	}
}
