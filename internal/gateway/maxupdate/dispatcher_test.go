package maxupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"delim/internal/gateway/callback"
	postgresrepo "delim/internal/gateway/repository/postgres"
	corev1 "delim/pkg/gen/core/v1"
	"delim/pkg/maxapi"
)

const testSecret = "dispatcher-test-secret-000000000000"

// fakeStore implements the Dispatcher storage surface in memory.
type fakeStore struct {
	mu            sync.Mutex
	subscriptions map[int64]int64
	groupByChat   map[int64]int64
	chatByGroup   map[int64]postgresrepo.ChatGroupBinding
}

func newFakeStore() *fakeStore {
	return &fakeStore{subscriptions: map[int64]int64{}, groupByChat: map[int64]int64{}, chatByGroup: map[int64]postgresrepo.ChatGroupBinding{}}
}

func (f *fakeStore) UpsertPersonalSubscription(_ context.Context, maxUserID, chatID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subscriptions[maxUserID] = chatID
	return nil
}

func (f *fakeStore) GetGroupByChat(_ context.Context, chatID int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	groupID, ok := f.groupByChat[chatID]
	if !ok {
		return 0, postgresrepo.ErrBindingNotFound
	}
	return groupID, nil
}

func (f *fakeStore) GetChatByGroup(_ context.Context, groupID int64) (postgresrepo.ChatGroupBinding, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	binding, ok := f.chatByGroup[groupID]
	if !ok {
		return postgresrepo.ChatGroupBinding{}, postgresrepo.ErrBindingNotFound
	}
	return binding, nil
}

func (f *fakeStore) UpsertChat(context.Context, int64, bool, string, time.Time) error { return nil }

func (f *fakeStore) bind(chatID, groupID, boundBy int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.groupByChat[chatID] = groupID
	f.chatByGroup[groupID] = postgresrepo.ChatGroupBinding{ChatID: chatID, GroupID: groupID, BoundByUserID: boundBy, Status: "active"}
}

// fakeCore models Core for the bot flows: users by MAX id, balances, groups,
// and settlement receivers (permission check).
type fakeCore struct {
	mu           sync.Mutex
	nextID       int64
	userByMax    map[int64]int64
	netByUser    map[int64]int64
	groupID      int64
	receiverByID map[int64]int64 // settlement id -> receiver core id
	added        map[int64][]int64
}

func newFakeCore(groupID int64) *fakeCore {
	return &fakeCore{nextID: 1, userByMax: map[int64]int64{}, netByUser: map[int64]int64{}, groupID: groupID, receiverByID: map[int64]int64{}, added: map[int64][]int64{}}
}

func (c *fakeCore) UpsertUser(_ context.Context, req *corev1.UpsertUserRequest) (*corev1.UpsertUserResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id, ok := c.userByMax[req.MaxUserId]
	if !ok {
		id = c.nextID
		c.nextID++
		c.userByMax[req.MaxUserId] = id
	}
	return &corev1.UpsertUserResponse{User: &corev1.User{Id: id}}, nil
}

func (c *fakeCore) GetBalance(_ context.Context, req *corev1.GetBalanceRequest) (*corev1.GetBalanceResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	net := c.netByUser[req.ActorUserId]
	return &corev1.GetBalanceResponse{Balances: []*corev1.Balance{{UserId: req.ActorUserId, Currency: "RUB", NetAmountMinor: net}}}, nil
}

func (c *fakeCore) AddGroupMembers(_ context.Context, req *corev1.AddGroupMembersRequest) (*corev1.AddGroupMembersResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	added := []*corev1.GroupMember{}
	for _, id := range req.UserIds {
		if !contains(c.added[req.GroupId], id) {
			c.added[req.GroupId] = append(c.added[req.GroupId], id)
			added = append(added, &corev1.GroupMember{UserId: id, GroupId: req.GroupId, Role: corev1.MemberRole_MEMBER_ROLE_MEMBER})
		}
	}
	return &corev1.AddGroupMembersResponse{Members: added}, nil
}

