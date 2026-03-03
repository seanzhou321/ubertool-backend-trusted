package domain

import "time"

type FcmToken struct {
	ID              int32
	UserID          int32
	Token           string
	AndroidDeviceID string
	DeviceInfo      map[string]string
	Status          string // ACTIVE, OBSOLETE
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
