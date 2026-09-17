package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ecoplan-backend/config"
	"ecoplan-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateArticleInput mendefinisikan struktur DTO (Data Transfer Object) untuk enkapsulasi payload pengajuan draf artikel baru.
type CreateArticleInput struct {
	Title         string  `json:"title" binding:"required"`
	Content       string  `json:"content" binding:"required"`
	TurnitinScore float64 `json:"turnitin_score"`
}

// UpdateArticleInput mendefinisikan struktur DTO untuk pembaruan draf artikel oleh penulis.
type UpdateArticleInput struct {
	Title         string   `json:"title"`
	Content       string   `json:"content"`
	TurnitinScore *float64 `json:"turnitin_score"`
}

// ReviewArticleInput mendefinisikan struktur DTO untuk proses verifikasi, pengecekan Turnitin, dan moderasi publikasi oleh Admin.
type ReviewArticleInput struct {
	Status        models.ArticleStatus `json:"status" binding:"required"`
	TurnitinScore *float64             `json:"turnitin_score"`
}

// CreateArticle mengeksekusi pengiriman draf artikel baru oleh pengguna terautentikasi (Member/User).
// Jika pengguna mencantumkan skor Turnitin, status alur publikasi otomatis diset ke 'review_turnitin'.
//
// HTTP Endpoint : POST /api/v1/articles
// Hak Akses     : Authenticated User (Member / User / Seller / Admin)
func CreateArticle(c *gin.Context) {
	// 1. Membaca identitas pengguna (User ID) dari konteks otentikasi JWT Middleware
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

	// 2. Validasi format skema payload HTTP Request Body
	var input CreateArticleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format data masukan tidak valid. Pastikan judul dan konten artikel telah diisi.",
			"error":   err.Error(),
		})
		return
	}

	// 3. Menentukan status awal alur pengajuan artikel
	// Jika skor Turnitin sudah dilampirkan, alur dialihkan ke tahap 'review_turnitin', selain itu menjadi 'draft'
	initialStatus := models.ArticleStatusDraft
	if input.TurnitinScore > 0 {
		initialStatus = models.ArticleStatusReviewTurnitin
	}

	// 4. Instansiasi objek model Article
	newArticle := models.Article{
		AuthorID:      userID,
		Title:         strings.TrimSpace(input.Title),
		Content:       strings.TrimSpace(input.Content),
		TurnitinScore: input.TurnitinScore,
		Status:        initialStatus,
		ViewsCount:    0,
	}

	// 5. Menyimpan entitas draf artikel ke basis data PostgreSQL/Supabase
	if err := config.DB.Create(&newArticle).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menyimpan draf artikel ke basis data.",
			"error":   err.Error(),
		})
		return
	}

	// 6. Preload data penulis (Author) untuk penyajian respons lengkap
	config.DB.Preload("Author").First(&newArticle, newArticle.ID)

	// 7. Mengembalikan respons status HTTP 201 Created
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Draf artikel berhasil dikirimkan dan siap dilakukan verifikasi moderasi.",
		"data":    newArticle,
	})
}

