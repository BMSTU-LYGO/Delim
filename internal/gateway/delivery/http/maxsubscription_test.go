package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"delim/internal/gateway/auth"
	"delim/pkg/maxapi"
)

type subscriptionStoreFake struct {
	connected      bool
	disabled       int64
	upsertedUserID int64
	upsertedChatID int64
}

func (s *subscriptionStoreFake) GetPersonalSubscription(_ context.Context, _ int64) (bool, error) {
	return s.connected, nil
}

func (s *subscriptionStoreFake) UpsertPersonalSubscription(_ context.Context, maxUserID, chatID int64) error {
	s.upsertedUserID = maxUserID
	s.upsertedChatID = chatID
	s.connected = true
	return nil
}

func (s *subscriptionStoreFake) DisablePersonalSubscription(_ context.Context, maxUserID int64) error {
	s.disabled = maxUserID
	s.connected = false
	return nil
}

func subscriptionRequest(method string) *http.Request {
	req := httptest.NewRequest(method, "/api/v1/max-subscription", nil)
	return req.WithContext(context.WithValue(req.Context(), sessionContextKey{}, auth.Session{UserID: 1, MAXUserID: 42, ChatID: 111}))
}

func newSubscriptionMAXAPI(t *testing.T, chatType string) *maxapi.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/chats/111" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, `{"chat_id":111,"type":"`+chatType+`"}`)
	}))
	t.Cleanup(server.Close)
	return maxapi.New(server.URL, "test-token")
}

func TestPersonalSubscriptionHandlers(t *testing.T) {
	store := &subscriptionStoreFake{}

	connect := httptest.NewRecorder()
	connectMaxSubscription(store, newSubscriptionMAXAPI(t, "dialog")).ServeHTTP(connect, subscriptionRequest(http.MethodPost))
	if connect.Code != http.StatusOK || connect.Body.String() != "{\"connected\":true}\n" || store.upsertedUserID != 42 || store.upsertedChatID != 111 {
		t.Fatalf("connect response = %d %s", connect.Code, connect.Body.String())
	}

	store.connected = true
	get := httptest.NewRecorder()
	getMaxSubscription(store).ServeHTTP(get, subscriptionRequest(http.MethodGet))
	if get.Code != http.StatusOK || get.Body.String() != "{\"connected\":true}\n" {
		t.Fatalf("get response = %d %s", get.Code, get.Body.String())
	}

	disable := httptest.NewRecorder()
	disableMaxSubscription(store).ServeHTTP(disable, subscriptionRequest(http.MethodDelete))
	if disable.Code != http.StatusNoContent || store.disabled != 42 {
		t.Fatalf("disable = %d, user = %d", disable.Code, store.disabled)
	}
}

func TestConnectMaxSubscriptionRejectsNonDialog(t *testing.T) {
	store := &subscriptionStoreFake{}
	response := httptest.NewRecorder()
	connectMaxSubscription(store, newSubscriptionMAXAPI(t, "chat")).ServeHTTP(response, subscriptionRequest(http.MethodPost))
	if response.Code != http.StatusConflict || store.upsertedUserID != 0 {
		t.Fatalf("connect response = %d %s", response.Code, response.Body.String())
	}
}
