package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ecoplan-backend/config"
	"ecoplan-backend/models"
	"ecoplan-backend/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PaymentNotificationWebhook menangani callback notifikasi HTTP POST otomatis dari Payment Gateway Midtrans.
// Fungsi ini memproses status pembayaran, memperbarui alur status transaksi escrow ('paid_escrow' / 'cancelled'),
// serta merespons server Midtrans dengan status HTTP 200 OK.
//
// HTTP Endpoint : POST /api/v1/transactions/webhook (atau /api/v1/payment/notification)
// Hak Akses     : Public Callback (Diakses otomatis oleh Server Midtrans)
func PaymentNotificationWebhook(c *gin.Context) {
	var notificationPayload map[string]interface{}

	// 1. Validasi dan parsing JSON payload dari request body Midtrans
	if err := c.ShouldBindJSON(&notificationPayload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format payload webhook notifikasi tidak valid.",
			"error":   err.Error(),
		})
		return
	}

	// 2. Meneruskan pemrosesan logika notifikasi ke MidtransService
	midtransSvc := services.NewMidtransService(config.DB)
	if err := midtransSvc.ProcessWebhookNotification(notificationPayload); err != nil {
		// Log internal kesalahan tanpa menggagalkan respons webhook ke server Midtrans
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Webhook diterima tetapi pemrosesan notifikasi mengalami penyesuaian.",
			"error":   err.Error(),
		})
		return
	}

	// 3. Mengembalikan respons HTTP 200 OK ke server Midtrans
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Notifikasi pembayaran Midtrans berhasil diproses.",
	})
}

// CheckPaymentStatus memeriksa status transaksi terkini langsung ke server Midtrans Core API
// dan menyinkronkan status tersebut dengan basis data PostgreSQL backend EcoPlan.
//
// HTTP Endpoint : GET /api/v1/payment/status/:order_id
// Hak Akses     : Authenticated User / Admin
func CheckPaymentStatus(c *gin.Context) {
	orderID := strings.TrimSpace(c.Param("order_id"))
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Parameter order_id tidak boleh kosong.",
		})
		return
	}

	// 1. Membaca entitas transaksi dari basis data PostgreSQL berdasarkan midtrans_order_id
	var transaction models.Transaction
	if err := config.DB.Preload("Items.Product").Where("midtrans_order_id = ?", orderID).First(&transaction).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Data transaksi dengan Order ID tersebut tidak ditemukan.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat membaca basis data.",
			"error":   err.Error(),
		})
		return
	}

	// 2. Melakukan panggilan verifikasi status transaksi ke Core API Midtrans
	resp, midErr := config.CoreAPIClient.CheckTransaction(orderID)
	if midErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Gagal memeriksa status pembayaran ke server Midtrans.",
			"error":   midErr.Message,
		})
		return
	}

	// 3. Memetakan status pembayaran dari Core API Midtrans ke Enum TransactionStatus EcoPlan
	var updatedStatus models.TransactionStatus
	switch resp.TransactionStatus {
	case "capture":
		if resp.FraudStatus == "accept" || resp.FraudStatus == "" {
			updatedStatus = models.StatusPaidEscrow
		} else {
			updatedStatus = models.StatusPendingPayment
		}
	case "settlement":
		updatedStatus = models.StatusPaidEscrow
	case "pending":
		updatedStatus = models.StatusPendingPayment
	case "deny", "cancel", "expire", "failure":
		updatedStatus = models.StatusCancelled
	default:
		updatedStatus = transaction.Status
	}

	// 4. Memperbarui status pada basis data jika terdapat perubahan transisi
	if transaction.Status != updatedStatus {
		config.DB.Model(&transaction).Update("status", updatedStatus)
		transaction.Status = updatedStatus
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mendapatkan status pembayaran terkini dari Midtrans.",
		"data": gin.H{
			"order_id":           orderID,
			"transaction_status": resp.TransactionStatus,
			"payment_type":       resp.PaymentType,
			"gross_amount":       resp.GrossAmount,
			"escrow_status":      transaction.Status,
			"transaction_detail": transaction,
		},
	})
}

