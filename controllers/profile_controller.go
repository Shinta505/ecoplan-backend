package controllers

import (
	"errors"
	"net/http"
	"strings"

	"ecoplan-backend/config"
	"ecoplan-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UpdateProfileInput mendefinisikan struktur Data Transfer Object (DTO)
// untuk memvalidasi payload HTTP JSON pada operasi pembaruan informasi profil pengguna.
type UpdateProfileInput struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

// ProfileSummaryResponse mendefinisikan struktur DTO untuk menyajikan respons data profil
// secara komprehensif, mencakup informasi pengguna, toko (jika terdaftar), akumulasi eco-points,
// saldo rekening, serta agregasi statistik aktivitas platform.
type ProfileSummaryResponse struct {
	User              models.User   `json:"user"`
	Store             *models.Store `json:"store,omitempty"`
	TotalDetections   int64         `json:"total_detections"`
	TotalTransactions int64         `json:"total_transactions"`
	TotalArticles     int64         `json:"total_articles"`
}

// TransactionHistoryResponse mendefinisikan struktur DTO untuk menyajikan riwayat transaksi
// beserta informasi ringkasan saldo pengguna.
type TransactionHistoryResponse struct {
	CurrentBalance float64              `json:"current_balance"`
	EcoPoints      int                  `json:"eco_points"`
	TotalCount     int64                `json:"total_count"`
	Transactions   []models.Transaction `json:"transactions"`
}

// GetProfile mengeksekusi pengambilan data profil pengguna yang sedang terautentikasi (login).
// Fungsi ini menyajikan data identitas diri, saldo rekening escrow/penarikan, akumulasi eco-points,
// rincian toko (jika bertindak sebagai penjual/seller), serta statistik aktivitas platform.
//
// HTTP Endpoint: GET /api/v1/profile
// Access Control: Authenticated User (Bearer JWT)
func GetProfile(c *gin.Context) {
	// 1. Mengambil User ID dari konteks otentikasi middleware JWT
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengonversi identitas pengguna (User ID).",
		})
		return
	}

	// 2. Membaca data pengguna dari basis data PostgreSQL berdasarkan UUID
	var user models.User
	if err := config.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Data profil pengguna tidak ditemukan dalam sistem.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat mengambil data profil dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	// 3. Memeriksa keberadaan data toko yang terikat dengan pengguna (Multi-Role Support)
	var store models.Store
	var storePtr *models.Store
	if err := config.DB.Where("user_id = ?", userID).First(&store).Error; err == nil {
		storePtr = &store
	}

	// 4. Mengkalkulasi agregasi statistik aktivitas pengguna pada platform EcoPlan
	var totalDetections int64
	var totalTransactions int64
	var totalArticles int64

	// Akumulasi riwayat pemindaian sampah AI
	config.DB.Model(&models.WasteDetection{}).Where("user_id = ?", userID).Count(&totalDetections)

	// Akumulasi riwayat transaksi belanja
	config.DB.Model(&models.Transaction{}).Where("buyer_id = ?", userID).Count(&totalTransactions)

	// Akumulasi publikasi artikel edukasi
	config.DB.Model(&models.Article{}).Where("author_id = ?", userID).Count(&totalArticles)

	// 5. Mengembalikan respons JSON dengan data profil lengkap
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil data profil dan akumulasi statistik pengguna.",
		"data": ProfileSummaryResponse{
			User:              user,
			Store:             storePtr,
			TotalDetections:   totalDetections,
			TotalTransactions: totalTransactions,
			TotalArticles:     totalArticles,
		},
	})
}

// UpdateProfile mengeksekusi pembaruan informasi profil pengguna (seperti Nama Lengkap dan Email).
// Fungsi ini juga melakukan validasi keunikan alamat email untuk mencegah duplikasi data.
//
// HTTP Endpoint: PUT /api/v1/profile
// Access Control: Authenticated User (Bearer JWT)
func UpdateProfile(c *gin.Context) {
	// 1. Mengambil User ID dari konteks otentikasi middleware JWT
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	userID := userIDVal.(uuid.UUID)

	// 2. Validasi format skema payload HTTP Request Body
	var input UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format data masukan tidak valid.",
			"error":   err.Error(),
		})
		return
	}

	// 3. Normalisasi input email dan pemrosesan keunikan email pada basis data
	normalizedEmail := strings.ToLower(strings.TrimSpace(input.Email))

	var existingUser models.User
	err := config.DB.Where("email = ? AND id != ?", normalizedEmail, userID).First(&existingUser).Error
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "Alamat email sudah digunakan oleh pengguna lain.",
		})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal memverifikasi ketersediaan email pada basis data.",
		})
		return
	}

	// 4. Membaca entitas pengguna dari basis data
	var user models.User
	if err := config.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Data pengguna tidak ditemukan.",
		})
		return
	}

	// 5. Melakukan pembaruan atribut nama dan email
	user.Name = strings.TrimSpace(input.Name)
	user.Email = normalizedEmail

	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal memperbarui data profil pengguna ke basis data.",
			"error":   err.Error(),
		})
		return
	}

	// 6. Mengembalikan respons JSON pembaruan profil yang sukses
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Informasi profil pengguna berhasil diperbarui.",
		"data":    user,
	})
}

// GetProfileTransactions mengeksekusi pengambilan riwayat transaksi pembelian produk marketplace
// beserta rincian saldo transaksi escrow pengguna.
//
// HTTP Endpoint: GET /api/v1/profile/transactions
// Access Control: Authenticated User (Bearer JWT)
func GetProfileTransactions(c *gin.Context) {
	// 1. Mengambil User ID dari konteks otentikasi middleware JWT
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	userID := userIDVal.(uuid.UUID)

	// 2. Membaca data pengguna untuk mendapatkan nilai saldo dan eco-points terbaru
	var user models.User
	if err := config.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Data pengguna tidak ditemukan.",
		})
		return
	}

	// 3. Mengambil daftar transaksi pengguna dari tabel transactions
	// Preload dilakukan pada relasi Items dan Product untuk mendapatkan rincian item yang dibeli
	var transactions []models.Transaction
	err := config.DB.Where("buyer_id = ?", userID).
		Preload("Items.Product").
		Order("created_at DESC").
		Find(&transactions).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil data riwayat transaksi dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	var totalCount int64 = int64(len(transactions))

	// 4. Mengembalikan respons JSON riwayat transaksi dan saldo
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil riwayat transaksi dan saldo pengguna.",
		"data": TransactionHistoryResponse{
			CurrentBalance: user.Balance,
			EcoPoints:      user.EcoPoints,
			TotalCount:     totalCount,
			Transactions:   transactions,
		},
	})
}

// GetDetectionHistory mengeksekusi pengambilan riwayat hasil pemindaian/deteksi sampah AI
// yang pernah dilakukan oleh pengguna yang sedang login.
//
// HTTP Endpoint: GET /api/v1/profile/detections
// Access Control: Authenticated User (Bearer JWT)
func GetDetectionHistory(c *gin.Context) {
	// 1. Mengambil User ID dari konteks otentikasi middleware JWT
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	userID := userIDVal.(uuid.UUID)

	// 2. Mengambil riwayat deteksi dari tabel waste_detections
	var detections []models.WasteDetection
	err := config.DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&detections).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil riwayat deteksi sampah dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	// 3. Mengembalikan respons JSON riwayat deteksi
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil riwayat pemindaian deteksi sampah pengguna.",
		"data": gin.H{
			"total_count": len(detections),
			"detections":  detections,
		},
	})
}
