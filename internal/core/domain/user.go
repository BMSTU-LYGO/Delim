package domain

import "time"

type User struct {
	ID        int64
	MaxUserID int64
	FirstName string
	LastName  string
	Username  string
	CreatedAt time.Time
	UpdatedAt time.Time
}
