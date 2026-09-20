package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"delim/internal/gateway/auth"
)

type subscriptionStoreFake struct {
	connected bool
	disabled  int64
}

func (s *subscriptionStoreFake) GetPersonalSubscription(_ context.Context, _ int64) (bool, error) {
	return s.connected, nil
}

func (s *subscriptionStoreFake) DisablePersonalSubscription(_ context.Context, maxUserID int64) error {
	s.disabled = maxUserID
	s.connected = false
	return nil
}

func subscriptionRequest(method string) *http.Request {
	req := httptest.NewRequest(method, "/api/v1/max-subscription", nil)
	return req.WithContext(context.WithValue(req.Context(), sessionContextKey{}, auth.Session{UserID: 1, MAXUserID: 42}))
}

func TestPersonalSubscriptionHandlers(t *testing.T) {
	store := &subscriptionStoreFake{}

	connect := httptest.NewRecorder()
	connectMaxSubscription("@delim_bot").ServeHTTP(connect, subscriptionRequest(http.MethodPost))
	if connect.Code != http.StatusOK || connect.Body.String() != "{\"connected\":false,\"bot_url\":\"https://max.ru/delim_bot\"}\n" {
		t.Fatalf("connect response = %d %s", connect.Code, connect.Body.String())
	}

	store.connected = true
	get := httptest.NewRecorder()
	getMaxSubscription(store, "delim_bot").ServeHTTP(get, subscriptionRequest(http.MethodGet))
	if get.Code != http.StatusOK || get.Body.String() != "{\"connected\":true,\"bot_url\":\"https://max.ru/delim_bot\"}\n" {
		t.Fatalf("get response = %d %s", get.Code, get.Body.String())
	}

	disable := httptest.NewRecorder()
	disableMaxSubscription(store).ServeHTTP(disable, subscriptionRequest(http.MethodDelete))
	if disable.Code != http.StatusNoContent || store.disabled != 42 {
		t.Fatalf("disable = %d, user = %d", disable.Code, store.disabled)
	}
}
