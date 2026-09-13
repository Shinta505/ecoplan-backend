package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"ecoplan-backend/config"
	"ecoplan-backend/models"
	"ecoplan-backend/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CheckoutItemInput mendefinisikan struktur DTO untuk setiap item produk yang dibeli.
type CheckoutItemInput struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,gt=0"`
}

// CreateTransactionInput mendefinisikan skema payload DTO untuk eksekusi checkout pesanan escrow.
type CreateTransactionInput struct {
	Items        []CheckoutItemInput `json:"items" binding:"required,dive"`
	ShippingCost float64             `json:"shipping_cost" binding:"gte=0"`
}

// UpdateTransactionStatusInput mendefinisikan DTO untuk mengubah status alur transaksi escrow.
type UpdateTransactionStatusInput struct {
	Status models.TransactionStatus `json:"status" binding:"required"`
}

// CreateTransaction mengeksekusi alur checkout transaksi Rekening Bersama (Escrow).
// Fungsi ini menghitung total belanja, mengurangi stok produk, mencatat entitas transaksi,
// dan menerbitkan Snap Transaction Token dari Midtrans.
//
// HTTP Endpoint : POST /api/v1/transactions
// Hak Akses     : Authenticated User (Buyer)
func CreateTransaction(c *gin.Context) {
	// 1. Membaca identitas pembeli dari konteks otentikasi JWT Middleware
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	buyerID, ok := userIDVal.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengonversi identitas pembeli (User ID).",
		})
		return
	}

	buyerEmail, _ := c.Get("user_email")
	buyerEmailStr, _ := buyerEmail.(string)

	// 2. Validasi format skema payload HTTP JSON Request
	var input CreateTransactionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format data checkout tidak valid. Pastikan items dan shipping_cost terisi dengan benar.",
			"error":   err.Error(),
		})
		return
	}

	if len(input.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Daftar item belanjaan tidak boleh kosong.",
		})
		return
	}

	// 3. Membaca data lengkap pembeli dari basis data
	var buyer models.User
	if err := config.DB.Where("id = ?", buyerID).First(&buyer).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Data pembeli tidak ditemukan dalam sistem.",
		})
		return
	}

	// 4. Memulai Transaksi Basis Data (DB Transaction) untuk menjaga konsistensi persediaan stok
	dbTx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			dbTx.Rollback()
		}
	}()

	var transactionItems []models.TransactionItem
	var subtotal float64 = 0.0

	// 5. Verifikasi ketersediaan stok produk dan kalkulasi akumulasi subtotal
	for _, itemInput := range input.Items {
		prodUUID, err := uuid.Parse(itemInput.ProductID)
		if err != nil {
			dbTx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("Format Product ID '%s' tidak valid.", itemInput.ProductID),
			})
			return
		}

		var product models.Product
		if err := dbTx.Where("id = ?", prodUUID).First(&product).Error; err != nil {
			dbTx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"message": fmt.Sprintf("Produk dengan ID '%s' tidak ditemukan.", itemInput.ProductID),
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Gagal membaca data produk dari basis data.",
			})
			return
		}

		// Memastikan ketersediaan stok
		if product.Stock < itemInput.Quantity {
			dbTx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("Stok produk '%s' tidak mencukupi (Tersedia: %d, Diminta: %d).", product.Name, product.Stock, itemInput.Quantity),
			})
			return
		}

		// Pengurangan stok secara bersyarat
		product.Stock -= itemInput.Quantity
		if err := dbTx.Save(&product).Error; err != nil {
			dbTx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Gagal memperbarui stok persediaan produk.",
			})
			return
		}

		itemPrice := product.Price
		itemTotal := itemPrice * float64(itemInput.Quantity)
		subtotal += itemTotal

		txItem := models.TransactionItem{
			ProductID: prodUUID,
			Quantity:  itemInput.Quantity,
			Price:     itemPrice,
			Product:   &product,
		}
		transactionItems = append(transactionItems, txItem)
	}

	totalAmount := subtotal + input.ShippingCost
	midtransOrderID := fmt.Sprintf("ECO-%d-%s", time.Now().Unix(), buyerID.String()[:8])

	// 6. Menyimpan entitas transaksi utama pada tabel transactions
	transaction := models.Transaction{
		BuyerID:         buyerID,
		TotalAmount:     totalAmount,
		ShippingCost:    input.ShippingCost,
		Status:          models.StatusPendingPayment,
		MidtransOrderID: midtransOrderID,
		Items:           transactionItems,
	}

	if err := dbTx.Create(&transaction).Error; err != nil {
		dbTx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mencatat transaksi baru ke basis data.",
			"error":   err.Error(),
		})
		return
	}

	// 7. Menerbitkan Token Pembayaran Midtrans Snap API via MidtransService
	midtransSvc := services.NewMidtransService(dbTx)
	snapResp, err := midtransSvc.CreateSnapTransaction(&transaction, buyer.Name, buyerEmailStr)
	if err != nil {
		dbTx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menghasilkan Snap Token pembayaran Midtrans.",
			"error":   err.Error(),
		})
		return
	}

	// Commit seluruh perubahan basis data jika berhasil
	if err := dbTx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menyelesaikan komit transaksi basis data.",
		})
		return
	}

	// Preload relasi untuk penyajian respons utuh
	config.DB.Preload("Items.Product").Preload("Buyer").First(&transaction, transaction.ID)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Berhasil membuat pesanan transaksi escrow. Silakan selesaikan pembayaran.",
		"data": gin.H{
			"transaction":       transaction,
			"snap_token":        snapResp.Token,
			"snap_redirect_url": snapResp.RedirectURL,
		},
	})
}

