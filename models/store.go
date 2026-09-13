package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Store merepresentasikan entitas basis data untuk tabel stores pada PostgreSQL[cite: 1].
// Berkas ini mendefinisikan struktur data toko yang terintegrasi dengan relasi Foreign Key ke entitas User[cite: 1].
type Store struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID           uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	User             *User     `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"user,omitempty"`
	StoreName        string    `gorm:"type:varchar(255);not null" json:"store_name"`
	StoreDescription string    `gorm:"type:text" json:"store_description"`
	Address          string    `gorm:"type:text;not null" json:"address"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName menetapkan penamaan skema tabel PostgreSQL secara spesifik[cite: 3].
func (Store) TableName() string {
	return "stores"
}

// BeforeCreate merupakan life-cycle hook GORM untuk mengeksekusi otomatisasi penciptaan UUID v4 sebelum operasi insert dilakukan[cite: 3].
func (s *Store) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}
