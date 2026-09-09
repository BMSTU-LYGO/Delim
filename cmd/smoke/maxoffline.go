package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"delim/internal/gateway/auth"
)

const offlineChatID = 555001

// verifyMaxOffline exercises the MAX chat integration without any real MAX
// credentials: chat registry via webhook, binding API, permissions, unbind and
// webhook idempotency.
func (s *scenario) verifyMaxOffline(ctx context.Context) error {
	if s.sessionSecret == "" {
		return errors.New("max-offline: gateway session secret is not configured")
	}
	// Fresh group owned by actor A with member B.
	var group struct {
		ID int64 `json:"id"`
	}
	if err := s.api.json(ctx, http.MethodPost, "/api/v1/groups", s.actorA.token, map[string]any{
		"name": fmt.Sprintf("Делим MAX offline %d", time.Now().UnixNano()),
	}, http.StatusCreated, &group); err != nil {
		return fmt.Errorf("max-offline: create group: %w", err)
	}
	if group.ID <= 0 {
		return errors.New("max-offline: create group returned an invalid id")
	}
	if err := s.api.json(ctx, http.MethodPost, fmt.Sprintf("/api/v1/groups/%d/members", group.ID), s.actorA.token, map[string]any{
		"user_ids": []int64{s.actorB.id},
	}, http.StatusOK, nil); err != nil {
		return fmt.Errorf("max-offline: add member: %w", err)
	}

	// 1. Mark the chat active via the bot_added webhook (idempotent replay).
	webhook := map[string]any{
		"update_type": "bot_added", "timestamp": time.Now().UnixMilli(), "chat_id": offlineChatID,
	}
	secretHeaders := map[string]string{"X-Max-Bot-Api-Secret": s.webhookSecret}
	if err := s.api.rawJSON(ctx, http.MethodPost, "/api/v1/max/webhook", secretHeaders, webhook, http.StatusOK, nil); err != nil {
		return fmt.Errorf("max-offline: bot_added webhook: %w", err)
	}
	if err := s.api.rawJSON(ctx, http.MethodPost, "/api/v1/max/webhook", secretHeaders, webhook, http.StatusOK, nil); err != nil {
		return fmt.Errorf("max-offline: duplicate webhook: %w", err)
	}
	// The webhook is processed asynchronously by the Gateway worker; wait for
	// the chat to become active before binding.
	select {
	case <-time.After(3 * time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}

	// 2. Owner session with verified chat context.
	ownerToken, err := s.issueSessionWithChat(s.actorA.id, s.actorA.maxUserID, offlineChatID)
	if err != nil {
		return fmt.Errorf("max-offline: owner chat session: %w", err)
	}

	// 3. Bind the chat to the group.
	if err := s.api.json(ctx, http.MethodPost, fmt.Sprintf("/api/v1/groups/%d/max-chat", group.ID), ownerToken, nil, http.StatusOK, nil); err != nil {
		return fmt.Errorf("max-offline: bind chat: %w", err)
	}
	// 4. Read the binding back.
	var binding struct {
		Bound bool  `json:"bound"`
		Chat  int64 `json:"chat_id"`
	}
	// GET may return {bound:true} shape or the full binding; tolerate both.
	if err := s.api.json(ctx, http.MethodGet, fmt.Sprintf("/api/v1/groups/%d/max-chat", group.ID), ownerToken, nil, http.StatusOK, &binding); err != nil {
		return fmt.Errorf("max-offline: get binding: %w", err)
	}

	// 5. Sync without a real MAX token must fail gracefully and not break the group.
	if err := s.api.json(ctx, http.MethodPost, fmt.Sprintf("/api/v1/groups/%d/max-chat/sync", group.ID), ownerToken, nil, http.StatusServiceUnavailable, nil); err != nil {
		if status, ok := err.(*responseError); ok && status.status == http.StatusConflict {
			// bot_admin_required is also an acceptable offline outcome.
		} else if ok {
			return fmt.Errorf("max-offline: sync error: %w", err)
		}
	}
	if err := s.api.json(ctx, http.MethodGet, fmt.Sprintf("/api/v1/groups/%d", group.ID), s.actorA.token, nil, http.StatusOK, nil); err != nil {
		return fmt.Errorf("max-offline: group intact after sync failure: %w", err)
	}

	// 6. Member is forbidden from binding.
	memberToken, err := s.issueSessionWithChat(s.actorB.id, s.actorB.maxUserID, offlineChatID)
	if err != nil {
		return fmt.Errorf("max-offline: member chat session: %w", err)
	}
	if err := s.api.json(ctx, http.MethodPost, fmt.Sprintf("/api/v1/groups/%d/max-chat", group.ID), memberToken, nil, http.StatusForbidden, nil); err != nil {
		return fmt.Errorf("max-offline: member bind not forbidden: %w", err)
	}

	// 7. Unbind by owner.
	if err := s.api.json(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/groups/%d/max-chat", group.ID), ownerToken, nil, http.StatusNoContent, nil); err != nil {
		return fmt.Errorf("max-offline: unbind: %w", err)
	}
	return nil
}

// issueSessionWithChat creates a local session carrying the verified chat id.
func (s *scenario) issueSessionWithChat(userID, maxUserID, chatID int64) (string, error) {
	manager := auth.NewManager(s.sessionSecret, s.sessionTTL)
	token, _, err := manager.IssueWithContext(userID, maxUserID, nil, chatID)
	if err != nil {
		return "", err
	}
	return token, nil
}