// GetAllTransactions mengambil daftar riwayat transaksi.
// Pengguna umum hanya dapat melihat transaksi miliknya (sebagai pembeli),
// sedangkan Admin berwenang memantau seluruh transaksi platform.
//
// HTTP Endpoint : GET /api/v1/transactions
// Hak Akses     : Authenticated User / Admin
func GetAllTransactions(c *gin.Context) {
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

	query := config.DB.Model(&models.Transaction{})

	// Pembatasan hak akses berbasis peranan (RBAC)
	if userRole != models.RoleAdmin {
		query = query.Where("buyer_id = ?", userID)
	}

	if statusQuery != "" {
		query = query.Where("status = ?", statusQuery)
	}

	var totalRecords int64
	query.Count(&totalRecords)

	var transactions []models.Transaction
	err := query.Preload("Buyer").
		Preload("Items.Product.Store").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&transactions).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil daftar transaksi dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil daftar riwayat transaksi.",
		"data": gin.H{
			"transactions":  transactions,
			"total_records": totalRecords,
			"page":          page,
			"limit":         limit,
			"total_pages":   (totalRecords + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetTransactionByID mengambil rincian detail transaksi berdasarkan UUID Identifier.
//
// HTTP Endpoint : GET /api/v1/transactions/:id
// Hak Akses     : Authenticated User (Pembeli / Penjual Terkait / Admin)
func GetTransactionByID(c *gin.Context) {
	txIDStr := c.Param("id")
	txID, err := uuid.Parse(txIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format ID Transaksi tidak valid. Harap gunakan format UUID.",
		})
		return
	}

	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Sesi otentikasi tidak ditemukan.",
		})
		return
	}

	userID := userIDVal.(uuid.UUID)
	userRoleVal, _ := c.Get("user_role")
	userRole, _ := userRoleVal.(models.UserRole)

	var transaction models.Transaction
	err = config.DB.Preload("Buyer").
		Preload("Items.Product.Store").
		Where("id = ?", txID).
		First(&transaction).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Data transaksi tidak ditemukan.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat membaca data transaksi dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	// Verifikasi hak akses pembacaan transaksi
	if userRole != models.RoleAdmin && transaction.BuyerID != userID {
		// Memeriksa apakah pengguna bertindak sebagai penjual (Seller) dari salah satu item dalam transaksi
		isSeller := false
		var userStore models.Store
		if err := config.DB.Where("user_id = ?", userID).First(&userStore).Error; err == nil {
			for _, item := range transaction.Items {
				if item.Product != nil && item.Product.StoreID == userStore.ID {
					isSeller = true
					break
				}
			}
		}

		if !isSeller {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Akses ditolak. Anda tidak memiliki wewenang untuk melihat rincian transaksi ini.",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil rincian detail transaksi.",
		"data":    transaction,
	})
}

