package membersync

import (
	"context"
	"errors"
	"fmt"

	postgresrepo "delim/internal/gateway/repository/postgres"
	"delim/pkg/maxapi"
)

var (
	ErrChatInactive          = errors.New("MAX chat is not active")
	ErrMemberSyncUnavailable = errors.New("member_sync_unavailable")
)

type UnavailableError struct {
	ChatID int64
	Cause  error
}

func (e *UnavailableError) Error() string {
	return fmt.Sprintf("%s for chat %d", ErrMemberSyncUnavailable, e.ChatID)
}

func (e *UnavailableError) Unwrap() error {
	return ErrMemberSyncUnavailable
}

type User struct {
	UserID    int64
	FirstName string
	LastName  string
	Username  string
	AvatarURL string
}

type Service struct {
	store  *postgresrepo.Store
	maxAPI *maxapi.Client
}

func New(store *postgresrepo.Store, maxAPI *maxapi.Client) *Service {
	return &Service{store: store, maxAPI: maxAPI}
}

func (s *Service) GetMembers(ctx context.Context, chatID int64) ([]User, error) {
	active, err := s.store.IsChatActive(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, ErrChatInactive
	}
	membership, err := s.maxAPI.GetBotMembership(ctx, chatID)
	if err != nil {
		return nil, memberError(chatID, err)
	}
	if !membership.IsAdmin {
		return nil, &UnavailableError{ChatID: chatID, Cause: maxapi.ErrInsufficientPermissions}
	}
	members, err := s.maxAPI.GetAllChatMembers(ctx, chatID)
	if err != nil {
		return nil, memberError(chatID, err)
	}

	users := make([]User, 0, len(members))
	for _, member := range members {
		if member.IsBot {
			continue
		}
		user := User{UserID: member.UserID, FirstName: member.FirstName, AvatarURL: member.AvatarURL}
		if user.FirstName == "" && member.Name != nil {
			user.FirstName = *member.Name
		}
		if member.LastName != nil {
			user.LastName = *member.LastName
		}
		if member.Username != nil {
			user.Username = *member.Username
		}
		if user.AvatarURL == "" {
			user.AvatarURL = member.FullAvatarURL
		}
		users = append(users, user)
	}
	return users, nil
}

func memberError(chatID int64, err error) error {
	if errors.Is(err, maxapi.ErrInsufficientPermissions) {
		return &UnavailableError{ChatID: chatID, Cause: err}
	}
	return err
}
