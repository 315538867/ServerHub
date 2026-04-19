package model

import "time"

type Setting struct {
	Key       string    `gorm:"primaryKey" json:"key"`
	Value     string    `gorm:"not null"   json:"value"`
	UpdatedAt time.Time `                  json:"updated_at"`
}