// UpdateTransactionStatus memfasilitasi pembaruan alur transaksi Rekening Bersama (Escrow).
// Ketika status diubah menjadi 'completed' oleh pembeli atau admin, dana hasil penjualan
// secara otomatis dicairkan ke saldo toko penjual (Seller Balance).
//
// HTTP Endpoint : PATCH /api/v1/transactions/:id/status
// Hak Akses     : Authenticated User (Pembeli / Penjual / Admin)
func UpdateTransactionStatus(c *gin.Context) {
	txIDStr := c.Param("id")
	txID, err := uuid.Parse(txIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format ID Transaksi tidak valid. Harap gunakan format UUID.",
		})
		return
	}

	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Sesi otentikasi tidak ditemukan.",
		})
		return
	}

	userID := userIDVal.(uuid.UUID)
	userRoleVal, _ := c.Get("user_role")
	userRole, _ := userRoleVal.(models.UserRole)

	var input UpdateTransactionStatusInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format payload perubahan status tidak valid.",
			"error":   err.Error(),
		})
		return
	}

	// 1. Membaca entitas transaksi lengkap dengan relasi item dan toko penjual
	var transaction models.Transaction
	if err := config.DB.Preload("Items.Product.Store").Where("id = ?", txID).First(&transaction).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Data transaksi tidak ditemukan.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat membaca transaksi dari basis data.",
		})
		return
	}

	// 2. Memvalidasi transisi alur status transaksi
	validStatuses := map[models.TransactionStatus]bool{
		models.StatusPendingPayment: true,
		models.StatusPaidEscrow:     true,
		models.StatusShipped:        true,
		models.StatusCompleted:      true,
		models.StatusCancelled:      true,
	}

	if !validStatuses[input.Status] {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Nilai status '%s' tidak valid.", input.Status),
		})
		return
	}

	// Pembatasan hak akses transisi status
	switch input.Status {
	case models.StatusCompleted:
		// Konfirmasi pesanan diterima ('completed') umumnya dilakukan oleh Pembeli atau Admin
		if transaction.BuyerID != userID && userRole != models.RoleAdmin {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Hanya pembeli atau administrator yang dapat mengonfirmasi transaksi selesai.",
			})
			return
		}
	case models.StatusShipped:
		// Perubahan status menjadi 'shipped' dilakukan oleh Penjual atau Admin
		if userRole != models.RoleAdmin {
			var sellerStore models.Store
			if err := config.DB.Where("user_id = ?", userID).First(&sellerStore).Error; err != nil {
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"message": "Akses ditolak. Anda bukan pemilik toko terdaftar.",
				})
				return
			}
		}
	}

	// 3. Memproses pencairan saldo toko penjual saat transaksi diselesaikan ('completed')
	if transaction.Status != models.StatusCompleted && input.Status == models.StatusCompleted {
		dbTx := config.DB.Begin()

		// Mengelompokkan total pendapatan per toko penjual
		storeEarnings := make(map[uuid.UUID]float64)
		for _, item := range transaction.Items {
			if item.Product != nil && item.Product.StoreID != uuid.Nil {
				itemTotal := item.Price * float64(item.Quantity)
				storeEarnings[item.Product.StoreID] += itemTotal
			}
		}

		// Mencairkan dana ke saldo akun pemilik toko (Seller Balance)
		for storeID, earnings := range storeEarnings {
			var store models.Store
			if err := dbTx.Where("id = ?", storeID).First(&store).Error; err == nil {
				if err := dbTx.Model(&models.User{}).
					Where("id = ?", store.UserID).
					UpdateColumn("balance", gorm.Expr("balance + ?", earnings)).Error; err != nil {
					dbTx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{
						"success": false,
						"message": "Gagal mencairkan saldo hasil penjualan ke rekening penjual.",
						"error":   err.Error(),
					})
					return
				}
			}
		}

		transaction.Status = models.StatusCompleted
		if err := dbTx.Save(&transaction).Error; err != nil {
			dbTx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Gagal memperbarui status transaksi menjadi completed.",
			})
			return
		}

		// Memberikan Eco-Points apresiasi transaksi ramah lingkungan kepada pembeli
		dbTx.Model(&models.User{}).
			Where("id = ?", transaction.BuyerID).
			UpdateColumn("eco_points", gorm.Expr("eco_points + ?", 25))

		if err := dbTx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Gagal mengomit transaksi pencairan saldo basis data.",
			})
			return
		}

		config.DB.Preload("Items.Product.Store").Preload("Buyer").First(&transaction, transaction.ID)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Transaksi berhasil dikonfirmasi selesai. Dana Rekening Bersama (Escrow) telah dicairkan ke saldo toko penjual.",
			"data":    transaction,
		})
		return
	}

	// 4. Pengembalian stok jika transaksi dibatalkan ('cancelled')
	if transaction.Status != models.StatusCancelled && input.Status == models.StatusCancelled {
		dbTx := config.DB.Begin()
		for _, item := range transaction.Items {
			if err := dbTx.Model(&models.Product{}).
				Where("id = ?", item.ProductID).
				UpdateColumn("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
				dbTx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "Gagal mengembalikan persediaan stok produk.",
				})
				return
			}
		}

		transaction.Status = models.StatusCancelled
		if err := dbTx.Save(&transaction).Error; err != nil {
			dbTx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Gagal memperbarui status pembatalan transaksi.",
			})
			return
		}

		dbTx.Commit()
		config.DB.Preload("Items.Product.Store").Preload("Buyer").First(&transaction, transaction.ID)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Transaksi berhasil dibatalkan dan stok produk telah dikembalikan.",
			"data":    transaction,
		})
		return
	}

	// Pembaruan status standar (misalnya: paid_escrow -> shipped)
	transaction.Status = input.Status
	if err := config.DB.Save(&transaction).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal memperbarui status transaksi pada basis data.",
			"error":   err.Error(),
		})
		return
	}

	config.DB.Preload("Items.Product.Store").Preload("Buyer").First(&transaction, transaction.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Status transaksi berhasil diperbarui menjadi '%s'.", transaction.Status),
		"data":    transaction,
	})
}
