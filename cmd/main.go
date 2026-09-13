package main

import (
	"log"

	"ecoplan-backend/config"
	"ecoplan-backend/models"
	"ecoplan-backend/routes"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Memuat variabel lingkungan (Environment Variables) dari berkas .env
	cfg := config.LoadEnv()

	// 2. Menginisialisasi koneksi basis data PostgreSQL menggunakan GORM
	db := config.ConnectDatabase()

	// 3. Menjalankan Auto-Migrate untuk memastikan seluruh tabel model terbuat secara otomatis di basis data
	err := db.AutoMigrate(
		&models.User{},
		&models.Store{},
		&models.Product{},
		&models.Transaction{},
		&models.TransactionItem{},
		&models.WasteDetection{},
		&models.Article{},
		&models.Video{},
	)
	if err != nil {
		log.Fatalf("[ERROR] Gagal melakukan migrasi otomatis basis data: %v", err)
	}
	log.Println("[INFO] Migrasi skema tabel basis data PostgreSQL berhasil diselesaikan.")

	// 4. Menginisialisasi konfigurasi Payment Gateway Midtrans
	config.InitMidtrans()

	// 5. Mengatur mode Gin Engine secara aman (menggunakan debug sebagai default atau membaca environment GIN_MODE)
	gin.SetMode(cfg.GinMode)

	// 6. Menginisialisasi dan mendaftarkan seluruh rute API melalui package routes
	r := routes.SetupRouter()

	// 7. Menjalankan HTTP Server pada port yang dikonfigurasi
	// Cloud Run akan menyuntikkan env var PORT secara otomatis.
	port := cfg.AppPort
	if port == "" {
		port = "8080"
	}

	// Mengikat secara eksplisit ke 0.0.0.0 agar container dapat menerima traffic dari luar (Cloud Run proxy)
	serverAddr := "0.0.0.0:" + port
	log.Printf("[INFO] Server HTTP Backend EcoPlan berjalan dan mendengarkan pada antarmuka %s...", serverAddr)

	if err := r.Run(serverAddr); err != nil {
		log.Fatalf("[ERROR] Gagal menjalankan HTTP Server: %v", err)
	}
}
