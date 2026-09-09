package maxupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"delim/internal/gateway/callback"
	"delim/internal/gateway/launch"
	postgresrepo "delim/internal/gateway/repository/postgres"
	corev1 "delim/pkg/gen/core/v1"
	"delim/pkg/maxapi"
)

const testSecret = "dispatcher-test-secret-000000000000"

// fakeStore implements the Dispatcher storage surface in memory.
type fakeStore struct {
	mu          sync.Mutex
	groupByChat map[int64]int64
	chatByGroup map[int64]postgresrepo.ChatGroupBinding
}

func newFakeStore() *fakeStore {
	return &fakeStore{groupByChat: map[int64]int64{}, chatByGroup: map[int64]postgresrepo.ChatGroupBinding{}}
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

// newTestDispatcher builds a dispatcher wired to an httptest MAX API server.
func newTestDispatcher(t *testing.T, store *fakeStore, core *fakeCore) (*Dispatcher, *maxapi.Client, *[]maxapi.NewMessage) {
	t.Helper()
	var messages []maxapi.NewMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/me" && r.Method == http.MethodGet:
			_, _ = io.WriteString(w, `{"user_id":42,"first_name":"delim","is_bot":true}`)
		case r.URL.Path == "/messages" && r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			var message maxapi.NewMessage
			_ = json.Unmarshal(body, &message)
			messages = append(messages, message)
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
	launches := launch.NewManager(testSecret, time.Hour)
	callbacks := callback.NewManager(testSecret, time.Hour)
	dispatcher := NewDispatcher(store, maxClient, core, launches, callbacks, slog.New(slog.DiscardHandler), nil, "https://app.example")
	return dispatcher, maxClient, &messages
}

func message(text string) Update {
	return Update{UpdateType: MessageCreated, ChatID: 111, Message: &Message{Sender: &User{UserID: 9001, FirstName: "Алексей"}, Body: MessageBody{Text: text}}}
}

func TestDispatcherStartAndHelp(t *testing.T) {
	t.Parallel()
	dispatcher, _, messages := newTestDispatcher(t, newFakeStore(), newFakeCore(1))
	if err := dispatcher.Dispatch(context.Background(), message("/start")); err != nil {
		t.Fatalf("/start: %v", err)
	}
	if err := dispatcher.Dispatch(context.Background(), message("/help")); err != nil {
		t.Fatalf("/help: %v", err)
	}
	if len(*messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(*messages))
	}
}

func TestNewBoundIssuesLaunchToken(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.bind(111, 5, 3)
	dispatcher, _, messages := newTestDispatcher(t, store, newFakeCore(5))
	if err := dispatcher.Dispatch(context.Background(), message("/new")); err != nil {
		t.Fatalf("/new bound: %v", err)
	}
	msg := (*messages)[0]
	buttons := msg.Attachments[0].Payload.Buttons
	if len(buttons) == 0 || buttons[0][0].Type != "open_app" {
		t.Fatalf("expected open_app button, got %+v", buttons)
	}
	if !strings.Contains(buttons[0][0].URL, "startapp=dl_") {
		t.Fatalf("expected launch token in URL, got %q", buttons[0][0].URL)
	}
}

func TestNewUnboundPrompts(t *testing.T) {
	t.Parallel()
	dispatcher, _, messages := newTestDispatcher(t, newFakeStore(), newFakeCore(1))
	if err := dispatcher.Dispatch(context.Background(), message("/new")); err != nil {
		t.Fatalf("/new unbound: %v", err)
	}
	msg := (*messages)[0]
	if !strings.Contains(msg.Text, "Привяжите этот чат") {
		t.Fatalf("expected bind prompt, got %q", msg.Text)
	}
}

func TestBalanceCommand(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.bind(111, 7, 3)
	core := newFakeCore(7)
	core.netByUser[1] = 124000 // "Тебе должны 1 240 ₽"
	dispatcher, _, messages := newTestDispatcher(t, store, core)
	if err := dispatcher.Dispatch(context.Background(), message("/balance")); err != nil {
		t.Fatalf("/balance: %v", err)
	}
	msg := (*messages)[0]
	if !strings.Contains(msg.Text, "Тебе должны") || !strings.Contains(msg.Text, "1 240") {
		t.Fatalf("unexpected balance text %q", msg.Text)
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
	if len(core.added[9]) != 1 {
		t.Fatalf("expected 1 member added, got %v", core.added[9])
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
