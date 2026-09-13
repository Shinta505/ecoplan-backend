package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ArticleStatus mendefinisikan tipe data eksplisit untuk konstanta status alur publikasi artikel edukasi[cite: 1].
type ArticleStatus string

const (
	ArticleStatusDraft          ArticleStatus = "draft"
	ArticleStatusReviewTurnitin ArticleStatus = "review_turnitin"
	ArticleStatusApproved       ArticleStatus = "approved"
	ArticleStatusPublished      ArticleStatus = "published"
	ArticleStatusRejected       ArticleStatus = "rejected"
)

// Article merepresentasikan entitas basis data untuk tabel articles pada PostgreSQL[cite: 1].
// Berkas ini menyimpan informasi artikel edukasi lingkungan, relasi ke akun penulis (AuthorID),
// persentase skor plagiasi Turnitin, status alur moderasi admin, serta akumulasi penonton (ViewsCount)[cite: 1].
type Article struct {
	ID            uuid.UUID     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	AuthorID      uuid.UUID     `gorm:"type:uuid;not null;index" json:"author_id"`
	Author        *User         `gorm:"foreignKey:AuthorID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"author,omitempty"`
	Title         string        `gorm:"type:varchar(255);not null" json:"title"`
	Content       string        `gorm:"type:text;not null" json:"content"`
	TurnitinScore float64       `gorm:"type:numeric(5,2);default:0.00;not null" json:"turnitin_score"`
	Status        ArticleStatus `gorm:"type:varchar(50);default:'draft';not null" json:"status"`
	ViewsCount    int           `gorm:"type:integer;default:0;not null" json:"views_count"`
	CreatedAt     time.Time     `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time     `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName menetapkan penamaan skema tabel PostgreSQL secara eksplisit untuk entitas Article[cite: 1].
func (Article) TableName() string {
	return "articles"
}

// BeforeCreate merupakan life-cycle hook GORM untuk mengeksekusi otomatisasi penciptaan UUID v4
// sebelum operasi insert basis data dijalankan.
func (a *Article) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
