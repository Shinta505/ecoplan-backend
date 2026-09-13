package controllers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"ecoplan-backend/config"
	"ecoplan-backend/middleware"
	"ecoplan-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// RegisterInput mendefinisikan struktur DTO (Data Transfer Object) untuk enkapsulasi payload registrasi pengguna.
type RegisterInput struct {
	Name     string          `json:"name" binding:"required"`
	Email    string          `json:"email" binding:"required,email"`
	Password string          `json:"password" binding:"required,min=6"`
	Role     models.UserRole `json:"role"`
}

// LoginInput mendefinisikan struktur DTO untuk enkapsulasi payload otentikasi masuk pengguna.
type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse mendefinisikan struktur respons standar untuk menyajikan informasi otentikasi beserta token JWT.
type AuthResponse struct {
	Token     string      `json:"token"`
	TokenType string      `json:"token_type"`
	ExpiresAt string      `json:"expires_at"`
	User      models.User `json:"user"`
}

// Register mengeksekusi pendaftaran akun baru, enkripsi kata sandi via bcrypt, dan penyimpanan ke PostgreSQL.
func Register(c *gin.Context) {
	var input RegisterInput

	// 1. Validasi struktur skema payload HTTP JSON Request
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format data masukan tidak valid.",
			"error":   err.Error(),
		})
		return
	}

	// 2. Normalisasi input email dan pengecekan keberadaan akun pada basis data
	normalizedEmail := strings.ToLower(strings.TrimSpace(input.Email))
	var existingUser models.User
	err := config.DB.Where("email = ?", normalizedEmail).First(&existingUser).Error
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "Alamat email sudah terdaftar dalam sistem.",
		})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat memverifikasi data email pada basis data.",
		})
		return
	}

	// 3. Enkripsi kata sandi menggunakan algoritma Hashing Bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal melakukan enkripsi kata sandi pengguna.",
		})
		return
	}

	// 4. Validasi penetapan peranan pengguna (Role Assignment)
	// Registrasi publik secara default menetapkan role 'user' atau 'seller' (mencegah eksploitasi peranan 'admin')
	userRole := models.RoleUser
	if input.Role == models.RoleSeller {
		userRole = models.RoleSeller
	}

	// 5. Instansiasi objek model User dan eksekusi operasi persistensi basis data
	newUser := models.User{
		Name:         strings.TrimSpace(input.Name),
		Email:        normalizedEmail,
		PasswordHash: string(hashedPassword),
		Role:         userRole,
		EcoPoints:    0,
		Balance:      0.0,
	}

	if err := config.DB.Create(&newUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menyimpan data registrasi pengguna ke basis data.",
		})
		return
	}

	// 6. Mengembalikan respons status HTTP 201 Created
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Registrasi akun pengguna berhasil dilakukan.",
		"data": gin.H{
			"id":         newUser.ID,
			"name":       newUser.Name,
			"email":      newUser.Email,
			"role":       newUser.Role,
			"created_at": newUser.CreatedAt,
		},
	})
}

// Login mengeksekusi otentikasi kredensial pengguna dan menerbitkan JWT Access Token.
func Login(c *gin.Context) {
	var input LoginInput

	// 1. Validasi struktur payload JSON Request
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Kredensial login tidak lengkap atau format tidak sesuai.",
			"error":   err.Error(),
		})
		return
	}

	// 2. Pencarian entitas pengguna berdasarkan alamat email
	normalizedEmail := strings.ToLower(strings.TrimSpace(input.Email))
	var user models.User
	if err := config.DB.Where("email = ?", normalizedEmail).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Kredensial login tidak valid. Silakan periksa email dan kata sandi Anda.",
		})
		return
	}

	// 3. Verifikasi kesesuaian kata sandi plain-text dengan Hashed Password pada basis data
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Kredensial login tidak valid. Silakan periksa email dan kata sandi Anda.",
		})
		return
	}

	// 4. Konfigurasi klaim dan masa berlaku JWT Access Token (24 jam)
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &middleware.JWTClaims{
		UserID: user.ID.String(),
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "ecoplan-backend",
		},
	}

	// 5. Penandatanganan token menggunakan skema HMAC-SHA256 (HS256) dan JWTSecret
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.ENV.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menerbitkan token otentikasi JWT.",
		})
		return
	}

	// 6. Mengembalikan HTTP 200 OK beserta payload token otentikasi
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Otentikasi login berhasil.",
		"data": AuthResponse{
			Token:     tokenString,
			TokenType: "Bearer",
			ExpiresAt: expirationTime.Format(time.RFC3339),
			User:      user,
		},
	})
}