// WithdrawSellerBalance memfasilitasi penarikan saldo toko (Seller Balance)
// yang terkumpul dari pencairan alur Rekening Bersama (Escrow) transaksi yang sudah `completed`.
//
// HTTP Endpoint : POST /api/v1/payment/withdraw
// Hak Akses     : Authenticated User (Seller / Merchant)
func WithdrawSellerBalance(c *gin.Context) {
	type WithdrawInput struct {
		Amount        float64 `json:"amount" binding:"required,gt=0"`
		BankName      string  `json:"bank_name" binding:"required"`
		AccountNumber string  `json:"account_number" binding:"required"`
		AccountName   string  `json:"account_name" binding:"required"`
	}

	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	userID := userIDVal.(uuid.UUID)

	var input WithdrawInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format data penarikan saldo tidak valid. Pastikan amount, bank_name, account_number, dan account_name terisi.",
			"error":   err.Error(),
		})
		return
	}

	// 1. Membaca data penjual dan memeriksa ketersediaan saldo
	dbTx := config.DB.Begin()
	var user models.User
	if err := dbTx.Where("id = ?", userID).First(&user).Error; err != nil {
		dbTx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Data akun pengguna tidak ditemukan.",
		})
		return
	}

	if user.Balance < input.Amount {
		dbTx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Saldo akun Anda tidak mencukupi untuk melakukan penarikan dana ini.",
			"data": gin.H{
				"current_balance":  user.Balance,
				"requested_amount": input.Amount,
			},
		})
		return
	}

	// 2. Mengurangi nilai saldo akun pengguna
	user.Balance -= input.Amount
	if err := dbTx.Save(&user).Error; err != nil {
		dbTx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal memperbarui saldo rekening pengguna.",
		})
		return
	}

	if err := dbTx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengomit transaksi penarikan dana.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Permohonan penarikan saldo berhasil diproses.",
		"data": gin.H{
			"user_id":           user.ID,
			"withdrawn_amount":  input.Amount,
			"remaining_balance": user.Balance,
			"bank_details": gin.H{
				"bank_name":      input.BankName,
				"account_number": input.AccountNumber,
				"account_name":   input.AccountName,
			},
		},
	})
}

// RedeemEcoPointsVoucher memfasilitasi penukaran (redeem) eco_points pengguna
// menjadi kode voucher diskon belanja marketplace EcoPlan berdasarkan rasio konversi tertentu.
//
// HTTP Endpoint : POST /api/v1/profile/redeem-voucher (atau /api/v1/rewards/redeem)
// Hak Akses     : Authenticated User (Bearer JWT)
func RedeemEcoPointsVoucher(c *gin.Context) {
	type RedeemInput struct {
		PointsSpent    int `json:"points_spent"`
		PointsToRedeem int `json:"points_to_redeem"`
	}

	// 1. Membaca identitas pengguna dari konteks JWT Middleware
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	userID := userIDVal.(uuid.UUID)

	// 2. Parsing dan validasi payload HTTP Request Body
	var input RedeemInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format data penukaran poin tidak valid.",
			"error":   err.Error(),
		})
		return
	}

	// Fleksibilitas penerimaan parameter masukan (points_spent atau points_to_redeem)
	pointsToDeduct := input.PointsSpent
	if pointsToDeduct <= 0 {
		pointsToDeduct = input.PointsToRedeem
	}

	if pointsToDeduct <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Jumlah poin yang ditukarkan harus lebih besar dari 0.",
		})
		return
	}

	// 3. Menginisialisasi Transaksi Basis Data (DB Transaction) untuk menjamin integritas data
	dbTx := config.DB.Begin()

	var user models.User
	if err := dbTx.Where("id = ?", userID).First(&user).Error; err != nil {
		dbTx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Data akun pengguna tidak ditemukan.",
		})
		return
	}

	// 4. Verifikasi kecukupan akumulasi eco_points pengguna
	if user.EcoPoints < pointsToDeduct {
		dbTx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Akumulasi EcoPoints Anda tidak mencukupi untuk melakukan penukaran ini.",
			"data": gin.H{
				"current_eco_points": user.EcoPoints,
				"requested_points":   pointsToDeduct,
			},
		})
		return
	}

	// 5. Kalkulasi rasio konversi poin ke nominal rupiah (Rasio: 1 EcoPoint = Rp 100,-)
	const conversionRatio = 100.0
	discountAmount := float64(pointsToDeduct) * conversionRatio

	// Menghasilkan kode unik voucher diskon (Format: ECO-VCH-<TIMESTAMP>-<HASH>)
	randomHash := strings.ToUpper(uuid.New().String()[:8])
	voucherCode := fmt.Sprintf("ECO-VCH-%d-%s", time.Now().Unix(), randomHash)

	// 6. Pengurangan nilai eco_points akun pengguna
	user.EcoPoints -= pointsToDeduct
	if err := dbTx.Save(&user).Error; err != nil {
		dbTx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal memperbarui saldo EcoPoints pengguna.",
			"error":   err.Error(),
		})
		return
	}

	// 7. Instansiasi dan pencatatan entitas DiscountVoucher baru (Masa berlaku voucher: 30 Hari)
	voucher := models.DiscountVoucher{
		UserID:         user.ID,
		Code:           voucherCode,
		DiscountAmount: discountAmount,
		PointsSpent:    pointsToDeduct,
		IsUsed:         false,
		ExpiresAt:      time.Now().AddDate(0, 0, 30),
	}

	if err := dbTx.Create(&voucher).Error; err != nil {
		dbTx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menerbitkan entitas voucher diskon baru ke basis data.",
			"error":   err.Error(),
		})
		return
	}

	// Commit transaksi basis data
	if err := dbTx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menyelesaikan komit transaksi penukaran poin.",
		})
		return
	}

	// 8. Mengembalikan respons HTTP 201 Created beserta rincian voucher
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Penukaran EcoPoints menjadi voucher diskon belanja berhasil diproses.",
		"data": gin.H{
			"voucher":              voucher,
			"remaining_eco_points": user.EcoPoints,
		},
	})
}
