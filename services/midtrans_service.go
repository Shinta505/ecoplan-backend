package services

import (
	"errors"
	"fmt"
	"log"

	"ecoplan-backend/config"
	"ecoplan-backend/models"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/snap"
	"gorm.io/gorm"
)

// MidtransService mendefinisikan antarmuka layanan bisnis untuk menangani integrasi Midtrans Snap API
// dan pemrosesan callback notifikasi webhook alur pembayaran rekening bersama (escrow) platform EcoPlan.
type MidtransService struct {
	db *gorm.DB
}

// NewMidtransService menginstansiasi objek MidtransService dengan injeksi dependensi instance basis data GORM.
func NewMidtransService(db *gorm.DB) *MidtransService {
	return &MidtransService{
		db: db,
	}
}

// CreateSnapTransaction menghasilkan Snap Transaction Token dan Redirect URL untuk transaksi escrow
// belanja produk daur ulang / sampah layak pakai di marketplace EcoPlan.
func (s *MidtransService) CreateSnapTransaction(tx *models.Transaction, buyerName, buyerEmail string) (*snap.Response, error) {
	if tx == nil {
		return nil, errors.New("objek data transaksi tidak boleh bernilai nil")
	}

	if tx.MidtransOrderID == "" {
		return nil, errors.New("midtrans_order_id transaksi tidak boleh kosong")
	}

	// 1. Menyusun daftar rincian barang (Item Details) untuk dikirimkan ke Midtrans Snap API
	var items []midtrans.ItemDetails

	for _, item := range tx.Items {
		itemName := "Produk Daur Ulang EcoPlan"
		if item.Product != nil && item.Product.Name != "" {
			itemName = item.Product.Name
		}

		// Membatasi panjang nama produk maksimal 50 karakter sesuai spesifikasi batas API Midtrans
		if len(itemName) > 50 {
			itemName = itemName[:50]
		}

		items = append(items, midtrans.ItemDetails{
			ID:    item.ProductID.String(),
			Name:  itemName,
			Price: int64(item.Price),
			Qty:   int32(item.Quantity),
		})
	}

	// 2. Mengintegrasikan elemen komponen biaya pengiriman (Ongkos Kirim RajaOngkir) jika tersedia
	if tx.ShippingCost > 0 {
		items = append(items, midtrans.ItemDetails{
			ID:    "SHIPPING_FEE",
			Name:  "Ongkos Kirim RajaOngkir",
			Price: int64(tx.ShippingCost),
			Qty:   1,
		})
	}

	// 3. Menghitung akumulasi total nilai transaksi (Gross Amount) untuk mencegah ketidakcocokan data
	var grossAmt int64
	for _, item := range items {
		grossAmt += item.Price * int64(item.Qty)
	}

	// Fallback nilai grossAmt dari atribut TotalAmount jika array items belum di-preload
	if grossAmt == 0 {
		grossAmt = int64(tx.TotalAmount)
	}

	// 4. Membangun struktur payload snap.Request
	snapReq := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  tx.MidtransOrderID,
			GrossAmt: grossAmt,
		},
		CustomerDetail: &midtrans.CustomerDetails{
			FName: buyerName,
			Email: buyerEmail,
		},
		Items: &items,
	}

	// 5. Mengeksekusi pembuatan Snap Transaction Token menggunakan SnapClient Midtrans
	snapResp, midErr := config.SnapClient.CreateTransaction(snapReq)
	if midErr != nil {
		log.Printf("[ERROR] Gagal memproses Snap Transaction di Midtrans (OrderID: %s): %s", tx.MidtransOrderID, midErr.Message)
		return nil, fmt.Errorf("gagal menghasilkan Snap Token Midtrans: %s", midErr.Message)
	}

	// 6. Memperbarui atribut SnapToken dan SnapRedirectURL pada skema tabel transactions
	tx.SnapToken = snapResp.Token
	tx.SnapRedirectURL = snapResp.RedirectURL

	// Menggunakan pengondisian eksplisit berbasis midtrans_order_id agar aman dari error 'WHERE conditions required'
	if err := s.db.Model(&models.Transaction{}).
		Where("midtrans_order_id = ?", tx.MidtransOrderID).
		Updates(map[string]interface{}{
			"snap_token":        snapResp.Token,
			"snap_redirect_url": snapResp.RedirectURL,
		}).Error; err != nil {
		log.Printf("[WARNING] Gagal memperbarui status Snap Token pada basis data (OrderID: %s): %v", tx.MidtransOrderID, err)
	}

	log.Printf("[SUCCESS] Berhasil menerbitkan Snap Transaction Token untuk OrderID: %s", tx.MidtransOrderID)
	return snapResp, nil
}

