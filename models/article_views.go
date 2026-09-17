package models

import (
	"github.com/google/uuid"
	"time"
)

type ArticleView struct {
	ID        uint      `gorm:"primaryKey"`
	ArticleID uuid.UUID `gorm:"type:uuid;index:idx_article_ip,unique;not null"`
	IPAddress string    `gorm:"index:idx_article_ip,unique;size:45;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}
