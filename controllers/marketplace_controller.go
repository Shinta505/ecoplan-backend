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

// Validasi 10 Kategori Sampah Resmi Platform EcoPlan
var validProductCategories = map[string]bool{
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

// CreateStoreInput mendefinisikan struktur DTO (Data Transfer Object) untuk registrasi toko baru.
type CreateStoreInput struct {
	StoreName        string `json:"store_name" binding:"required"`
	StoreDescription string `json:"store_description"`
	Address          string `json:"address" binding:"required"`
}

// UpdateStoreInput mendefinisikan struktur DTO untuk pembaruan profil toko oleh Seller.
type UpdateStoreInput struct {
	StoreName        string `json:"store_name"`
	StoreDescription string `json:"store_description"`
	Address          string `json:"address"`
}

// CreateProductInput mendefinisikan struktur DTO untuk penambahan produk daur ulang baru.
type CreateProductInput struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	Price       float64 `json:"price" binding:"required,gte=0"`
	Stock       int     `json:"stock" binding:"required,gte=0"`
	Category    string  `json:"category" binding:"required"`
}

// UpdateProductInput mendefinisikan struktur DTO untuk pembaruan katalog produk.
type UpdateProductInput struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       *float64 `json:"price"`
	Stock       *int     `json:"stock"`
	Category    string   `json:"category"`
}

// CreateStore mengeksekusi pendaftaran toko baru (Seller) untuk pengguna terautentikasi.
// Fungsi ini juga memperbarui peranan pengguna (UserRole) menjadi 'seller' secara otomatis.
//
// HTTP Endpoint : POST /api/v1/stores
// Hak Akses     : Authenticated User (Member / User)
func CreateStore(c *gin.Context) {
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

	// 2. Memeriksa apakah pengguna sudah memiliki toko yang terdaftar
	var existingStore models.Store
	err := config.DB.Where("user_id = ?", userID).First(&existingStore).Error
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "Pengguna sudah memiliki toko terdaftar.",
			"data":    existingStore,
		})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat memeriksa data toko pada basis data.",
			"error":   err.Error(),
		})
		return
	}

	// 3. Validasi format skema payload HTTP Request Body
	var input CreateStoreInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format data masukan tidak valid. Pastikan nama toko dan alamat telah diisi.",
			"error":   err.Error(),
		})
		return
	}

	// 4. Instansiasi objek model Store
	newStore := models.Store{
		UserID:           userID,
		StoreName:        strings.TrimSpace(input.StoreName),
		StoreDescription: strings.TrimSpace(input.StoreDescription),
		Address:          strings.TrimSpace(input.Address),
	}

	// 5. Menyimpan entitas toko ke basis data PostgreSQL/Supabase
	if err := config.DB.Create(&newStore).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menyimpan data toko ke basis data.",
			"error":   err.Error(),
		})
		return
	}

	// 6. Memperbarui peranan pengguna (role) menjadi 'seller' jika pengguna saat ini ber-role 'user'
	var user models.User
	if err := config.DB.Where("id = ?", userID).First(&user).Error; err == nil {
		if user.Role == models.RoleUser {
			config.DB.Model(&user).Update("role", models.RoleSeller)
		}
	}

	// 7. Preload data pemilik toko (User) untuk menyajikan respons lengkap
	config.DB.Preload("User").First(&newStore, newStore.ID)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Pendaftaran toko baru berhasil dilakukan. Fitur seller telah diaktifkan.",
		"data":    newStore,
	})
}

// GetMyStore mengambil rincian data toko milik pengguna yang sedang terautentikasi (login).
//
// HTTP Endpoint : GET /api/v1/stores/my-store
// Hak Akses     : Authenticated User / Seller
func GetMyStore(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	userID := userIDVal.(uuid.UUID)

	var store models.Store
	if err := config.DB.Preload("User").Where("user_id = ?", userID).First(&store).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Anda belum mendaftarkan toko.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat membaca data toko dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil rincian profil toko.",
		"data":    store,
	})
}

// UpdateStore memfasilitasi pembaruan informasi profil toko milik Seller.
//
// HTTP Endpoint : PUT /api/v1/stores
// Hak Akses     : Authenticated User (Pemilik Toko / Seller)
func UpdateStore(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	userID := userIDVal.(uuid.UUID)

	var store models.Store
	if err := config.DB.Where("user_id = ?", userID).First(&store).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Toko tidak ditemukan. Silakan daftarkan toko terlebih dahulu.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat membaca data toko.",
			"error":   err.Error(),
		})
		return
	}

	var input UpdateStoreInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format data pembaruan toko tidak valid.",
			"error":   err.Error(),
		})
		return
	}

	if strings.TrimSpace(input.StoreName) != "" {
		store.StoreName = strings.TrimSpace(input.StoreName)
	}
	if input.StoreDescription != "" {
		store.StoreDescription = strings.TrimSpace(input.StoreDescription)
	}
	if strings.TrimSpace(input.Address) != "" {
		store.Address = strings.TrimSpace(input.Address)
	}

	if err := config.DB.Save(&store).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal memperbarui informasi toko ke basis data.",
			"error":   err.Error(),
		})
		return
	}

	config.DB.Preload("User").First(&store, store.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Informasi toko berhasil diperbarui.",
		"data":    store,
	})
}

