package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func newFakeSSMClient(handler http.HandlerFunc) (*ssm.Client, func()) {
	srv := httptest.NewServer(handler)
	client := ssm.NewFromConfig(aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("fake", "fake", ""),
	}, func(o *ssm.Options) {
		o.BaseEndpoint = aws.String(srv.URL)
	})
	return client, srv.Close
}

func ssmParamHandler(value string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Parameter": map[string]any{
				"Name":  "/test/param",
				"Value": value,
			},
		})
	}
}

func TestLiveInt_usesDefaultWhenFetchFails(t *testing.T) {
	client, cleanup := newFakeSSMClient(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l := NewLiveInt(ctx, client, "/test/param", 99, time.Hour)
	if l.Get() != 99 {
		t.Fatalf("expected default 99, got %d", l.Get())
	}
}

func TestLiveInt_fetchesInitialValue(t *testing.T) {
	client, cleanup := newFakeSSMClient(ssmParamHandler("42"))
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l := NewLiveInt(ctx, client, "/test/param", 0, time.Hour)
	if l.Get() != 42 {
		t.Fatalf("expected 42, got %d", l.Get())
	}
}

func TestLiveInt_stopsPollingWhenContextCancelled(t *testing.T) {
	var calls int
	var mu sync.Mutex
	client, cleanup := newFakeSSMClient(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		ssmParamHandler("1")(w, r)
	})
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	NewLiveInt(ctx, client, "/test/param", 0, 20*time.Millisecond)
	cancel()

	// Wait a bit, then count calls — no new polls should happen after cancel.
	time.Sleep(80 * time.Millisecond)
	mu.Lock()
	before := calls
	mu.Unlock()

	time.Sleep(80 * time.Millisecond)
	mu.Lock()
	after := calls
	mu.Unlock()

	if after != before {
		t.Fatalf("expected polling to stop after cancel, got %d extra calls", after-before)
	}
}

func TestLiveInt_pollsAndUpdatesValue(t *testing.T) {
	var mu sync.Mutex
	paramVal := "10"
	client, cleanup := newFakeSSMClient(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		v := paramVal
		mu.Unlock()
		ssmParamHandler(v)(w, r)
	})
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l := NewLiveInt(ctx, client, "/test/param", 0, 30*time.Millisecond)
	if l.Get() != 10 {
		t.Fatalf("expected initial value 10, got %d", l.Get())
	}

	mu.Lock()
	paramVal = "99"
	mu.Unlock()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if l.Get() == 99 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("value never updated to 99, got %d", l.Get())
}
