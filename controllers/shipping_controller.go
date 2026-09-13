package controllers

import (
	"errors"
	"net/http"
	"strings"

	"ecoplan-backend/config"
	"ecoplan-backend/models"
	"ecoplan-backend/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ShippingController mengelola logika bisnis pengiriman yang terintegrasi langsung
// dengan data transaksi marketplace pada basis data PostgreSQL/Supabase.
type ShippingController struct {
	rajaOngkirService *services.RajaOngkirService
}

// NewShippingController menginstansiasi objek ShippingController.
func NewShippingController() *ShippingController {
	return &ShippingController{
		rajaOngkirService: services.NewRajaOngkirService(),
	}
}

// UpdateReceiptInput mendefinisikan DTO untuk pembaruan nomor resi transaksi oleh penjual/seller.
type UpdateReceiptInput struct {
	ReceiptNumber string `json:"receipt_number" binding:"required"`
}

// CalculateCheckoutShipping mengeksekusi kalkulasi biaya pengiriman khusus saat proses alur checkout pesanan.
//
// HTTP Endpoint : POST /api/v1/shipping/calculate
// Hak Akses     : Authenticated User (Member / Buyer)
func (sc *ShippingController) CalculateCheckoutShipping(c *gin.Context) {
	var input CalculateShippingCostInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Payload perhitungan ongkos kirim checkout tidak valid.",
			"error":   err.Error(),
		})
		return
	}

	costReq := services.DomesticCostRequest{
		Origin:      input.Origin,
		Destination: input.Destination,
		Weight:      input.Weight,
		Courier:     strings.TrimSpace(input.Courier),
		Price:       strings.TrimSpace(input.Price),
	}

	result, err := sc.rajaOngkirService.CalculateDomesticCost(costReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Gagal memperoleh opsi tarif pengiriman dari RajaOngkir.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil menghitung opsi tarif pengiriman checkout pesanan.",
		"data":    result.Data,
	})
}

// TrackTransactionShipment membaca data transaksi berdasarkan UUID ID Transaksi,
// mengambil nomor resi pengiriman yang terdaftar, lalu mengeksekusi pelacakan status resi real-time.
//
// HTTP Endpoint : GET /api/v1/shipping/tracking/:transaction_id
// Hak Akses     : Authenticated User (Pembeli / Penjual / Admin)
func (sc *ShippingController) TrackTransactionShipment(c *gin.Context) {
	txIDStr := c.Param("transaction_id")
	txID, err := uuid.Parse(txIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format ID Transaksi tidak valid. Harap gunakan format UUID.",
		})
		return
	}

	// 1. Membaca entitas transaksi dari basis data PostgreSQL
	var transaction models.Transaction
	if err := config.DB.Where("id = ?", txID).First(&transaction).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Data transaksi tidak ditemukan dalam sistem.",
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

	// 2. Memeriksa ketersediaan nomor resi pengiriman pada transaksi
	if strings.TrimSpace(transaction.ReceiptNumber) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Transaksi ini belum memiliki nomor resi pengiriman yang diinputkan oleh penjual.",
			"data": gin.H{
				"transaction_id": transaction.ID,
				"status":         transaction.Status,
			},
		})
		return
	}

	courierQuery := c.DefaultQuery("courier", "")

	// 3. Meneruskan permintaan pelacakan ke modul RajaOngkirService
	trackingResult, err := sc.rajaOngkirService.TrackWaybill(transaction.ReceiptNumber, courierQuery)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Gagal melacak nomor resi transaksi melalui ekspedisi RajaOngkir.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil melacak rekam jejak posisi pengiriman transaksi pesanan.",
		"data": gin.H{
			"transaction_id": transaction.ID,
			"receipt_number": transaction.ReceiptNumber,
			"status":         transaction.Status,
			"tracking_info":  trackingResult.Data,
		},
	})
}

// UpdateTransactionReceipt memfasilitasi pembaruan nomor resi oleh penjual (Seller)
// serta secara otomatis memperbarui status alur transaksi escrow menjadi 'shipped'.
//
// HTTP Endpoint : PATCH /api/v1/shipping/receipt/:transaction_id
// Hak Akses     : Authenticated User (Seller / Admin)
func (sc *ShippingController) UpdateTransactionReceipt(c *gin.Context) {
	txIDStr := c.Param("transaction_id")
	txID, err := uuid.Parse(txIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format ID Transaksi tidak valid. Harap gunakan format UUID.",
		})
		return
	}

	var input UpdateReceiptInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format data nomor resi tidak valid.",
			"error":   err.Error(),
		})
		return
	}

	// 1. Membaca entitas transaksi dari basis data
	var transaction models.Transaction
	if err := config.DB.Where("id = ?", txID).First(&transaction).Error; err != nil {
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
			"error":   err.Error(),
		})
		return
	}

	// 2. Mengubah nomor resi dan memperbarui status alur transaksi escrow menjadi 'shipped'
	transaction.ReceiptNumber = strings.TrimSpace(input.ReceiptNumber)
	if transaction.Status == models.StatusPaidEscrow {
		transaction.Status = models.StatusShipped
	}

	if err := config.DB.Save(&transaction).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengonfirmasi nomor resi pengiriman ke basis data.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Nomor resi pengiriman berhasil diperbarui dan status pesanan diubah menjadi 'shipped'.",
		"data":    transaction,
	})
}

// Handler fungsi tunggal untuk kompatibilitas pemanggilan routing fungsional Gin.
func CalculateShipping(c *gin.Context) {
	ctrl := NewShippingController()
	ctrl.CalculateCheckoutShipping(c)
}

func TrackOrder(c *gin.Context) {
	ctrl := NewShippingController()
	ctrl.TrackTransactionShipment(c)
}

func UpdateReceipt(c *gin.Context) {
	ctrl := NewShippingController()
	ctrl.UpdateTransactionReceipt(c)
}