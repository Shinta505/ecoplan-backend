package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Product merepresentasikan entitas basis data untuk tabel products pada PostgreSQL.
// Berkas ini mendefinisikan struktur data barang daur ulang, sampah layak pakai, maupun kompos
// yang terikat melalui relasi Foreign Key ke entitas toko (stores.id).
type Product struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	StoreID     uuid.UUID `gorm:"type:uuid;not null;index" json:"store_id"`
	Store       *Store    `gorm:"foreignKey:StoreID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"store,omitempty"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Price       float64   `gorm:"type:numeric(15,2);not null;default:0.00" json:"price"`
	Stock       int       `gorm:"type:integer;not null;default:0" json:"stock"`
	Category    string    `gorm:"type:varchar(100);not null" json:"category"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName menetapkan penamaan skema tabel PostgreSQL secara eksplisit.
func (Product) TableName() string {
	return "products"
}

// BeforeCreate merupakan life-cycle hook GORM untuk mengeksekusi otomatisasi penciptaan UUID v4 
// sebelum operasi insert basis data dijalankan.
func (p *Product) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}