package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Video merepresentasikan entitas basis data untuk tabel videos pada PostgreSQL[cite: 2].
// Berkas ini mendefinisikan struktur data materi edukasi video YouTube yang dikelola oleh Admin[cite: 2],
// serta terikat melalui relasi Foreign Key ke entitas User (AdminID)[cite: 2].
type Video struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	AdminID     uuid.UUID `gorm:"type:uuid;not null;index" json:"admin_id"`
	Admin       *User     `gorm:"foreignKey:AdminID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"admin,omitempty"`
	Title       string    `gorm:"type:varchar(255);not null" json:"title"`
	YoutubeURL  string    `gorm:"type:varchar(255);not null" json:"youtube_url"`
	Description string    `gorm:"type:text" json:"description"`
	Category    string    `gorm:"type:varchar(100);not null" json:"category"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName menetapkan penamaan skema tabel PostgreSQL secara eksplisit untuk entitas Video[cite: 2].
func (Video) TableName() string {
	return "videos"
}

// BeforeCreate merupakan life-cycle hook GORM untuk mengeksekusi otomatisasi penciptaan UUID v4
// sebelum operasi insert basis data dijalankan.
func (v *Video) BeforeCreate(tx *gorm.DB) (err error) {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return nil
}
