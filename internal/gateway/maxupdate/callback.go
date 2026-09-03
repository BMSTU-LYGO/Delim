package maxupdate

import (
	"errors"
	"strings"
)

const callbackVersion = "v1"

var ErrInvalidCallbackPayload = errors.New("invalid callback payload")

type CallbackAction struct {
	Version string
	Action  string
}

func ParseCallbackPayload(payload string) (CallbackAction, error) {
	version, action, ok := strings.Cut(payload, ":")
	if !ok || version != callbackVersion || action == "" || strings.Contains(action, ":") {
		return CallbackAction{}, ErrInvalidCallbackPayload
	}
	return CallbackAction{Version: version, Action: action}, nil
}
