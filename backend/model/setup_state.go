package model

import "time"

// SetupState holds transient state for the first-run wizard, primarily the
// ed25519 private key generated during the local-server activation step
// before the user has confirmed the host-side authorized_keys/sudo command.
//
// Stored as a single row (id=1). Cleared once the wizard finishes or the
// row expires.
type SetupState struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	EncryptedPrivateKey string    `gorm:"default:''" json:"-"` // AES-GCM encrypted ed25519 PEM
	PublicKey           string    `gorm:"default:''" json:"public_key"`
	HostGateway         string    `gorm:"default:''" json:"host_gateway"`
	TargetUser          string    `gorm:"default:''" json:"target_user"`
	ExpiresAt           time.Time `json:"expires_at"`
	CreatedAt           time.Time `json:"created_at"`
}
