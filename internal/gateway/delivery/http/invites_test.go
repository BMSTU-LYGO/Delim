package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"delim/internal/gateway/auth"
	"delim/internal/gateway/invite"
	"delim/internal/gateway/launch"
	corev1 "delim/pkg/gen/core/v1"
	"delim/pkg/maxauth"
	"github.com/go-chi/chi/v5"
)

type inviteFlowCore struct {
	joinAttempts int
	members      map[int64]struct{}
}

func (*inviteFlowCore) Ping(context.Context) error { return nil }
func (*inviteFlowCore) GetUser(context.Context, *corev1.GetUserRequest) (*corev1.GetUserResponse, error) {
	return nil, nil
}
func (*inviteFlowCore) UpsertUser(context.Context, *corev1.UpsertUserRequest) (*corev1.UpsertUserResponse, error) {
	return &corev1.UpsertUserResponse{User: &corev1.User{Id: 17}}, nil
}
func (c *inviteFlowCore) GetGroup(_ context.Context, request *corev1.GetGroupRequest) (*corev1.GetGroupResponse, error) {
	role := corev1.MemberRole_MEMBER_ROLE_MEMBER
	if request.GetActorUserId() == 1 && request.GetGroupId() == 42 {
		role = corev1.MemberRole_MEMBER_ROLE_OWNER
	}
	return &corev1.GetGroupResponse{Group: &corev1.Group{Id: request.GetGroupId(), CurrentUserRole: role}}, nil
}
func (c *inviteFlowCore) JoinGroup(_ context.Context, request *corev1.JoinGroupRequest) (*corev1.JoinGroupResponse, error) {
	if request.GetActorUserId() != 17 || request.GetGroupId() != 42 {
		return nil, context.Canceled
	}
	c.joinAttempts++
	if c.members == nil {
		c.members = make(map[int64]struct{})
	}
	c.members[request.GetActorUserId()] = struct{}{}
	return &corev1.JoinGroupResponse{}, nil
}

func TestTwoUsersCreateInviteShareStartAppAuthAndJoinGroup(t *testing.T) {
	const botToken = "bot-token"
	invites := invite.NewManager("invite-secret")
	core := &inviteFlowCore{}
	createRequest := httptest.NewRequest(http.MethodPost, "/groups/42/invite", nil)
	createRequest = createRequest.WithContext(context.WithValue(createRequest.Context(), sessionContextKey{}, auth.Session{UserID: 1}))
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("groupID", "42")
	createRequest = createRequest.WithContext(context.WithValue(createRequest.Context(), chi.RouteCtxKey, routeContext))
	created := httptest.NewRecorder()
	createGroupInvite(core, invites, time.Hour, "@delim_bot").ServeHTTP(created, createRequest)
	if created.Code != http.StatusCreated {
		t.Fatalf("create invite status=%d body=%s", created.Code, created.Body.String())
	}
	var createdInvite inviteResponse
	if err := json.NewDecoder(created.Body).Decode(&createdInvite); err != nil {
		t.Fatal(err)
	}
	deepLink, err := url.Parse(createdInvite.DeepLink)
	if err != nil {
		t.Fatal(err)
	}
	if deepLink.Scheme != "https" || deepLink.Host != "max.ru" || deepLink.Path != "/delim_bot" || deepLink.Query().Get("startapp") != createdInvite.StartParam {
		t.Fatalf("invalid MAX deep link %q", createdInvite.DeepLink)
	}
	verified, err := invites.Verify(createdInvite.StartParam)
	if err != nil || verified.GroupID != 42 || !verified.ExpiresAt.Equal(createdInvite.ExpiresAt) {
		t.Fatalf("issued invite verification: invite=%#v err=%v", verified, err)
	}

	// User A created and shared the link above. User B opens it in a separate
	// MAX account; membership is granted by the invite during MAX auth, without
	// a MAX group-chat id or bot membership being involved.
	sessions := auth.NewManager("session-secret", time.Hour)
	login := maxLogin(maxauth.NewInitDataVerifier(botToken, time.Hour), sessions, invites, launch.NewManager("launch-secret", time.Hour), core, core)
	response := loginRequest(t, login, signedInitDataForUser(t, botToken, createdInvite.StartParam, 202, "Boris", "E2E", "boris_e2e"))
	if response.Invite == nil || response.Invite.Status != "joined" || response.Invite.GroupID != 42 {
		t.Fatalf("login invite response = %#v", response.Invite)
	}
	memberSession, err := sessions.Verify(response.Token)
	if err != nil || memberSession.MAXUserID != 202 || memberSession.UserID != 17 {
		t.Fatalf("member session = %#v, err=%v", memberSession, err)
	}

	// Re-opening the shared link is idempotent for the same second account.
	reopened := loginRequest(t, login, signedInitDataForUser(t, botToken, createdInvite.StartParam, 202, "Boris", "E2E", "boris_e2e"))
	if reopened.Invite == nil || reopened.Invite.Status != "joined" || reopened.Invite.GroupID != 42 {
		t.Fatalf("reopened invite response = %#v", reopened.Invite)
	}
	if core.joinAttempts != 2 || len(core.members) != 1 {
		t.Fatalf("join attempts=%d unique members=%d", core.joinAttempts, len(core.members))
	}
}

func TestMaxDeepLinkNormalizesUsernameAndEncodesStartParam(t *testing.T) {
	link := maxDeepLink(" @@delim_bot ", "di_value+with&reserved")
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Path != "/delim_bot" || parsed.Query().Get("startapp") != "di_value+with&reserved" {
		t.Fatalf("deep link = %q", link)
	}
	if maxDeepLink("   ", "token") != "" {
		t.Fatal("empty username must not produce a MAX link")
	}
	if !strings.Contains(link, "startapp=di_value%2Bwith%26reserved") {
		t.Fatalf("startapp is not URL-encoded: %q", link)
	}
}
