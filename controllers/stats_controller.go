package controllers

import (
	"net/http"

	"ecoplan-backend/config"
	"ecoplan-backend/models"

	"github.com/gin-gonic/gin"
)

// SystemStatsSummaryResponse mendefinisikan struktur DTO ringkasan agregasi statistik terpusat.
type SystemStatsSummaryResponse struct {
	TotalUsers        int64   `json:"total_users"`
	TotalStores       int64   `json:"total_stores"`
	TotalProducts     int64   `json:"total_products"`
	TotalDetections   int64   `json:"total_detections"`
	TotalArticles     int64   `json:"total_articles"`
	TotalVideos       int64   `json:"total_videos"`
	TotalTransactions int64   `json:"total_transactions"`
	TotalGrossVolume  float64 `json:"total_gross_volume"`
}

// GetStatsSummary mengeksekusi kalkulasi akumulasi data terpusat seluruh modul sistem EcoPlan.
//
// HTTP Endpoint : GET /api/v1/stats/summary
// Hak Akses     : Publik / Authenticated User
func GetStatsSummary(c *gin.Context) {
	var stats SystemStatsSummaryResponse

	// Agregasi penghitungan entitas basis data via GORM
	config.DB.Model(&models.User{}).Count(&stats.TotalUsers)
	config.DB.Model(&models.Store{}).Count(&stats.TotalStores)
	config.DB.Model(&models.Product{}).Count(&stats.TotalProducts)
	config.DB.Model(&models.WasteDetection{}).Count(&stats.TotalDetections)
	config.DB.Model(&models.Article{}).Where("status = ?", models.ArticleStatusPublished).Count(&stats.TotalArticles)
	config.DB.Model(&models.Video{}).Count(&stats.TotalVideos)
	config.DB.Model(&models.Transaction{}).Count(&stats.TotalTransactions)

	// Kalkulasi total perputaran akumulasi nilai transaksi sukses (Status: completed)
	config.DB.Model(&models.Transaction{}).
		Where("status = ?", models.StatusCompleted).
		Select("COALESCE(SUM(total_amount), 0)").
		Scan(&stats.TotalGrossVolume)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil memperoleh ringkasan statistik terpusat platform EcoPlan.",
		"data":    stats,
	})
}