// ProcessWebhookNotification memproses payload notifikasi otomatis (HTTP Callback) dari server Midtrans
// untuk memperbarui status alur transaksi Rekening Bersama (Escrow) secara real-time pada basis data PostgreSQL.
func (s *MidtransService) ProcessWebhookNotification(payload map[string]interface{}) error {
	orderID, ok := payload["order_id"].(string)
	if !ok || orderID == "" {
		return errors.New("payload notifikasi webhook tidak valid: atribut order_id tidak ditemukan")
	}

	// 1. Melakukan konfirmasi status transaksi langsung ke server Midtrans (Core API Check) untuk keamanan
	var transactionStatus string
	var fraudStatus string

	resp, midErr := config.CoreAPIClient.CheckTransaction(orderID)
	if midErr != nil {
		log.Printf("[WARNING] Gagal verifikasi status via Core API (OrderID: %s): %s. Menggunakan data payload callback.", orderID, midErr.Message)

		if status, exists := payload["transaction_status"].(string); exists {
			transactionStatus = status
		}
		if fraud, exists := payload["fraud_status"].(string); exists {
			fraudStatus = fraud
		}
	} else {
		transactionStatus = resp.TransactionStatus
		fraudStatus = resp.FraudStatus
	}

	// 2. Mengambil entitas transaksi dari basis data PostgreSQL berdasarkan atribut midtrans_order_id
	var tx models.Transaction
	if err := s.db.Where("midtrans_order_id = ?", orderID).First(&tx).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("transaksi dengan Order ID '%s' tidak ditemukan pada basis data", orderID)
		}
		return fmt.Errorf("terjadi kesalahan pembacaan basis data: %w", err)
	}

	// 3. Memetakan status respons Midtrans ke enum TransactionStatus pada sistem Escrow EcoPlan
	var newStatus models.TransactionStatus

	switch transactionStatus {
	case "capture":
		switch fraudStatus {
		case "challenge":
			newStatus = models.StatusPendingPayment
		case "accept", "":
			newStatus = models.StatusPaidEscrow
		}
	case "settlement":
		newStatus = models.StatusPaidEscrow
	case "pending":
		newStatus = models.StatusPendingPayment
	case "deny", "cancel", "expire", "failure":
		newStatus = models.StatusCancelled
	default:
		log.Printf("[INFO] Notifikasi status transaksi Midtrans tidak diproses/diabaikan: %s (OrderID: %s)", transactionStatus, orderID)
		return nil
	}

	// 4. Memperbarui status alur transaksi escrow pada basis data PostgreSQL jika terjadi perubahan
	if tx.Status != newStatus {
		if err := s.db.Model(&tx).Update("status", newStatus).Error; err != nil {
			log.Printf("[ERROR] Gagal memperbarui status alur transaksi escrow (OrderID: %s): %v", orderID, err)
			return fmt.Errorf("gagal memperbarui status transaksi: %w", err)
		}
		log.Printf("[SUCCESS] Status transaksi escrow (OrderID: %s) berhasil diperbarui menjadi '%s'", orderID, newStatus)
	}

	return nil
}