// GetAllArticles mengeksekusi pengambilan daftar artikel edukasi pengolahan sampah.
// Pengguna umum hanya dapat melihat artikel berstatus 'published', sedangkan Admin dapat memfilter seluruh status.
//
// HTTP Endpoint : GET /api/v1/articles
// Hak Akses     : Publik / Authenticated User
func GetAllArticles(c *gin.Context) {
	// 1. Membaca parameter query untuk pencarian, filter status, dan paginasi
	searchQuery := c.Query("search")
	statusQuery := c.Query("status")
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

	// 2. Menentukan kebijakan visibilitas artikel berdasarkan peranan pengguna (Role-Based Control)
	userRoleVal, hasRole := c.Get("user_role")
	isAdmin := false
	if hasRole {
		if role, ok := userRoleVal.(models.UserRole); ok && role == models.RoleAdmin {
			isAdmin = true
		}
	}

	query := config.DB.Model(&models.Article{})

	// Jika bukan admin, batasi penayangan hanya untuk artikel yang telah dipublikasikan secara resmi ('published')
	if !isAdmin {
		query = query.Where("status = ?", models.ArticleStatusPublished)
	} else if statusQuery != "" {
		// Admin dapat memilih filter status spesifik (draft, review_turnitin, approved, published, rejected)
		query = query.Where("status = ?", statusQuery)
	}

	// 3. Menambahkan pencarian berbasis kata kunci pada judul atau konten artikel
	if strings.TrimSpace(searchQuery) != "" {
		searchTerm := "%" + strings.TrimSpace(searchQuery) + "%"
		query = query.Where("title ILIKE ? OR content ILIKE ?", searchTerm, searchTerm)
	}

	// 4. Hitung total akumulasi rekam data artikel
	var totalRecords int64
	query.Count(&totalRecords)

	// 5. Mengambil daftar artikel dengan paginasi dan relasi data Penulis (Author)
	var articles []models.Article
	err := query.Preload("Author").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&articles).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil daftar artikel dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	// 6. Mengembalikan respons JSON daftar artikel
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil daftar artikel edukasi.",
		"data": gin.H{
			"articles":      articles,
			"total_records": totalRecords,
			"page":          page,
			"limit":         limit,
			"total_pages":   (totalRecords + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetArticleByID mengambil rincian detail artikel berdasarkan UUID Identifier
// sekaligus mencatat penambahan jumlah pembaca (ViewsCount) & reward Eco-Points kelipatan 10.
//
// HTTP Endpoint : GET /api/v1/articles/:id
// Hak Akses     : Publik / Authenticated User
func GetArticleByID(c *gin.Context) {
	articleIDStr := c.Param("id")
	clientIP := c.ClientIP()

	// 1. Validasi format UUID artikel
	parsedArticleID, err := uuid.Parse(articleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format ID artikel tidak valid. Harap gunakan format UUID.",
		})
		return
	}

	// 2. Cek apakah IP ini sudah melihat artikel ini dalam 24 jam terakhir
	var existingView models.ArticleView
	err = config.DB.Where("article_id = ? AND ip_address = ? AND created_at >= ?",
		parsedArticleID, clientIP, time.Now().Add(-24*time.Hour)).First(&existingView).Error

	// 3. Jika belum ada, catat view baru dan increment views_count secara aman
	if err != nil {
		tx := config.DB.Begin()

		newView := models.ArticleView{
			ArticleID: parsedArticleID,
			IPAddress: clientIP,
		}

		if err := tx.Create(&newView).Error; err == nil {
			// Increment views_count pada artikel
			tx.Model(&models.Article{}).Where("id = ?", parsedArticleID).
				UpdateColumn("views_count", gorm.Expr("views_count + ?", 1))

			// Ambil data artikel terbaru untuk mengecek total views_count saat ini
			var tempArticle models.Article
			if err := tx.Where("id = ?", parsedArticleID).First(&tempArticle).Error; err == nil {
				// Jika views_count kelipatan 10, berikan bonus +7 Eco-Points ke Author
				if tempArticle.ViewsCount > 0 && tempArticle.ViewsCount%10 == 0 {
					tx.Model(&models.User{}).Where("id = ?", tempArticle.AuthorID).
						UpdateColumn("eco_points", gorm.Expr("eco_points + ?", 7))
				}
			}

			tx.Commit()
		} else {
			tx.Rollback()
		}
	}

	// 4. Ambil data artikel beserta relasi Author menggunakan parsedArticleID (UUID)
	var article models.Article
	if err := config.DB.Preload("Author").Where("id = ?", parsedArticleID).First(&article).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Artikel tidak ditemukan.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil data artikel.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil rincian artikel.",
		"data":    article,
	})
}

// ReviewArticle mengeksekusi proses verifikasi, pembaruan skor Turnitin, serta persetujuan moderasi oleh Admin.
//
// HTTP Endpoint : PATCH /api/v1/articles/:id/review
// Hak Akses     : Khusus Admin (RoleAdmin)
func ReviewArticle(c *gin.Context) {
	articleIDStr := c.Param("id")
	articleID, err := uuid.Parse(articleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format ID artikel tidak valid. Harap gunakan format UUID.",
		})
		return
	}

	// 1. Validasi struktur payload HTTP Request Body
	var input ReviewArticleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format payload moderasi tidak valid. Pastikan bidang status telah ditentukan.",
			"error":   err.Error(),
		})
		return
	}

	// 2. Validasi nilai enum status alur artikel
	validStatuses := map[models.ArticleStatus]bool{
		models.ArticleStatusDraft:          true,
		models.ArticleStatusReviewTurnitin: true,
		models.ArticleStatusApproved:       true,
		models.ArticleStatusPublished:      true,
		models.ArticleStatusRejected:       true,
	}

	if !validStatuses[input.Status] {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Nilai status '%s' tidak valid. Gunakan status: draft, review_turnitin, approved, published, atau rejected.", input.Status),
		})
		return
	}

	// 3. Membaca entitas artikel dari basis data
	var article models.Article
	if err := config.DB.Where("id = ?", articleID).First(&article).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Artikel yang akan ditinjau tidak ditemukan.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal membaca data artikel dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	// 4. Menyusun pembaruan atribut artikel (Status & Skor Turnitin)
	previousStatus := article.Status
	article.Status = input.Status
	if input.TurnitinScore != nil {
		article.TurnitinScore = *input.TurnitinScore
	}

	if err := config.DB.Save(&article).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menyimpan pembaruan status moderasi artikel.",
			"error":   err.Error(),
		})
		return
	}

	// 5. Pemberian hadiah Eco-Points awal kepada penulis (300 poin) jika artikel disetujui publikasi untuk pertama kali
	if previousStatus != models.ArticleStatusPublished && article.Status == models.ArticleStatusPublished {
		config.DB.Model(&models.User{}).
			Where("id = ?", article.AuthorID).
			UpdateColumn("eco_points", gorm.Expr("eco_points + ?", 300))
	}

	// 6. Preload data Penulis untuk menyajikan respons hasil moderasi
	config.DB.Preload("Author").First(&article, article.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Status moderasi artikel berhasil diperbarui menjadi '%s'.", article.Status),
		"data":    article,
	})
}

