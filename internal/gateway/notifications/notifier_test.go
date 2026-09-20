package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"delim/internal/gateway/repository/postgres"
)

type notifierStore struct {
	subscriptions []postgres.PersonalSubscription
	lookupErr     error
	enqueueErr    error
	enqueued      []queuedNotification
}

type queuedNotification struct {
	key, kind string
	chatID    int64
	payload   []byte
}

func (s *notifierStore) ListActivePersonalSubscriptions(_ context.Context, _ []int64) ([]postgres.PersonalSubscription, error) {
	return s.subscriptions, s.lookupErr
}

func (s *notifierStore) EnqueueNotification(_ context.Context, key, kind string, chatID int64, payload []byte) (bool, error) {
	s.enqueued = append(s.enqueued, queuedNotification{key: key, kind: kind, chatID: chatID, payload: payload})
	return true, s.enqueueErr
}

func TestNotifyPersonalQueuesEveryActivePrivateChat(t *testing.T) {
	store := &notifierStore{subscriptions: []postgres.PersonalSubscription{
		{MAXUserID: 101, ChatID: 1001}, {MAXUserID: 202, ChatID: 2002},
	}}
	notifier := NewNotifier(store, nil)
	if err := notifier.NotifyPersonal(context.Background(), []int64{101, 202, 303}, "expense_created", "expense-create:9", Payload{Text: "message"}); err != nil {
		t.Fatalf("NotifyPersonal returned error: %v", err)
	}
	if len(store.enqueued) != 2 {
		t.Fatalf("queued %d notifications, want 2", len(store.enqueued))
	}
	if store.enqueued[0].chatID != 1001 || store.enqueued[1].chatID != 2002 {
		t.Fatalf("queued non-personal chats: %#v", store.enqueued)
	}
	if store.enqueued[0].key != "expense-create:9:101" || store.enqueued[1].key != "expense-create:9:202" {
		t.Fatalf("recipient dedupe keys = %#v", store.enqueued)
	}
	var payload Payload
	if err := json.Unmarshal(store.enqueued[0].payload, &payload); err != nil || payload.Text != "message" {
		t.Fatalf("payload = %#v, err = %v", payload, err)
	}
}

func TestNotifyPersonalNeverBreaksBusinessOperation(t *testing.T) {
	for _, store := range []*notifierStore{
		{lookupErr: errors.New("db unavailable")},
		{subscriptions: []postgres.PersonalSubscription{{MAXUserID: 101, ChatID: 1001}}, enqueueErr: errors.New("outbox unavailable")},
	} {
		if err := NewNotifier(store, nil).NotifyPersonal(context.Background(), []int64{101}, "expense_created", "key", Payload{Text: "message"}); err != nil {
			t.Fatalf("NotifyPersonal returned error: %v", err)
		}
	}
}