// CreateProduct mengeksekusi penambahan produk daur ulang baru ke dalam katalog toko Seller.
// Validasi kategori memastikan kategori produk tergolong dalam 10 kategori sampah resmi.
//
// HTTP Endpoint : POST /api/v1/marketplace/products
// Hak Akses     : Authenticated User (Seller / Admin)
func CreateProduct(c *gin.Context) {
	// 1. Membaca identitas pengguna (User ID) dari konteks otentikasi JWT Middleware
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	userID := userIDVal.(uuid.UUID)

	// 2. Memastikan pengguna sudah memiliki toko terdaftar
	var store models.Store
	if err := config.DB.Where("user_id = ?", userID).First(&store).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Akses ditolak. Anda harus mendaftarkan toko terlebih dahulu untuk menambahkan produk.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat memverifikasi toko penjual.",
			"error":   err.Error(),
		})
		return
	}

	// 3. Validasi format skema payload HTTP Request Body
	var input CreateProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format data masukan tidak valid. Pastikan nama, harga, stok, dan kategori telah diisi.",
			"error":   err.Error(),
		})
		return
	}

	// 4. Validasi kesesuaian kategori produk dengan 10 kategori sampah resmi EcoPlan
	categoryLower := strings.ToLower(strings.TrimSpace(input.Category))
	if !validProductCategories[categoryLower] {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Kategori '%s' tidak valid. Gunakan salah satu dari 10 kategori resmi: battery, biological, cardboard, clothes, glass, metal, paper, plastic, shoes, trash.", input.Category),
			"valid_categories": []string{
				"battery", "biological", "cardboard", "clothes", "glass", "metal", "paper", "plastic", "shoes", "trash",
			},
		})
		return
	}

	// 5. Instansiasi objek model Product
	newProduct := models.Product{
		StoreID:     store.ID,
		Name:        strings.TrimSpace(input.Name),
		Description: strings.TrimSpace(input.Description),
		Price:       input.Price,
		Stock:       input.Stock,
		Category:    categoryLower,
	}

	// 6. Menyimpan entitas produk baru ke basis data PostgreSQL/Supabase
	if err := config.DB.Create(&newProduct).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menyimpan produk baru ke basis data.",
			"error":   err.Error(),
		})
		return
	}

	// 7. Preload data Toko (Store) untuk penyajian respons lengkap
	config.DB.Preload("Store").First(&newProduct, newProduct.ID)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Berhasil menambahkan produk daur ulang baru ke dalam katalog toko.",
		"data":    newProduct,
	})
}

// GetAllProducts mengambil seluruh daftar produk yang tersedia di marketplace EcoPlan.
// Fitur ini mendukung pencarian kata kunci, filter kategori spesifik, dan paginasi data.
//
// HTTP Endpoint : GET /api/v1/marketplace/products
// Hak Akses     : Publik / Authenticated User
func GetAllProducts(c *gin.Context) {
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

	query := config.DB.Model(&models.Product{})

	// 1. Menambahkan filter berdasarkan kategori spesifik jika dicantumkan
	if categoryQuery != "" {
		if !validProductCategories[categoryQuery] {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("Kategori '%s' tidak valid. Gunakan salah satu dari 10 kategori resmi.", categoryQuery),
			})
			return
		}
		query = query.Where("category = ?", categoryQuery)
	}

	// 2. Menambahkan pencarian berbasis kata kunci pada nama atau deskripsi produk
	if strings.TrimSpace(searchQuery) != "" {
		searchTerm := "%" + strings.TrimSpace(searchQuery) + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", searchTerm, searchTerm)
	}

	// 3. Hitung total akumulasi rekam data produk
	var totalRecords int64
	query.Count(&totalRecords)

	// 4. Mengambil daftar produk dengan paginasi dan relasi data Toko (Store)
	var products []models.Product
	err := query.Preload("Store").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&products).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil daftar produk dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil daftar produk marketplace.",
		"data": gin.H{
			"products":      products,
			"total_records": totalRecords,
			"page":          page,
			"limit":         limit,
			"total_pages":   (totalRecords + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetProductByID mengambil rincian detail produk daur ulang berdasarkan UUID Identifier.
//
// HTTP Endpoint : GET /api/v1/marketplace/products/:id
// Hak Akses     : Publik / Authenticated User
func GetProductByID(c *gin.Context) {
	productIDStr := c.Param("id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format ID produk tidak valid. Harap gunakan format UUID.",
		})
		return
	}

	var product models.Product
	if err := config.DB.Preload("Store.User").Where("id = ?", productID).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Produk tidak ditemukan.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat membaca data produk dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil rincian detail produk.",
		"data":    product,
	})
}

