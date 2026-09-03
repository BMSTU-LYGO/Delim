package maxapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

var ErrInsufficientPermissions = errors.New("MAX bot has insufficient chat permissions")

type PermissionError struct {
	ChatID int64
	Cause  *APIError
}

func (e *PermissionError) Error() string {
	return fmt.Sprintf("%s for chat %d", ErrInsufficientPermissions, e.ChatID)
}

func (e *PermissionError) Unwrap() error {
	return ErrInsufficientPermissions
}

type Chat struct {
	ChatID            int64   `json:"chat_id"`
	Type              string  `json:"type"`
	Status            string  `json:"status"`
	Title             *string `json:"title"`
	LastEventTime     int64   `json:"last_event_time"`
	ParticipantsCount int32   `json:"participants_count"`
	OwnerID           *int64  `json:"owner_id,omitempty"`
	IsPublic          bool    `json:"is_public"`
	Link              *string `json:"link,omitempty"`
	Description       *string `json:"description"`
}

type ChatMember struct {
	UserID        int64    `json:"user_id"`
	FirstName     string   `json:"first_name"`
	LastName      *string  `json:"last_name,omitempty"`
	Username      *string  `json:"username"`
	IsBot         bool     `json:"is_bot"`
	Name          *string  `json:"name,omitempty"`
	AvatarURL     string   `json:"avatar_url,omitempty"`
	FullAvatarURL string   `json:"full_avatar_url,omitempty"`
	IsOwner       bool     `json:"is_owner"`
	IsAdmin       bool     `json:"is_admin"`
	Permissions   []string `json:"permissions,omitempty"`
}

type ChatMembersPage struct {
	Members []ChatMember `json:"members"`
	Marker  *int64       `json:"marker"`
}

func (c *Client) GetChat(ctx context.Context, chatID int64) (Chat, error) {
	var chat Chat
	if err := c.do(ctx, http.MethodGet, chatPath(chatID), nil, nil, &chat, true); err != nil {
		return Chat{}, err
	}
	return chat, nil
}

func (c *Client) GetBotMembership(ctx context.Context, chatID int64) (ChatMember, error) {
	var member ChatMember
	err := c.do(ctx, http.MethodGet, chatPath(chatID)+"/members/me", nil, nil, &member, true)
	if err != nil {
		return ChatMember{}, memberPermissionError(chatID, err)
	}
	return member, nil
}

func (c *Client) GetChatMembers(ctx context.Context, chatID int64, marker *int64) (ChatMembersPage, error) {
	query := url.Values{"count": []string{"100"}}
	if marker != nil {
		query.Set("marker", strconv.FormatInt(*marker, 10))
	}
	var page ChatMembersPage
	err := c.do(ctx, http.MethodGet, chatPath(chatID)+"/members", query, nil, &page, true)
	if err != nil {
		return ChatMembersPage{}, memberPermissionError(chatID, err)
	}
	return page, nil
}

func (c *Client) GetAllChatMembers(ctx context.Context, chatID int64) ([]ChatMember, error) {
	var members []ChatMember
	var marker *int64
	seenMarkers := make(map[int64]struct{})
	for {
		page, err := c.GetChatMembers(ctx, chatID, marker)
		if err != nil {
			return nil, err
		}
		members = append(members, page.Members...)
		if page.Marker == nil {
			return members, nil
		}
		if _, seen := seenMarkers[*page.Marker]; seen {
			return nil, errors.New("MAX API returned a repeated members marker")
		}
		seenMarkers[*page.Marker] = struct{}{}
		marker = page.Marker
	}
}

func chatPath(chatID int64) string {
	return "/chats/" + strconv.FormatInt(chatID, 10)
}

func memberPermissionError(chatID int64, err error) error {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
		return &PermissionError{ChatID: chatID, Cause: apiErr}
	}
	return err
}
