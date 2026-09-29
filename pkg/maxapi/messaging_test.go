package maxapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendMessageToUser(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/messages" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("user_id"); got != "123" {
			t.Fatalf("user_id = %q, want 123", got)
		}
		if r.URL.Query().Has("chat_id") {
			t.Fatalf("unexpected chat_id in query: %v", r.URL.Query())
		}
		if got := r.Header.Get("Authorization"); got != "test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q", got)
		}
		var message NewMessage
		if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if message.Text != "hello" || len(message.Attachments) != 0 {
			t.Fatalf("message = %+v", message)
		}
		_, _ = w.Write([]byte(`{"message":{"mid":"m1","timestamp":1,"body":{"text":"hello"}}}`))
	}))
	t.Cleanup(server.Close)

	client := New(server.URL, "test-token")
	if _, err := client.SendMessageToUser(context.Background(), 123, NewMessage{Text: "hello"}); err != nil {
		t.Fatalf("SendMessageToUser: %v", err)
	}
}
