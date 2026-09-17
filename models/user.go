package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserRole mendefinisikan tipe data eksplisit untuk konstanta peranan pengguna dalam sistem[cite: 1, 5].
type UserRole string

const (
	RoleAdmin  UserRole = "admin"
	RoleUser   UserRole = "user"
	RoleSeller UserRole = "seller"
)

// DiscountVoucher merepresentasikan entitas basis data untuk tabel discount_vouchers pada PostgreSQL,
// yang menyimpan data relasi voucher diskon hasil penukaran eco-points pengguna.
type DiscountVoucher struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID         uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Code           string    `gorm:"type:varchar(50);uniqueIndex;not null" json:"code"`
	DiscountAmount float64   `gorm:"type:numeric(15,2);not null;default:0.00" json:"discount_amount"`
	PointsSpent    int       `gorm:"type:integer;not null;default:0" json:"points_spent"`
	IsUsed         bool      `gorm:"type:boolean;default:false;not null" json:"is_used"`
	ExpiresAt      time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName menetapkan penamaan skema tabel PostgreSQL secara spesifik untuk entitas DiscountVoucher.
func (DiscountVoucher) TableName() string {
	return "discount_vouchers"
}

// BeforeCreate merupakan life-cycle hook GORM untuk mengeksekusi otomatisasi penciptaan UUID v4 sebelum operasi insert dilakukan.
func (dv *DiscountVoucher) BeforeCreate(tx *gorm.DB) (err error) {
	if dv.ID == uuid.Nil {
		dv.ID = uuid.New()
	}
	return nil
}

// User merepresentasikan entitas basis data untuk tabel users pada PostgreSQL[cite: 1, 5].
type User struct {
	ID               uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name             string            `gorm:"type:varchar(255);not null" json:"name"`
	Email            string            `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	PasswordHash     string            `gorm:"type:varchar(255);not null" json:"-"`
	Role             UserRole          `gorm:"type:varchar(20);default:'user';not null" json:"role"`
	EcoPoints        int               `gorm:"type:integer;default:0;not null" json:"eco_points"`
	Balance          float64           `gorm:"type:numeric(15,2);default:0.00;not null" json:"balance"`
	DiscountVouchers []DiscountVoucher `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"discount_vouchers,omitempty"`
	CreatedAt        time.Time         `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time         `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName menetapkan penamaan skema tabel PostgreSQL secara spesifik.
func (User) TableName() string {
	return "users"
}

// BeforeCreate merupakan life-cycle hook GORM untuk mengeksekusi otomatisasi penciptaan UUID v4 sebelum operasi insert dilakukan[cite: 5].
func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
