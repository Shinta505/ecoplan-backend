package config

import (
	"log"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/midtrans/midtrans-go/snap"
)

// SnapClient merupakan variabel global untuk memfasilitasi pembuatan transaksi dan token pembayaran Snap API.
var SnapClient snap.Client

// CoreAPIClient merupakan variabel global untuk pemrosesan status transaksi dan penanganan callback webhook Midtrans.
var CoreAPIClient coreapi.Client

// InitMidtrans menginisialisasi parameter konfigurasi global Payment Gateway Midtrans berdasarkan variabel lingkungan.
func InitMidtrans() {
	// Menentukan lingkungan eksekusi transaksi (Sandbox untuk tahap pengembangan atau Production untuk perilisan)
	envType := midtrans.Sandbox
	if ENV.MidtransIsProduction {
		envType = midtrans.Production
		log.Println("[INFO] Mode Payment Gateway Midtrans dikonfigurasi pada lingkungan Production.")
	} else {
		log.Println("[INFO] Mode Payment Gateway Midtrans dikonfigurasi pada lingkungan Sandbox.")
	}

	// Mengatur kunci autentikasi server (Server Key) dan mode lingkungan global SDK Midtrans
	midtrans.ServerKey = ENV.MidtransServerKey
	midtrans.ClientKey = ENV.MidtransClientKey
	midtrans.Environment = envType

	// Menginstansiasi Snap Client untuk antarmuka pembayaran
	SnapClient.New(ENV.MidtransServerKey, envType)

	// Menginstansiasi Core API Client untuk kebutuhan pemrosesan transaksi tingkat rendah (Low-Level API)
	CoreAPIClient.New(ENV.MidtransServerKey, envType)

	log.Println("[INFO] Inisialisasi modul konfigurasi Midtrans Payment Gateway berhasil dimuat.")
}