// GetMyArticles mengambil seluruh daftar artikel yang dibuat oleh pengguna yang sedang terautentikasi (login).
//
// HTTP Endpoint : GET /api/v1/articles/my-articles
// Hak Akses     : Authenticated User (Member / User / Seller / Admin)
func GetMyArticles(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	userID := userIDVal.(uuid.UUID)

	var articles []models.Article
	err := config.DB.Where("author_id = ?", userID).
		Order("created_at DESC").
		Find(&articles).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil daftar artikel milik pengguna.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil seluruh artikel karya penulis.",
		"data": gin.H{
			"total_count": len(articles),
			"articles":    articles,
		},
	})
}

// UpdateArticle memfasilitasi pembaruan draf atau konten artikel oleh penulis asli.
//
// HTTP Endpoint : PUT /api/v1/articles/:id
// Hak Akses     : Authenticated User (Pemilik Artikel)
func UpdateArticle(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	userID := userIDVal.(uuid.UUID)

	articleIDStr := c.Param("id")
	articleID, err := uuid.Parse(articleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format ID artikel tidak valid. Harap gunakan format UUID.",
		})
		return
	}

	// 1. Validasi payload pembaruan artikel
	var input UpdateArticleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format data pembaruan artikel tidak valid.",
			"error":   err.Error(),
		})
		return
	}

	// 2. Pencarian entitas artikel dan validasi kepemilikan (Ownership Check)
	var article models.Article
	if err := config.DB.Where("id = ?", articleID).First(&article).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Artikel tidak ditemukan.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat membaca data artikel.",
			"error":   err.Error(),
		})
		return
	}

	if article.AuthorID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Akses ditolak. Anda tidak memiliki hak akses untuk mengubah artikel ini.",
		})
		return
	}

	// 3. Melakukan pembaruan nilai atribut
	if strings.TrimSpace(input.Title) != "" {
		article.Title = strings.TrimSpace(input.Title)
	}
	if strings.TrimSpace(input.Content) != "" {
		article.Content = strings.TrimSpace(input.Content)
	}
	if input.TurnitinScore != nil {
		article.TurnitinScore = *input.TurnitinScore
		// Apabila skor Turnitin diperbarui, kembalikan status ke tahap 'review_turnitin'
		article.Status = models.ArticleStatusReviewTurnitin
	}

	// 4. Menyimpan pembaruan ke basis data PostgreSQL
	if err := config.DB.Save(&article).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal memperbarui artikel ke basis data.",
			"error":   err.Error(),
		})
		return
	}

	config.DB.Preload("Author").First(&article, article.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Informasi draf artikel berhasil diperbarui.",
		"data":    article,
	})
}

// DeleteArticle menghapus rekam data artikel dari basis data PostgreSQL.
//
// HTTP Endpoint : DELETE /api/v1/articles/:id
// Hak Akses     : Authenticated User (Penulis Artikel atau Admin)
func DeleteArticle(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	userID := userIDVal.(uuid.UUID)
	userRoleVal, _ := c.Get("user_role")
	userRole, _ := userRoleVal.(models.UserRole)

	articleIDStr := c.Param("id")
	articleID, err := uuid.Parse(articleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format ID artikel tidak valid. Harap gunakan format UUID.",
		})
		return
	}

	// 1. Membaca entitas artikel dari basis data
	var article models.Article
	if err := config.DB.Where("id = ?", articleID).First(&article).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Artikel tidak ditemukan.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat membaca data artikel.",
			"error":   err.Error(),
		})
		return
	}

	// 2. Memeriksa wewenang penghapusan (Hanya Penulis Asli atau Admin)
	if article.AuthorID != userID && userRole != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Akses ditolak. Anda tidak memiliki wewenang untuk menghapus artikel ini.",
		})
		return
	}

	// 3. Eksekusi operasi penghapusan entitas artikel dari basis data
	if err := config.DB.Delete(&article).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menghapus artikel dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Artikel berhasil dihapus dari sistem.",
	})
}
