package core

import (
	"context"
	"fmt"
	"time"

	corev1 "delim/pkg/gen/core/v1"
	"delim/pkg/grpcx"
	"delim/pkg/metricsx"
	"google.golang.org/grpc"
)

const (
	defaultTimeout = 10 * time.Second
	clientPeer     = "core"
)

type Client struct {
	conn     *grpc.ClientConn
	client   corev1.CoreServiceClient
	recorder *metricsx.Recorder
}

func New(address string, recorder *metricsx.Recorder) (*Client, error) {
	conn, err := grpcx.NewClient(address,
		grpc.WithChainUnaryInterceptor(deadlineUnaryInterceptor, clientMetricsInterceptor(recorder)),
	)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, client: corev1.NewCoreServiceClient(conn), recorder: recorder}, nil
}

func deadlineUnaryInterceptor(ctx context.Context, method string, req, reply any, conn *grpc.ClientConn, invoker grpc.UnaryInvoker, options ...grpc.CallOption) error {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	return invoker(callCtx, method, req, reply, conn, options...)
}

func clientMetricsInterceptor(recorder *metricsx.Recorder) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, conn *grpc.ClientConn, invoker grpc.UnaryInvoker, options ...grpc.CallOption) error {
		if recorder == nil {
			return invoker(ctx, method, req, reply, conn, options...)
		}
		err := invoker(ctx, method, req, reply, conn, options...)
		if err != nil {
			recorder.ObserveGRPCClientError(clientPeer, method, err)
		}
		return err
	}
}

func withDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, defaultTimeout)
}

func (c *Client) Ping(ctx context.Context) error {
	response, err := c.client.Ping(ctx, &corev1.PingRequest{}, grpc.WaitForReady(true))
	if err != nil {
		return fmt.Errorf("core ping: %w", err)
	}
	if response.GetStatus() != "ok" {
		return fmt.Errorf("core ping: unexpected status %q", response.GetStatus())
	}
	return nil
}

func (c *Client) UpsertUser(ctx context.Context, req *corev1.UpsertUserRequest) (*corev1.UpsertUserResponse, error) {
	return c.client.UpsertUser(ctx, req)
}

func (c *Client) GetUser(ctx context.Context, req *corev1.GetUserRequest) (*corev1.GetUserResponse, error) {
	return c.client.GetUser(ctx, req)
}

func (c *Client) CreateGroup(ctx context.Context, req *corev1.CreateGroupRequest) (*corev1.CreateGroupResponse, error) {
	return c.client.CreateGroup(ctx, req)
}

func (c *Client) GetGroup(ctx context.Context, req *corev1.GetGroupRequest) (*corev1.GetGroupResponse, error) {
	return c.client.GetGroup(ctx, req)
}

func (c *Client) ListGroups(ctx context.Context, req *corev1.ListGroupsRequest) (*corev1.ListGroupsResponse, error) {
	return c.client.ListGroups(ctx, req)
}

func (c *Client) JoinGroup(ctx context.Context, req *corev1.JoinGroupRequest) (*corev1.JoinGroupResponse, error) {
	return c.client.JoinGroup(ctx, req)
}

func (c *Client) ListGroupMembers(ctx context.Context, req *corev1.ListGroupMembersRequest) (*corev1.ListGroupMembersResponse, error) {
	return c.client.ListGroupMembers(ctx, req)
}

func (c *Client) AddGroupMembers(ctx context.Context, req *corev1.AddGroupMembersRequest) (*corev1.AddGroupMembersResponse, error) {
	return c.client.AddGroupMembers(ctx, req)
}

func (c *Client) UpdateMemberRole(ctx context.Context, req *corev1.UpdateMemberRoleRequest) (*corev1.UpdateMemberRoleResponse, error) {
	return c.client.UpdateMemberRole(ctx, req)
}

func (c *Client) ArchiveGroup(ctx context.Context, req *corev1.ArchiveGroupRequest) (*corev1.ArchiveGroupResponse, error) {
	return c.client.ArchiveGroup(ctx, req)
}

func (c *Client) CreateExpense(ctx context.Context, req *corev1.CreateExpenseRequest) (*corev1.CreateExpenseResponse, error) {
	return c.client.CreateExpense(ctx, req)
}

func (c *Client) GetExpense(ctx context.Context, req *corev1.GetExpenseRequest) (*corev1.GetExpenseResponse, error) {
	return c.client.GetExpense(ctx, req)
}

func (c *Client) ListExpenses(ctx context.Context, req *corev1.ListExpensesRequest) (*corev1.ListExpensesResponse, error) {
	return c.client.ListExpenses(ctx, req)
}

func (c *Client) UpdateExpense(ctx context.Context, req *corev1.UpdateExpenseRequest) (*corev1.UpdateExpenseResponse, error) {
	return c.client.UpdateExpense(ctx, req)
}

func (c *Client) ConfirmExpense(ctx context.Context, req *corev1.ConfirmExpenseRequest) (*corev1.ConfirmExpenseResponse, error) {
	return c.client.ConfirmExpense(ctx, req)
}

func (c *Client) CancelExpense(ctx context.Context, req *corev1.CancelExpenseRequest) (*corev1.CancelExpenseResponse, error) {
	return c.client.CancelExpense(ctx, req)
}

func (c *Client) GetBalance(ctx context.Context, req *corev1.GetBalanceRequest) (*corev1.GetBalanceResponse, error) {
	return c.client.GetBalance(ctx, req)
}

func (c *Client) GetBalanceBreakdown(ctx context.Context, req *corev1.GetBalanceBreakdownRequest) (*corev1.GetBalanceBreakdownResponse, error) {
	return c.client.GetBalanceBreakdown(ctx, req)
}

func (c *Client) GetSettlementPlan(ctx context.Context, req *corev1.GetSettlementPlanRequest) (*corev1.GetSettlementPlanResponse, error) {
	return c.client.GetSettlementPlan(ctx, req)
}

func (c *Client) CreateSettlement(ctx context.Context, req *corev1.CreateSettlementRequest) (*corev1.CreateSettlementResponse, error) {
	return c.client.CreateSettlement(ctx, req)
}

func (c *Client) ConfirmSettlement(ctx context.Context, req *corev1.ConfirmSettlementRequest) (*corev1.ConfirmSettlementResponse, error) {
	return c.client.ConfirmSettlement(ctx, req)
}

func (c *Client) ListSettlements(ctx context.Context, req *corev1.ListSettlementsRequest) (*corev1.ListSettlementsResponse, error) {
	return c.client.ListSettlements(ctx, req)
}

func (c *Client) CreateAdjustment(ctx context.Context, req *corev1.CreateAdjustmentRequest) (*corev1.CreateAdjustmentResponse, error) {
	return c.client.CreateAdjustment(ctx, req)
}

func (c *Client) ListAdjustments(ctx context.Context, req *corev1.ListAdjustmentsRequest) (*corev1.ListAdjustmentsResponse, error) {
	return c.client.ListAdjustments(ctx, req)
}

func (c *Client) ListGroupAdjustments(ctx context.Context, req *corev1.ListGroupAdjustmentsRequest) (*corev1.ListAdjustmentsResponse, error) {
	return c.client.ListGroupAdjustments(ctx, req)
}

func (c *Client) Close() error {
	return c.conn.Close()
}
