package rmq

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
)

// --- Error type ---

func TestNewError_setsFieldsCorrectly(t *testing.T) {
	err := NewError(404, "not found")
	if err.Code != 404 {
		t.Fatalf("expected code 404, got %d", err.Code)
	}
	if err.Message != "not found" {
		t.Fatalf("expected message %q, got %q", "not found", err.Message)
	}
}

func TestNewError_panicOnZeroCode(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for code 0")
		}
	}()
	NewError(0, "bad")
}

func TestError_Error_returnsMessage(t *testing.T) {
	err := NewError(500, "internal")
	if err.Error() != "internal" {
		t.Fatalf("expected %q, got %q", "internal", err.Error())
	}
}

func TestError_Is_matchesByCode(t *testing.T) {
	err := NewError(1001, "original message")
	sameCode := NewError(1001, "different message")
	diffCode := NewError(1002, "original message")

	if !errors.Is(err, sameCode) {
		t.Error("errors.Is should match on same code regardless of message")
	}
	if errors.Is(err, diffCode) {
		t.Error("errors.Is should not match on different code")
	}
}

func TestError_Is_worksAfterWrapping(t *testing.T) {
	sentinel := NewError(1001, "not found")
	wrapped := fmt.Errorf("outer: %w", sentinel)
	if !errors.Is(wrapped, sentinel) {
		t.Error("errors.Is should unwrap to find the rmq.Error")
	}
}

// --- ConvertHttpError ---

func TestConvertHttpError_nonRMQErrorReturnsNil(t *testing.T) {
	if got := ConvertHttpError(errors.New("plain error")); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestConvertHttpError_serviceLevelCodeReturnsNil(t *testing.T) {
	// Codes >= 1000 are service-level; the caller handles them.
	for _, code := range []int{1000, 1001, 9999} {
		if got := ConvertHttpError(NewError(code, "svc")); got != nil {
			t.Errorf("code %d: expected nil, got %v", code, got)
		}
	}
}

func TestConvertHttpError_503MapsToServiceUnavailable(t *testing.T) {
	got := ConvertHttpError(NewError(503, "broker down"))
	he, ok := got.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", got)
	}
	if he.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", he.Code)
	}
}

func TestConvertHttpError_otherSystemCodeMapsTo500(t *testing.T) {
	got := ConvertHttpError(NewError(400, "bad"))
	he, ok := got.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", got)
	}
	if he.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", he.Code)
	}
}

// --- envelope helpers ---

func TestWrapSuccess_roundTrip(t *testing.T) {
	payload := []byte(`{"key":"value"}`)
	b, err := wrapSuccess(payload)
	if err != nil {
		t.Fatal(err)
	}

	var env envelope
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if env.Version != envelopeVersion {
		t.Fatalf("expected version %d, got %d", envelopeVersion, env.Version)
	}
	if env.Type != responseTypeSuccess {
		t.Fatalf("expected type %q, got %q", responseTypeSuccess, env.Type)
	}
	if string(env.Response) != string(payload) {
		t.Fatalf("expected response %s, got %s", payload, env.Response)
	}
}

func TestWrapError_roundTrip(t *testing.T) {
	original := NewError(1234, "something failed")
	b, err := wrapError(original)
	if err != nil {
		t.Fatal(err)
	}

	var env envelope
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if env.Type != responseTypeError {
		t.Fatalf("expected type %q, got %q", responseTypeError, env.Type)
	}

	var rmqErr Error
	if err := json.Unmarshal(env.Response, &rmqErr); err != nil {
		t.Fatal(err)
	}
	if rmqErr.Code != original.Code {
		t.Fatalf("expected code %d, got %d", original.Code, rmqErr.Code)
	}
	if rmqErr.Message != original.Message {
		t.Fatalf("expected message %q, got %q", original.Message, rmqErr.Message)
	}
}
