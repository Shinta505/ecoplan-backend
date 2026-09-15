package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"ecoplan-backend/services"

	"github.com/gin-gonic/gin"
)

// LogisticsController mengendalikan antarmuka HTTP untuk fungsi logistik terisolasi,
// mencakup pencarian entitas wilayah, kalkulasi estimasi ongkos kirim multi-ekspedisi, dan pelacakan resi.
type LogisticsController struct {
	rajaOngkirService *services.RajaOngkirService
}

// NewLogisticsController menginstansiasi objek LogisticsController dengan injeksi dependensi RajaOngkirService.
func NewLogisticsController() *LogisticsController {
	return &LogisticsController{
		rajaOngkirService: services.NewRajaOngkirService(),
	}
}

// CalculateShippingCostInput mendefinisikan skema Data Transfer Object (DTO) untuk masukan kalkulasi ongkos kirim.
type CalculateShippingCostInput struct {
	Origin      int    `json:"origin" binding:"required,gt=0"`
	Destination int    `json:"destination" binding:"required,gt=0"`
	Weight      int    `json:"weight" binding:"required,gt=0"`
	Courier     string `json:"courier" binding:"required"`
	Price       string `json:"price"`
}

// SearchDestination mengeksekusi pencarian lokasi wilayah domestik (kabupaten/kota/kecamatan/kelurahan)
// melalui pemanggilan API RajaOngkir untuk mendukung antarmuka pencarian otomatis (autocomplete) pada frontend.
//
// HTTP Endpoint : GET /api/v1/logistics/destinations
// Hak Akses     : Publik / Authenticated User
// SearchDestination mengeksekusi pencarian lokasi wilayah domestik
func (lc *LogisticsController) SearchDestination(c *gin.Context) {
	search := c.Query("search")
	limitStr := c.Query("limit")

	limit := 5
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	// Gunakan pengecekan manual untuk memastikan service terinisialisasi
	svc := services.NewRajaOngkirService()
	if svc == nil {
		c.String(http.StatusInternalServerError, "FATAL: Gagal menginisialisasi RajaOngkirService")
		return
	}

	result, err := svc.SearchDomesticDestination(search, limit, 0)
	if err != nil {
		c.String(http.StatusInternalServerError, "ERROR DARI RAJAONGKIR SERVICE: %s", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// CalculateShippingCost mengeksekusi kalkulasi estimasi biaya pengiriman paket secara real-time
// berdasarkan parameter lokasi asal, lokasi tujuan, berat barang (gram), dan kode kurir ekspedisi.
//
// HTTP Endpoint : POST /api/v1/logistics/shipping-cost
// Hak Akses     : Publik / Authenticated User
func (lc *LogisticsController) CalculateShippingCost(c *gin.Context) {
	var input CalculateShippingCostInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Skema data masukan tidak valid. Pastikan origin, destination, weight (>0), dan courier terisi.",
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

	result, err := lc.rajaOngkirService.CalculateDomesticCost(costReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Gagal menghitung estimasi ongkos kirim melalui layanan RajaOngkir.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil menghitung estimasi ongkos kirim pengiriman.",
		"data":    result.Data,
	})
}

// TrackWaybill mengeksekusi pelacakan rekam jejak status pengiriman resi (Airway Bill / AWB) secara real-time.
//
// HTTP Endpoint : GET /api/v1/logistics/tracking
// Hak Akses     : Publik / Authenticated User
func (lc *LogisticsController) TrackWaybill(c *gin.Context) {
	resiQuery := c.Query("resi")
	if strings.TrimSpace(resiQuery) == "" {
		resiQuery = c.Query("awb")
	}

	if strings.TrimSpace(resiQuery) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Parameter nomor resi pengiriman ('resi' atau 'awb') wajib diisi.",
		})
		return
	}

	courierQuery := c.Query("courier")

	result, err := lc.rajaOngkirService.TrackWaybill(resiQuery, courierQuery)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Gagal melacak nomor resi pengiriman melalui API RajaOngkir.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil memperoleh informasi status pelacakan resi pengiriman.",
		"data":    result.Data,
	})
}

// Handler fungsi tunggal untuk kompatibilitas pemanggilan routing fungsional Gin.
func GetShippingCost(c *gin.Context) {
	ctrl := NewLogisticsController()
	ctrl.CalculateShippingCost(c)
}

func TrackShipment(c *gin.Context) {
	ctrl := NewLogisticsController()
	ctrl.TrackWaybill(c)
}
