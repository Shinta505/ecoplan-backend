package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ecoplan-backend/config"
	"ecoplan-backend/middleware"
	"ecoplan-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// setupTestEnvironment menginisialisasi lingkungan pengujian dan konfigurasi JWTSecret.
func setupTestEnvironment() string {
	secret := "ecoplan_jwt_test_secret_key_12345"
	config.ENV = &config.Config{
		JWTSecret: secret,
	}
	gin.SetMode(gin.TestMode)
	return secret
}

// generateMockToken membuat JWT Token tiruan untuk kebutuhan skenario pengujian unit.
func generateMockToken(secret string, userID string, email string, role models.UserRole, duration time.Duration) string {
	claims := &middleware.JWTClaims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(secret))
	return tokenString
}

// TestAuthMiddleware menguji berbagai kasus verifikasi header dan validitas token JWT.
func TestAuthMiddleware(t *testing.T) {
	secret := setupTestEnvironment()
	validUserID := uuid.New().String()

	// Pembuatan token mock untuk berbagai skenario
	validToken := generateMockToken(secret, validUserID, "user@ecoplan.com", models.RoleUser, 1*time.Hour)
	expiredToken := generateMockToken(secret, validUserID, "user@ecoplan.com", models.RoleUser, -1*time.Hour)
	invalidSignatureToken := generateMockToken("wrong_secret_key", validUserID, "user@ecoplan.com", models.RoleUser, 1*time.Hour)
	invalidUUIDToken := generateMockToken(secret, "invalid-uuid-format", "user@ecoplan.com", models.RoleUser, 1*time.Hour)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "Valid Token - Mengembalikan HTTP 200 OK",
			authHeader:     "Bearer " + validToken,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Header Authorization Kosong - Mengembalikan HTTP 401 Unauthorized",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Format Header Otorisasi Tanpa Prefix Bearer - Mengembalikan HTTP 401 Unauthorized",
			authHeader:     "Basic " + validToken,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Token Kedaluwarsa (Expired) - Mengembalikan HTTP 401 Unauthorized",
			authHeader:     "Bearer " + expiredToken,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Signature Token Tidak Valid (Wrong Secret) - Mengembalikan HTTP 401 Unauthorized",
			authHeader:     "Bearer " + invalidSignatureToken,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Format User ID UUID Tidak Valid - Mengembalikan HTTP 401 Unauthorized",
			authHeader:     "Bearer " + invalidUUIDToken,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(middleware.AuthMiddleware())
			r.GET("/api/v1/protected", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "success"})
			})

			req, _ := http.NewRequest(http.MethodGet, "/api/v1/protected", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestRequireRoles menguji pembatasan hak akses berbasis peranan (RBAC).
func TestRequireRoles(t *testing.T) {
	secret := setupTestEnvironment()
	adminUserID := uuid.New().String()
	regularUserID := uuid.New().String()

	adminToken := generateMockToken(secret, adminUserID, "admin@ecoplan.com", models.RoleAdmin, 1*time.Hour)
	userToken := generateMockToken(secret, regularUserID, "user@ecoplan.com", models.RoleUser, 1*time.Hour)

	tests := []struct {
		name           string
		authHeader     string
		allowedRoles   []models.UserRole
		expectedStatus int
	}{
		{
			name:           "Role Admin Mengakses Endpoint Khusus Admin - HTTP 200 OK",
			authHeader:     "Bearer " + adminToken,
			allowedRoles:   []models.UserRole{models.RoleAdmin},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Role User Mengakses Endpoint Khusus Admin - HTTP 403 Forbidden",
			authHeader:     "Bearer " + userToken,
			allowedRoles:   []models.UserRole{models.RoleAdmin},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Role User Mengakses Endpoint Multi-Role (User & Admin) - HTTP 200 OK",
			authHeader:     "Bearer " + userToken,
			allowedRoles:   []models.UserRole{models.RoleUser, models.RoleAdmin},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(middleware.AuthMiddleware(), middleware.RequireRoles(tt.allowedRoles...))
			r.GET("/api/v1/admin/dashboard", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "granted"})
			})

			req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
			req.Header.Set("Authorization", tt.authHeader)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
