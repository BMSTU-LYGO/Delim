package membersync

import (
	"context"
	"errors"
	"fmt"

	postgresrepo "delim/internal/gateway/repository/postgres"
	corev1 "delim/pkg/gen/core/v1"
	"delim/pkg/maxapi"
)

var (
	ErrChatInactive          = errors.New("MAX chat is not active")
	ErrMemberSyncUnavailable = errors.New("member_sync_unavailable")
	ErrBotAdminRequired      = errors.New("bot_admin_required")
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

// SyncCore is the Core surface the member sync needs. Core stays the source of
// truth for membership; the Gateway only forwards discovered chat members.
type SyncCore interface {
	UpsertUser(context.Context, *corev1.UpsertUserRequest) (*corev1.UpsertUserResponse, error)
	AddGroupMembers(context.Context, *corev1.AddGroupMembersRequest) (*corev1.AddGroupMembersResponse, error)
}

type Service struct {
	store  *postgresrepo.Store
	maxAPI *maxapi.Client
	core   SyncCore
}

func New(store *postgresrepo.Store, maxAPI *maxapi.Client, core SyncCore) *Service {
	return &Service{store: store, maxAPI: maxAPI, core: core}
}

// SyncCounts reports the outcome of a chat -> group member synchronization.
type SyncCounts struct {
	Discovered     int
	Added          int
	AlreadyPresent int
	Unavailable    int
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
		return nil, ErrBotAdminRequired
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
	// Any MAX API failure (auth, transport, permissions) means synchronization
	// is unavailable right now; surface a stable, recoverable error class.
	return &UnavailableError{ChatID: chatID, Cause: err}
}

// Sync copies MAX chat members into the bound Delim group. Idempotent; a member
// that leaves the MAX chat is never removed here (financial history persists).
func (s *Service) Sync(ctx context.Context, chatID, actorUserID, groupID int64) (SyncCounts, error) {
	users, err := s.GetMembers(ctx, chatID)
	if err != nil {
		return SyncCounts{}, err
	}
	counts := SyncCounts{Discovered: len(users)}
	coreIDs := make([]int64, 0, len(users))
	for _, user := range users {
		response, err := s.core.UpsertUser(ctx, &corev1.UpsertUserRequest{
			MaxUserId: user.UserID, FirstName: user.FirstName, LastName: user.LastName,
			Username: user.Username,
		})
		if err != nil || response.GetUser().GetId() == 0 {
			counts.Unavailable++
			continue
		}
		coreIDs = append(coreIDs, response.GetUser().GetId())
	}
	if len(coreIDs) > 0 {
		added, err := s.core.AddGroupMembers(ctx, &corev1.AddGroupMembersRequest{
			ActorUserId: actorUserID, GroupId: groupID, UserIds: coreIDs,
		})
		if err != nil {
			return counts, err
		}
		counts.Added = len(added.GetMembers())
	}
	counts.AlreadyPresent = len(coreIDs) - counts.Added
	return counts, nil
}
