package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WasteDetection merepresentasikan entitas basis data untuk tabel waste_detections pada PostgreSQL[cite: 2].
// Berkas ini menyimpan riwayat hasil klasifikasi citra sampah menggunakan model ResNet50 ONNX[cite: 1],
// dengan atribut UserID yang bersifat opsional (nullable) untuk mengakomodasi akses dari pengguna anonim (Guest)[cite: 2].
type WasteDetection struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID            *uuid.UUID `gorm:"type:uuid;index" json:"user_id,omitempty"`
	User              *User      `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"user,omitempty"`
	ImageURL          string     `gorm:"type:varchar(255);not null" json:"image_url"`
	PredictedCategory string     `gorm:"type:varchar(100);not null" json:"predicted_category"`
	WasteGroup        string     `gorm:"type:varchar(100);not null" json:"waste_group"`
	ConfidenceScore   float64    `gorm:"type:numeric(5,2);not null" json:"confidence_score"`
	ModelUsed         string     `gorm:"type:varchar(50);default:'ResNet50_ONNX';not null" json:"model_used"`
	CreatedAt         time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName menetapkan penamaan skema tabel PostgreSQL secara eksplisit[cite: 2, 4].
func (WasteDetection) TableName() string {
	return "waste_detections"
}

// BeforeCreate merupakan life-cycle hook GORM untuk mengeksekusi otomatisasi penciptaan UUID v4
// sebelum operasi insert basis data dijalankan[cite: 4].
func (w *WasteDetection) BeforeCreate(tx *gorm.DB) (err error) {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}