func (c *fakeCore) ConfirmSettlement(_ context.Context, req *corev1.ConfirmSettlementRequest) (*corev1.ConfirmSettlementResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.receiverByID[req.SettlementId] != req.ActorUserId {
		return nil, fmt.Errorf("forbidden: not the receiver")
	}
	return &corev1.ConfirmSettlementResponse{Settlement: &corev1.Settlement{Id: req.SettlementId}}, nil
}

func contains(list []int64, value int64) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

type sentMessage struct {
	message maxapi.NewMessage
	query   url.Values
}

// newTestDispatcher builds a dispatcher wired to an httptest MAX API server.
func newTestDispatcher(t *testing.T, store *fakeStore, core *fakeCore) (*Dispatcher, *maxapi.Client, *[]sentMessage) {
	t.Helper()
	var messages []sentMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/me" && r.Method == http.MethodGet:
			_, _ = io.WriteString(w, `{"user_id":42,"first_name":"delim","is_bot":true}`)
		case r.URL.Path == "/messages" && r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			var message maxapi.NewMessage
			_ = json.Unmarshal(body, &message)
			messages = append(messages, sentMessage{message: message, query: r.URL.Query()})
			if r.URL.Query().Get("user_id") == "999" {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = io.WriteString(w, `{"message":"delivery failed"}`)
				return
			}
			_, _ = io.WriteString(w, `{"message":{"mid":"m1","timestamp":1,"body":{"text":""}}}`)
		case r.URL.Path == "/answers" && r.Method == http.MethodPost:
			_, _ = io.WriteString(w, `{"success":true}`)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{}`)
		}
	}))
	t.Cleanup(server.Close)
	maxClient := maxapi.New(server.URL, "test-token")
	callbacks := callback.NewManager(testSecret, time.Hour)
	dispatcher := NewDispatcher(store, maxClient, core, callbacks, slog.New(slog.DiscardHandler), nil)
	return dispatcher, maxClient, &messages
}

func message(text string) Update {
	return Update{UpdateType: MessageCreated, Message: &Message{Sender: &User{UserID: 123, FirstName: "Алексей"}, Body: MessageBody{Text: text}}}
}

func TestDispatcherSendsEveryStartToSenderUser(t *testing.T) {
	t.Parallel()
	dispatcher, _, messages := newTestDispatcher(t, newFakeStore(), newFakeCore(1))
	for _, text := range []string{"/start", "/start something", "/start@delim_bot"} {
		if err := dispatcher.Dispatch(context.Background(), message(text)); err != nil {
			t.Fatalf("%q: %v", text, err)
		}
	}
	if len(*messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(*messages))
	}
	for _, sent := range *messages {
		if sent.query.Get("user_id") != "123" || sent.query.Has("chat_id") {
			t.Fatalf("unexpected recipient query: %v", sent.query)
		}
		if sent.message.Text != welcomeText || len(sent.message.Attachments) != 0 {
			t.Fatalf("unexpected welcome: %+v", sent.message)
		}
	}
}

func TestDispatcherIgnoresMessages(t *testing.T) {
	for _, text := range []string{"/help", "/new", "/balance", "обычный текст", "/unknown"} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			dispatcher, _, messages := newTestDispatcher(t, newFakeStore(), newFakeCore(1))
			if err := dispatcher.Dispatch(context.Background(), message(text)); err != nil {
				t.Fatalf("dispatch %q: %v", text, err)
			}
			if len(*messages) != 0 {
				t.Fatalf("message %q produced %d replies", text, len(*messages))
			}
		})
	}
}

func TestDispatcherIgnoresStartWithoutSender(t *testing.T) {
	for _, update := range []Update{
		{UpdateType: MessageCreated, Message: &Message{Body: MessageBody{Text: "/start"}}},
		{UpdateType: MessageCreated, Message: &Message{Sender: &User{}, Body: MessageBody{Text: "/start"}}},
	} {
		dispatcher, _, messages := newTestDispatcher(t, newFakeStore(), newFakeCore(1))
		if err := dispatcher.Dispatch(context.Background(), update); err != nil {
			t.Fatalf("missing sender: %v", err)
		}
		if len(*messages) != 0 {
			t.Fatalf("missing sender produced %d replies", len(*messages))
		}
	}
}

func TestDispatcherReturnsWelcomeDeliveryError(t *testing.T) {
	dispatcher, _, _ := newTestDispatcher(t, newFakeStore(), newFakeCore(1))
	update := message("/start")
	update.Message.Sender.UserID = 999
	if err := dispatcher.Dispatch(context.Background(), update); err == nil {
		t.Fatal("expected MAX API error")
	}
}

func TestUserAddedSyncsIntoBoundGroup(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.bind(111, 9, 3) // bound_by = core 3
	core := newFakeCore(9)
	dispatcher, _, _ := newTestDispatcher(t, store, core)
	update := Update{UpdateType: UserAdded, ChatID: 111, User: &User{UserID: 555, FirstName: "Новичок"}}
	if err := dispatcher.Dispatch(context.Background(), update); err != nil {
		t.Fatalf("user_added: %v", err)
	}
	core.mu.Lock()
	defer core.mu.Unlock()
	if len(core.added[9]) != 0 {
		t.Fatalf("MAX chat member must not be added to a Delim group, got %v", core.added[9])
	}
}

func TestBotStartedOnlyActivatesPersonalSubscription(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	dispatcher, _, messages := newTestDispatcher(t, store, newFakeCore(1))
	update := Update{UpdateType: BotStarted, ChatID: 111, User: &User{UserID: 555, FirstName: "Новичок"}}
	if err := dispatcher.Dispatch(context.Background(), update); err != nil {
		t.Fatalf("bot_started: %v", err)
	}
	if len(*messages) != 0 {
		t.Fatalf("bot_started produced %d welcome messages", len(*messages))
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.subscriptions[555] != 111 {
		t.Fatalf("personal subscription = %d, want 111", store.subscriptions[555])
	}
}

func TestConfirmSettlementCallback(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.bind(111, 9, 3)
	core := newFakeCore(9)
	receiverMAX := int64(777)
	receiverCore := int64(2)
	core.receiverByID[555] = receiverCore
	core.userByMax[receiverMAX] = receiverCore
	dispatcher, _, messages := newTestDispatcher(t, store, core)
	callbacks := callback.NewManager(testSecret, time.Hour)
	token, _, err := callbacks.Issue(callback.ConfirmSettlement, 555)
	if err != nil {
		t.Fatalf("issue callback: %v", err)
	}
	update := Update{
		UpdateType: MessageCallback, ChatID: 111, CallbackID: "cb1",
		Callback: &Callback{CallbackID: "cb1", Payload: token, User: &User{UserID: receiverMAX, FirstName: "Получатель"}},
	}
	if err := dispatcher.Dispatch(context.Background(), update); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if len(*messages) != 0 { // confirm uses AnswerCallback, not SendMessage
		t.Fatalf("unexpected messages %d", len(*messages))
	}
}

func TestForeignAndDuplicateCallbacksAreSafe(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.bind(111, 9, 3)
	core := newFakeCore(9)
	core.receiverByID[555] = 2 // receiver is core 2
	dispatcher, _, _ := newTestDispatcher(t, store, core)
	callbacks := callback.NewManager(testSecret, time.Hour)
	token, _, _ := callbacks.Issue(callback.ConfirmSettlement, 555)
	foreign := Update{UpdateType: MessageCallback, ChatID: 111, CallbackID: "cb2", Callback: &Callback{CallbackID: "cb2", Payload: token, User: &User{UserID: 100, FirstName: "Чужой"}}}
	// Foreign user: Core rejects -> handler answers but returns nil (no crash).
	if err := dispatcher.Dispatch(context.Background(), foreign); err != nil {
		t.Fatalf("foreign callback errored: %v", err)
	}
	// Unsigned/unknown payload must be ignored.
	update := Update{UpdateType: MessageCallback, ChatID: 111, CallbackID: "cb3", Callback: &Callback{CallbackID: "cb3", Payload: "v1:help"}}
	if err := dispatcher.Dispatch(context.Background(), update); err != nil {
		t.Fatalf("plain help callback errored: %v", err)
	}
}
