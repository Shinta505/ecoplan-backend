package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserRole mendefinisikan tipe data eksplisit untuk konstanta peranan pengguna dalam sistem[cite: 1].
type UserRole string

const (
	RoleAdmin  UserRole = "admin"
	RoleUser   UserRole = "user"
	RoleSeller UserRole = "seller"
)

// User merepresentasikan entitas basis data untuk tabel users pada PostgreSQL[cite: 1].
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name         string    `gorm:"type:varchar(255);not null" json:"name"`
	Email        string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"type:varchar(255);not null" json:"-"`
	Role         UserRole  `gorm:"type:varchar(20);default:'user';not null" json:"role"`
	EcoPoints    int       `gorm:"type:integer;default:0;not null" json:"eco_points"`
	Balance      float64   `gorm:"type:numeric(15,2);default:0.00;not null" json:"balance"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName menetapkan penamaan skema tabel PostgreSQL secara spesifik.
func (User) TableName() string {
	return "users"
}

// BeforeCreate merupakan life-cycle hook GORM untuk mengeksekusi otomatisasi penciptaan UUID v4 sebelum operasi insert dilakukan.
func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
