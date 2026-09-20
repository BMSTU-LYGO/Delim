package http

import (
	"context"
	"fmt"
	"strings"

	"delim/internal/gateway/notifications"
	corev1 "delim/pkg/gen/core/v1"
)

// notifyGroupEvent resolves Delim-group participants before enqueueing private
// MAX messages. A failed lookup is intentionally ignored: notifications must
// never change the result of the already-completed Core operation.
func notifyGroupEvent(ctx context.Context, core groupClient, notifier *notifications.Notifier, actorID, groupID int64, groupName, kind, dedupeKey string, payload notifications.Payload) {
	if notifier == nil || groupID <= 0 {
		return
	}
	members, err := core.ListGroupMembers(ctx, &corev1.ListGroupMembersRequest{ActorUserId: actorID, GroupId: groupID})
	if err != nil {
		return
	}
	maxUserIDs := make([]int64, 0, len(members.GetMembers()))
	actorName := fmt.Sprintf("Пользователь #%d", actorID)
	for _, member := range members.GetMembers() {
		if member.GetUser().GetMaxUserId() > 0 {
			maxUserIDs = append(maxUserIDs, member.GetUser().GetMaxUserId())
		}
		if member.GetUserId() == actorID {
			if name := strings.TrimSpace(member.GetUser().GetFirstName() + " " + member.GetUser().GetLastName()); name != "" {
				actorName = name
			} else if member.GetUser().GetUsername() != "" {
				actorName = member.GetUser().GetUsername()
			}
		}
	}
	payload.Text = fmt.Sprintf("Группа «%s»: %s\nДействие: %s", groupName, payload.Text, actorName)
	_ = notifier.NotifyPersonal(ctx, maxUserIDs, kind, dedupeKey, payload)
}