// GetMyStoreProducts mengambil seluruh daftar produk yang dimiliki oleh toko Seller yang sedang terautentikasi.
//
// HTTP Endpoint : GET /api/v1/stores/my-products
// Hak Akses     : Authenticated User (Seller / Merchant)
func GetMyStoreProducts(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	userID := userIDVal.(uuid.UUID)

	var store models.Store
	if err := config.DB.Where("user_id = ?", userID).First(&store).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Toko tidak ditemukan.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat memverifikasi toko penjual.",
			"error":   err.Error(),
		})
		return
	}

	var products []models.Product
	err := config.DB.Where("store_id = ?", store.ID).
		Order("created_at DESC").
		Find(&products).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil daftar produk toko dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil seluruh produk milik toko Anda.",
		"data": gin.H{
			"store_id":    store.ID,
			"store_name":  store.StoreName,
			"total_count": len(products),
			"products":    products,
		},
	})
}

// UpdateProduct memfasilitasi pembaruan informasi produk (harga, stok, atau deskripsi) oleh Seller pemilik toko.
//
// HTTP Endpoint : PUT /api/v1/marketplace/products/:id
// Hak Akses     : Authenticated User (Seller Pemilik Produk)
func UpdateProduct(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	userID := userIDVal.(uuid.UUID)

	productIDStr := c.Param("id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format ID produk tidak valid. Harap gunakan format UUID.",
		})
		return
	}

	// 1. Memeriksa keberadaan toko milik pengguna yang login
	var store models.Store
	if err := config.DB.Where("user_id = ?", userID).First(&store).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Akses ditolak. Anda tidak memiliki akses toko yang valid.",
		})
		return
	}

	// 2. Membaca entitas produk dan memvalidasi kepemilikan toko (Ownership Verification)
	var product models.Product
	if err := config.DB.Where("id = ?", productID).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Produk tidak ditemukan.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat membaca data produk.",
			"error":   err.Error(),
		})
		return
	}

	if product.StoreID != store.ID {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Akses ditolak. Anda tidak memiliki wewenang untuk mengubah produk milik toko lain.",
		})
		return
	}

	// 3. Validasi payload pembaruan produk
	var input UpdateProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format data pembaruan produk tidak valid.",
			"error":   err.Error(),
		})
		return
	}

	// 4. Melakukan pembaruan atribut produk jika diberikan pada payload
	if strings.TrimSpace(input.Name) != "" {
		product.Name = strings.TrimSpace(input.Name)
	}
	if input.Description != "" {
		product.Description = strings.TrimSpace(input.Description)
	}
	if input.Price != nil {
		if *input.Price < 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Harga produk tidak boleh bernilai negatif.",
			})
			return
		}
		product.Price = *input.Price
	}
	if input.Stock != nil {
		if *input.Stock < 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Stok produk tidak boleh bernilai negatif.",
			})
			return
		}
		product.Stock = *input.Stock
	}
	if strings.TrimSpace(input.Category) != "" {
		categoryLower := strings.ToLower(strings.TrimSpace(input.Category))
		if !validProductCategories[categoryLower] {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("Kategori '%s' tidak valid. Gunakan salah satu dari 10 kategori resmi.", input.Category),
			})
			return
		}
		product.Category = categoryLower
	}

	// 5. Menyimpan pembaruan ke basis data PostgreSQL/Supabase
	if err := config.DB.Save(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal memperbarui data produk ke basis data.",
			"error":   err.Error(),
		})
		return
	}

	config.DB.Preload("Store").First(&product, product.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Informasi produk berhasil diperbarui.",
		"data":    product,
	})
}

// DeleteProduct menghapus rekam data produk dari katalog marketplace.
//
// HTTP Endpoint : DELETE /api/v1/marketplace/products/:id
// Hak Akses     : Authenticated User (Seller Pemilik Produk atau Admin)
func DeleteProduct(c *gin.Context) {
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

	productIDStr := c.Param("id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format ID produk tidak valid. Harap gunakan format UUID.",
		})
		return
	}

	// 1. Membaca entitas produk dari basis data
	var product models.Product
	if err := config.DB.Where("id = ?", productID).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Produk tidak ditemukan.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat membaca data produk.",
			"error":   err.Error(),
		})
		return
	}

	// 2. Memeriksa wewenang penghapusan (Hanya Seller Pemilik Toko atau Admin)
	if userRole != models.RoleAdmin {
		var store models.Store
		if err := config.DB.Where("user_id = ?", userID).First(&store).Error; err != nil || product.StoreID != store.ID {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Akses ditolak. Anda tidak memiliki wewenang untuk menghapus produk ini.",
			})
			return
		}
	}

	// 3. Eksekusi operasi penghapusan entitas produk dari basis data
	if err := config.DB.Delete(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menghapus produk dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Produk berhasil dihapus dari marketplace.",
	})
}
