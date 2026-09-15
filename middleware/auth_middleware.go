package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"ecoplan-backend/config"
	"ecoplan-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTClaims mendefinisikan struktur data payload untuk klaim JWT token.
type JWTClaims struct {
	UserID string          `json:"user_id"`
	Email  string          `json:"email"`
	Role   models.UserRole `json:"role"`
	jwt.RegisteredClaims
}

// AuthMiddleware mengamankan endpoint terproteksi dengan memverifikasi keberadaan
// dan keabsahan Bearer JWT Token pada header HTTP Authorization.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Mengambil header Authorization dari HTTP Request
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Akses ditolak. Header Authorization tidak ditemukan.",
			})
			return
		}

		// 2. Memvalidasi format header Authorization ("Bearer <token>")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Format token otorisasi tidak valid. Gunakan skema 'Bearer <token>'.",
			})
			return
		}

		tokenString := parts[1]
		claims := &JWTClaims{}

		// 3. Melakukan dekoding dan verifikasi enkripsi HMAC signature menggunakan JWTSecret
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("metode penandatanganan token tidak valid: %v", token.Header["alg"])
			}
			return []byte(config.ENV.JWTSecret), nil
		})

		// 4. Memeriksa validitas token dan waktu kedaluwarsa (expiration time)
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Token otentikasi tidak valid atau telah kedaluwarsa.",
			})
			return
		}

		// 5. Konversi string ID pengguna menjadi tipe data UUID
		userUUID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Format identitas pengguna (User ID) pada token tidak valid.",
			})
			return
		}

		// 6. Menyimpan informasi pengguna ke dalam konteks Gin untuk diakses pada controller/handler
		c.Set("user_id", userUUID)
		c.Set("user_email", claims.Email)
		c.Set("user_role", claims.Role)

		c.Next()
	}
}

func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			claims := &JWTClaims{}
			token, err := jwt.ParseWithClaims(parts[1], claims, func(token *jwt.Token) (interface{}, error) {
				return []byte(config.ENV.JWTSecret), nil
			})
			if err == nil && token.Valid {
				if userUUID, err := uuid.Parse(claims.UserID); err == nil {
					c.Set("user_id", userUUID)
					c.Set("user_email", claims.Email)
					c.Set("user_role", claims.Role)
				}
			}
		}
		c.Next()
	}
}

// RequireRoles memvalidasi otorisasi peran pengguna (Role-Based Access Control / RBAC)
// untuk memastikan pengguna memiliki hak akses yang diperbolehkan.
func RequireRoles(allowedRoles ...models.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get("user_role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Sesi otentikasi pengguna tidak ditemukan.",
			})
			return
		}

		userRole, ok := roleVal.(models.UserRole)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Gagal membaca peranan pengguna dari konteks sistem.",
			})
			return
		}

		// Memeriksa apakah peranan pengguna terdapat pada daftar peranan yang diizinkan (allowedRoles)
		isAllowed := false
		for _, role := range allowedRoles {
			if userRole == role {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Akses ditolak. Anda tidak memiliki hak akses (role) untuk sumber daya ini.",
			})
			return
		}

		c.Next()
	}
}
