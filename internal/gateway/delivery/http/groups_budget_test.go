package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"delim/internal/gateway/auth"
	corev1 "delim/pkg/gen/core/v1"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type budgetTestCore struct {
	response         *corev1.GetGroupBudgetSummaryResponse
	err              error
	groupID, actorID int64
}

func (c *budgetTestCore) Ping(context.Context) error { return nil }
func (c *budgetTestCore) UpsertUser(context.Context, *corev1.UpsertUserRequest) (*corev1.UpsertUserResponse, error) {
	return nil, nil
}
func (c *budgetTestCore) GetUser(context.Context, *corev1.GetUserRequest) (*corev1.GetUserResponse, error) {
	return nil, nil
}
func (c *budgetTestCore) CreateGroup(context.Context, *corev1.CreateGroupRequest) (*corev1.CreateGroupResponse, error) {
	return nil, nil
}
func (c *budgetTestCore) GetGroup(context.Context, *corev1.GetGroupRequest) (*corev1.GetGroupResponse, error) {
	return nil, nil
}
func (c *budgetTestCore) ListGroups(context.Context, *corev1.ListGroupsRequest) (*corev1.ListGroupsResponse, error) {
	return nil, nil
}
func (c *budgetTestCore) JoinGroup(context.Context, *corev1.JoinGroupRequest) (*corev1.JoinGroupResponse, error) {
	return nil, nil
}
func (c *budgetTestCore) ListGroupMembers(context.Context, *corev1.ListGroupMembersRequest) (*corev1.ListGroupMembersResponse, error) {
	return nil, nil
}
func (c *budgetTestCore) AddGroupMembers(context.Context, *corev1.AddGroupMembersRequest) (*corev1.AddGroupMembersResponse, error) {
	return nil, nil
}
func (c *budgetTestCore) UpdateMemberRole(context.Context, *corev1.UpdateMemberRoleRequest) (*corev1.UpdateMemberRoleResponse, error) {
	return nil, nil
}
func (c *budgetTestCore) ArchiveGroup(context.Context, *corev1.ArchiveGroupRequest) (*corev1.ArchiveGroupResponse, error) {
	return nil, nil
}
func (c *budgetTestCore) GetGroupBudgetSummary(_ context.Context, request *corev1.GetGroupBudgetSummaryRequest) (*corev1.GetGroupBudgetSummaryResponse, error) {
	c.actorID, c.groupID = request.GetActorUserId(), request.GetGroupId()
	return c.response, c.err
}

func budgetRequest(path string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	return request.WithContext(context.WithValue(request.Context(), sessionContextKey{}, auth.Session{UserID: 17}))
}

func TestGetGroupBudgetSummary(t *testing.T) {
	core := &budgetTestCore{response: &corev1.GetGroupBudgetSummaryResponse{PlannedBudgetMinor: 10000, ConfirmedSpendMinor: 4000, PendingSpendMinor: 1500, TotalSpendMinor: 5500}}
	request := budgetRequest("/groups/42/budget-summary")
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("groupID", "42")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	recorder := httptest.NewRecorder()
	getGroupBudgetSummary(core).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || core.actorID != 17 || core.groupID != 42 {
		t.Fatalf("code=%d actor=%d group=%d", recorder.Code, core.actorID, core.groupID)
	}
	var body groupBudgetSummaryResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ConfirmedSpendMinor != 4000 || body.PendingSpendMinor != 1500 || body.TotalSpendMinor != 5500 {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestGetGroupBudgetSummaryErrors(t *testing.T) {
	t.Run("invalid id", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		getGroupBudgetSummary(&budgetTestCore{}).ServeHTTP(recorder, budgetRequest("/groups/bad/budget-summary"))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("code=%d", recorder.Code)
		}
	})
	t.Run("access denied", func(t *testing.T) {
		core := &budgetTestCore{err: status.Error(codes.PermissionDenied, "not a member")}
		request := budgetRequest("/groups/42/budget-summary")
		routeContext := chi.NewRouteContext()
		routeContext.URLParams.Add("groupID", "42")
		request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
		recorder := httptest.NewRecorder()
		getGroupBudgetSummary(core).ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("code=%d", recorder.Code)
		}
	})
}
