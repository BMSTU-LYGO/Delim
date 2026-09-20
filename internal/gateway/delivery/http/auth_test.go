package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"delim/internal/gateway/auth"
	"delim/internal/gateway/invite"
	"delim/internal/gateway/launch"
	corev1 "delim/pkg/gen/core/v1"
	"delim/pkg/maxauth"
)

type loginTestCore struct {
	joinCalls int
}

func (*loginTestCore) Ping(context.Context) error { return nil }
func (*loginTestCore) GetGroup(context.Context, *corev1.GetGroupRequest) (*corev1.GetGroupResponse, error) {
	return nil, nil
}
func (*loginTestCore) GetUser(context.Context, *corev1.GetUserRequest) (*corev1.GetUserResponse, error) {
	return nil, nil
}
func (*loginTestCore) UpsertUser(context.Context, *corev1.UpsertUserRequest) (*corev1.UpsertUserResponse, error) {
	return &corev1.UpsertUserResponse{User: &corev1.User{Id: 17}}, nil
}
func (c *loginTestCore) JoinGroup(_ context.Context, request *corev1.JoinGroupRequest) (*corev1.JoinGroupResponse, error) {
	if request.GetActorUserId() != 17 || request.GetGroupId() != 42 {
		return nil, context.Canceled
	}
	c.joinCalls++
	return &corev1.JoinGroupResponse{}, nil
}

func signedInitData(t *testing.T, botToken, startParam string) string {
	t.Helper()
	params := url.Values{
		"auth_date":   {""},
		"query_id":    {"query"},
		"start_param": {startParam},
		"user":        {`{"id":101,"first_name":"Max","last_name":"User","username":"max"}`},
	}
	params.Set("auth_date", strconv.FormatInt(time.Now().Unix(), 10))
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+params.Get(key))
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secret.Write([]byte(botToken))
	signature := hmac.New(sha256.New, secret.Sum(nil))
	_, _ = signature.Write([]byte(strings.Join(lines, "\n")))
	params.Set("hash", hex.EncodeToString(signature.Sum(nil)))
	return params.Encode()
}

func expiredInviteToken(secret string, groupID int64) string {
	payload := make([]byte, 29)
	payload[0] = invite.Version
	binary.BigEndian.PutUint64(payload[1:9], uint64(groupID))
	binary.BigEndian.PutUint64(payload[9:17], uint64(time.Now().Add(-time.Minute).Unix()))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "di_" + base64.RawURLEncoding.EncodeToString(append(payload, mac.Sum(nil)...))
}

func loginRequest(t *testing.T, handler http.Handler, initData string) maxLoginResponse {
	t.Helper()
	body, err := json.Marshal(maxLoginRequest{InitData: initData})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/auth/max", strings.NewReader(string(body))))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response maxLoginResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	return response
}

func TestMAXLoginInviteIsProcessedForEveryAuthenticatedLaunch(t *testing.T) {
	const botToken = "bot-token"
	const inviteSecret = "invite-secret"
	invites := invite.NewManager(inviteSecret)
	token, _, err := invites.Issue(42, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	core := &loginTestCore{}
	handler := maxLogin(maxauth.NewInitDataVerifier(botToken, time.Hour), auth.NewManager("session-secret", time.Hour), invites, launch.NewManager("launch-secret", time.Hour), core, core)

	for range 2 {
		response := loginRequest(t, handler, signedInitData(t, botToken, token))
		if response.Invite == nil || response.Invite.Status != "joined" || response.Invite.GroupID != 42 {
			t.Fatalf("invite response = %#v", response.Invite)
		}
	}
	if core.joinCalls != 2 {
		t.Fatalf("join calls = %d, want 2", core.joinCalls)
	}
}

func TestMAXLoginExpiredInviteReturnsStatus(t *testing.T) {
	const botToken = "bot-token"
	core := &loginTestCore{}
	handler := maxLogin(maxauth.NewInitDataVerifier(botToken, time.Hour), auth.NewManager("session-secret", time.Hour), invite.NewManager("invite-secret"), launch.NewManager("launch-secret", time.Hour), core, core)
	response := loginRequest(t, handler, signedInitData(t, botToken, expiredInviteToken("invite-secret", 42)))
	if response.Invite == nil || response.Invite.Status != "expired" {
		t.Fatalf("invite response = %#v", response.Invite)
	}
	if core.joinCalls != 0 {
		t.Fatalf("expired invite joined %d times", core.joinCalls)
	}
}
