package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"ecoplan-backend/config"
	"ecoplan-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateVideoInput mendefinisikan struktur DTO (Data Transfer Object) untuk enkapsulasi payload penambahan video edukasi baru.
type CreateVideoInput struct {
	Title       string `json:"title" binding:"required"`
	YoutubeURL  string `json:"youtube_url" binding:"required"`
	Description string `json:"description"`
	Category    string `json:"category" binding:"required"`
}

// UpdateVideoInput mendefinisikan struktur DTO untuk pembaruan data video edukasi oleh Admin.
type UpdateVideoInput struct {
	Title       string `json:"title"`
	YoutubeURL  string `json:"youtube_url"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// Validasi 10 Kategori Sampah Resmi Platform EcoPlan
var validVideoCategories = map[string]bool{
	"battery":    true,
	"biological": true,
	"cardboard":  true,
	"clothes":    true,
	"glass":      true,
	"metal":      true,
	"paper":      true,
	"plastic":    true,
	"shoes":      true,
	"trash":      true,
}

// CreateVideo mengeksekusi penambahan tautan video edukasi YouTube baru ke dalam katalog sistem oleh Administrator.
//
// HTTP Endpoint : POST /api/v1/videos
// Hak Akses     : Khusus Admin (RoleAdmin)
func CreateVideo(c *gin.Context) {
	// 1. Membaca identitas Administrator (Admin ID) dari konteks otentikasi JWT Middleware
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi administrator tidak ditemukan.",
		})
		return
	}

	adminID, ok := userIDVal.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengonversi identitas administrator (User ID).",
		})
		return
	}

	// 2. Validasi format skema payload HTTP Request Body
	var input CreateVideoInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format data masukan tidak valid. Pastikan judul, tautan YouTube, dan kategori telah diisi.",
			"error":   err.Error(),
		})
		return
	}

	// 3. Validasi kesesuaian kategori sampah dengan 10 kategori resmi EcoPlan
	categoryLower := strings.ToLower(strings.TrimSpace(input.Category))
	if !validVideoCategories[categoryLower] {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Kategori '%s' tidak valid. Gunakan salah satu dari 10 kategori resmi: battery, biological, cardboard, clothes, glass, metal, paper, plastic, shoes, trash.", input.Category),
		})
		return
	}

	// 4. Instansiasi objek model Video
	newVideo := models.Video{
		AdminID:     adminID,
		Title:       strings.TrimSpace(input.Title),
		YoutubeURL:  strings.TrimSpace(input.YoutubeURL),
		Description: strings.TrimSpace(input.Description),
		Category:    categoryLower,
	}

	// 5. Menyimpan entitas video baru ke basis data PostgreSQL/Supabase
	if err := config.DB.Create(&newVideo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menyimpan data video edukasi ke basis data.",
			"error":   err.Error(),
		})
		return
	}

	// 6. Preload data Administrator (Admin) untuk penyajian respons lengkap
	config.DB.Preload("Admin").First(&newVideo, newVideo.ID)

	// 7. Mengembalikan respons status HTTP 201 Created
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Berhasil menambahkan video edukasi YouTube baru ke dalam katalog.",
		"data":    newVideo,
	})
}

// GetAllVideos mengeksekusi pengambilan daftar video edukasi pengolahan sampah.
// Fitur ini mendukung pencarian berbasis kata kunci, filter kategori spesifik, dan paginasi data.
//
// HTTP Endpoint : GET /api/v1/videos
// Hak Akses     : Publik / Authenticated User / Admin
func GetAllVideos(c *gin.Context) {
	// 1. Membaca parameter query untuk pencarian, filter kategori, dan paginasi
	categoryQuery := strings.ToLower(strings.TrimSpace(c.Query("category")))
	searchQuery := c.Query("search")
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := config.DB.Model(&models.Video{})

	// 2. Menambahkan filter berdasarkan kategori spesifik jika dicantumkan
	if categoryQuery != "" {
		if !validVideoCategories[categoryQuery] {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("Kategori '%s' tidak valid. Gunakan salah satu dari 10 kategori resmi.", categoryQuery),
			})
			return
		}
		query = query.Where("category = ?", categoryQuery)
	}

	// 3. Menambahkan pencarian berbasis kata kunci pada judul atau deskripsi video
	if strings.TrimSpace(searchQuery) != "" {
		searchTerm := "%" + strings.TrimSpace(searchQuery) + "%"
		query = query.Where("title ILIKE ? OR description ILIKE ?", searchTerm, searchTerm)
	}

	// 4. Hitung total akumulasi rekam data video
	var totalRecords int64
	query.Count(&totalRecords)

	// 5. Mengambil daftar video dengan paginasi dan relasi data Administrator (Admin)
	var videos []models.Video
	err := query.Preload("Admin").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&videos).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil daftar video edukasi dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	// 6. Mengembalikan respons JSON daftar video edukasi
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil daftar video edukasi YouTube.",
		"data": gin.H{
			"videos":        videos,
			"total_records": totalRecords,
			"page":          page,
			"limit":         limit,
			"total_pages":   (totalRecords + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetVideoByID mengambil rincian detail video edukasi berdasarkan UUID Identifier.
//
// HTTP Endpoint : GET /api/v1/videos/:id
// Hak Akses     : Publik / Authenticated User / Admin
func GetVideoByID(c *gin.Context) {
	videoIDStr := c.Param("id")
	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format ID video tidak valid. Harap gunakan format UUID.",
		})
		return
	}

	// Membaca entitas video dari basis data beserta relasi Admin
	var video models.Video
	if err := config.DB.Preload("Admin").Where("id = ?", videoID).First(&video).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Data video edukasi tidak ditemukan.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat membaca data video dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil rincian detail video edukasi.",
		"data":    video,
	})
}

// UpdateVideo memfasilitasi pembaruan informasi video edukasi oleh Admin.
//
// HTTP Endpoint : PUT /api/v1/videos/:id
// Hak Akses     : Khusus Admin (RoleAdmin)
func UpdateVideo(c *gin.Context) {
	videoIDStr := c.Param("id")
	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format ID video tidak valid. Harap gunakan format UUID.",
		})
		return
	}

	// 1. Validasi payload pembaruan video
	var input UpdateVideoInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format data pembaruan video tidak valid.",
			"error":   err.Error(),
		})
		return
	}

	// 2. Pencarian entitas video pada basis data
	var video models.Video
	if err := config.DB.Where("id = ?", videoID).First(&video).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Video edukasi tidak ditemukan.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat membaca data video.",
			"error":   err.Error(),
		})
		return
	}

	// 3. Melakukan pembaruan nilai atribut jika diberikan pada payload
	if strings.TrimSpace(input.Title) != "" {
		video.Title = strings.TrimSpace(input.Title)
	}
	if strings.TrimSpace(input.YoutubeURL) != "" {
		video.YoutubeURL = strings.TrimSpace(input.YoutubeURL)
	}
	if input.Description != "" {
		video.Description = strings.TrimSpace(input.Description)
	}
	if strings.TrimSpace(input.Category) != "" {
		categoryLower := strings.ToLower(strings.TrimSpace(input.Category))
		if !validVideoCategories[categoryLower] {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("Kategori '%s' tidak valid. Gunakan salah satu dari 10 kategori resmi.", input.Category),
			})
			return
		}
		video.Category = categoryLower
	}

	// 4. Menyimpan pembaruan ke basis data PostgreSQL
	if err := config.DB.Save(&video).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal memperbarui data video edukasi ke basis data.",
			"error":   err.Error(),
		})
		return
	}

	config.DB.Preload("Admin").First(&video, video.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Informasi video edukasi berhasil diperbarui.",
		"data":    video,
	})
}

// DeleteVideo menghapus rekam data video edukasi dari basis data PostgreSQL.
//
// HTTP Endpoint : DELETE /api/v1/videos/:id
// Hak Akses     : Khusus Admin (RoleAdmin)
func DeleteVideo(c *gin.Context) {
	videoIDStr := c.Param("id")
	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format ID video tidak valid. Harap gunakan format UUID.",
		})
		return
	}

	// 1. Membaca entitas video dari basis data
	var video models.Video
	if err := config.DB.Where("id = ?", videoID).First(&video).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Video edukasi tidak ditemukan.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat membaca data video.",
			"error":   err.Error(),
		})
		return
	}

	// 2. Eksekusi operasi penghapusan entitas video dari basis data
	if err := config.DB.Delete(&video).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menghapus video edukasi dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Video edukasi berhasil dihapus dari sistem.",
	})
}
