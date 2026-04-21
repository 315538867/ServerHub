package sshpool

import (
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// GormHostKeyStore persists SSH host-key fingerprints into the Server row.
// Implements HostKeyStore.
type GormHostKeyStore struct{ DB *gorm.DB }

func NewGormHostKeyStore(db *gorm.DB) *GormHostKeyStore { return &GormHostKeyStore{DB: db} }

func (s *GormHostKeyStore) Get(id uint) (string, bool) {
	var srv model.Server
	if err := s.DB.Select("host_key_fp").First(&srv, id).Error; err != nil {
		return "", false
	}
	return srv.HostKeyFP, srv.HostKeyFP != ""
}

func (s *GormHostKeyStore) Set(id uint, fp string) error {
	return s.DB.Model(&model.Server{}).Where("id = ?", id).
		Update("host_key_fp", fp).Error
}
