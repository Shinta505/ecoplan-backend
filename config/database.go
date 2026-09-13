package config

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DB merupakan instansiasi variabel global untuk memfasilitasi akses entitas basis data GORM di seluruh modul backend.
var DB *gorm.DB

// ConnectDatabase menginisialisasi koneksi ke basis data PostgreSQL/Supabase menggunakan parameter DSN dari objek ENV.
func ConnectDatabase() *gorm.DB {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Jakarta",
		ENV.DBHost,
		ENV.DBUser,
		ENV.DBPassword,
		ENV.DBName,
		ENV.DBPort,
		ENV.DBSslMode,
	)

	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("[ERROR] Gagal melakukan koneksi ke basis data PostgreSQL: %v", err)
	}

	log.Println("[INFO] Berhasil terhubung ke basis data PostgreSQL/Supabase.")
	DB = database
	return DB
}
