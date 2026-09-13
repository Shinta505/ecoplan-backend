package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config mendefinisikan struktur data untuk menyimpan seluruh konfigurasi variabel lingkungan aplikasi.
type Config struct {
	// Server & Database Configuration
	AppPort    string
	DBHost     string
	DBUser     string
	DBPassword string
	DBName     string
	DBPort     string
	DBSslMode  string
	GinMode    string // Menambahkan properti GinMode untuk konfigurasi mode Gin Framework

	// Authentication Configuration
	JWTSecret string

	// External AI Microservice & Gemini API Configuration
	AIServiceURL string
	APIKeyGemini string

	// Midtrans Payment Gateway Configuration
	MidtransID           string
	MidtransServerKey    string
	MidtransClientKey    string
	MidtransIsProduction bool

	// RajaOngkir Logistics API Configuration
	RajaOngkirAPIKey string
}

// ENV merupakan instansiasi variabel global untuk memfasilitasi akses konfigurasi di seluruh modul backend.
var ENV *Config

// LoadEnv mengeksekusi pemuatan berkas .env ke dalam struktur data Config.
func LoadEnv() *Config {
	// Memuat berkas .env. Jika tidak ditemukan, sistem akan beralih menggunakan environment variable OS.
	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] Berkas .env tidak ditemukan. Memakai variabel lingkungan bawaan sistem (OS environment).")
	}

	ENV = &Config{
		AppPort:              getEnv("PORT", "8080"),
		DBHost:               getEnv("DB_HOST", "localhost"),
		DBUser:               getEnv("DB_USER", "postgres"),
		DBPassword:           getEnv("DB_PASSWORD", "postgres"),
		DBName:               getEnv("DB_NAME", "ecoplan_db"),
		DBPort:               getEnv("DB_PORT", "5432"),
		DBSslMode:            getEnv("DB_SSLMODE", "disable"),
		GinMode:              getEnv("GIN_MODE", "debug"), // Membaca GIN_MODE dengan fallback ke "debug"
		JWTSecret:            getEnv("JWT_SECRET", "ecoplan_jwt_secret_key"),
		AIServiceURL:         getEnv("AI_SERVICE_URL", "https://ecoplan-ai-service-production.up.railway.app"),
		APIKeyGemini:         getEnv("API_KEY_GEMINI", ""),
		MidtransID:           getEnv("MIDTRANS_ID", ""),
		MidtransServerKey:    getEnv("MIDTRANS_SERVER_KEY", ""),
		MidtransClientKey:    getEnv("MIDTRANS_CLIENT_KEY", ""),
		MidtransIsProduction: getEnvAsBool("MIDTRANS_IS_PRODUCTION", false),
		RajaOngkirAPIKey:     getEnv("RAJAONGKIR_API_KEY", ""),
	}

	return ENV
}

// getEnv melakukan pembacaan variabel lingkungan berdasarkan kunci (key) dengan fallback berupa nilai default.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getEnvAsBool melakukan pemetaan variabel lingkungan string ke tipe data boolean.
func getEnvAsBool(key string, fallback bool) bool {
	valStr := getEnv(key, "")
	if val, err := strconv.ParseBool(valStr); err == nil {
		return val
	}
	return fallback
}
