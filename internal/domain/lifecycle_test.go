package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLifecycleTTL(t *testing.T) {
	ttl, ok := LifecycleTTL(json.RawMessage(`{"ttlSeconds":30}`))
	if !ok {
		t.Fatal("expected ttlSeconds to be detected")
	}
	if ttl != 30*time.Second {
		t.Fatalf("unexpected ttl duration: %s", ttl)
	}
}

func TestLifecycleTTLIgnoresMissingInvalidOrDisabledValues(t *testing.T) {
	cases := []json.RawMessage{
		nil,
		json.RawMessage(`{}`),
		json.RawMessage(`{"ttlSeconds":0}`),
		json.RawMessage(`{"ttlSeconds":-1}`),
		json.RawMessage(`{"ttlSeconds":"30"}`),
	}
	for _, item := range cases {
		if ttl, ok := LifecycleTTL(item); ok {
			t.Fatalf("expected %s to be ignored, got ttl %s", string(item), ttl)
		}
	}
}
