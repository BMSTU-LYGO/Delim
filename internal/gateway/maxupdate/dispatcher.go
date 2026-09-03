package maxupdate

import (
	"context"
	"encoding/json"
)

type Update struct {
	UpdateType string          `json:"update_type"`
	Timestamp  int64           `json:"timestamp"`
	ChatID     int64           `json:"chat_id,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type Dispatcher struct{}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{}
}

func (d *Dispatcher) Dispatch(context.Context, Update) {}
