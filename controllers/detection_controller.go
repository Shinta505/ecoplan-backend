package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ecoplan-backend/config"
	"ecoplan-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AIDetectionData mendefinisikan struktur objek data hasil inferensi dari Microservice AI Python.
type AIDetectionData struct {
	PredictedCategory string  `json:"predicted_category"`
	WasteGroup        string  `json:"waste_group"`
	ConfidenceScore   float64 `json:"confidence_score"`
	ModelUsed         string  `json:"model_used"`
}

// AIDetectionResponse mendefinisikan struktur pembungkus JSON respons HTTP dari Microservice AI.
type AIDetectionResponse struct {
	Success bool            `json:"success"`
	Data    AIDetectionData `json:"data"`
	Detail  string          `json:"detail,omitempty"`
}

// DetectWaste mengeksekusi penerimaan berkas citra sampah, meneruskannya ke Microservice AI (ResNet50 ONNX),
// dan menyimpan hasil prediksi klasifikasi beserta metadata ke dalam basis data PostgreSQL/Supabase.
//
// HTTP Endpoint : POST /api/v1/detections
// Hak Akses     : Publik (Pengguna Anonim / Guest) dan Terautentikasi (Member / User)
func DetectWaste(c *gin.Context) {
	// 1. Membaca berkas gambar dari Form Data HTTP Request (Kunci utama: "image", fallback: "file")
	fileHeader, err := c.FormFile("image")
	if err != nil {
		fileHeader, err = c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Berkas citra sampah tidak ditemukan. Pastikan Form-Data menggunakan kunci 'image'.",
				"error":   err.Error(),
			})
			return
		}
	}

	// 2. Validasi ekstensi format citra (Mendukung format JPEG, JPG, PNG, WEBP, JFIF)
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" && ext != ".jfif" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format berkas tidak didukung. Harap unggah citra berformat JPEG, JPG, PNG, WEBP, atau JFIF.",
		})
		return
	}

	// 3. Penyimpanan fisik berkas citra ke direktori lokal server (/uploads/detections)
	uploadDir := "uploads/detections"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal membuat direktori penyimpanan berkas citra pada server.",
			"error":   err.Error(),
		})
		return
	}

	// Penamaan unik berkas menggunakan stempel waktu (timestamp) nanodetik
	uniqueFileName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(fileHeader.Filename))
	savedPath := filepath.Join(uploadDir, uniqueFileName)

	if err := c.SaveUploadedFile(fileHeader, savedPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menyimpan berkas citra sampah ke direktori server backend.",
			"error":   err.Error(),
		})
		return
	}

	imageURL := fmt.Sprintf("/uploads/detections/%s", uniqueFileName)

	// 4. Membuka berkas yang baru disimpan untuk dikirimkan ke Microservice AI Python
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal membaca berkas citra untuk diproses ke Microservice AI.",
			"error":   err.Error(),
		})
		return
	}
	defer file.Close()

	// 5. Menyusun payload Multipart/Form-Data HTTP Request ke Microservice AI
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	// Menentukan MIME Content-Type secara presisi agar lolos validasi FastAPI (image.content_type.startswith("image/"))
	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" || !strings.HasPrefix(mimeType, "image/") {
		switch ext {
		case ".png":
			mimeType = "image/png"
		case ".webp":
			mimeType = "image/webp"
		default:
			mimeType = "image/jpeg"
		}
	}

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="%s"`, filepath.Base(fileHeader.Filename)))
	h.Set("Content-Type", mimeType)

	partWriter, err := bodyWriter.CreatePart(h)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengonstruksi payload multipart untuk Microservice AI.",
			"error":   err.Error(),
		})
		return
	}

	if _, err := io.Copy(partWriter, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menyalin data biner citra ke aliran payload HTTP.",
			"error":   err.Error(),
		})
		return
	}
	bodyWriter.Close()

	// 6. Penentuan URL Endpoint Microservice AI berdasarkan Variabel Lingkungan (.env)
	aiBaseURL := config.ENV.AIServiceURL
	if strings.TrimSpace(aiBaseURL) == "" {
		aiBaseURL = os.Getenv("AI_SERVICE_URL")
	}
	if strings.TrimSpace(aiBaseURL) == "" {
		aiBaseURL = "https://ecoplan-ai-service-production.up.railway.app"
	}

	aiEndpoint := fmt.Sprintf("%s/api/v1/detections", strings.TrimRight(aiBaseURL, "/"))

	// 7. Pengiriman HTTP POST Request ke Microservice AI
	req, err := http.NewRequest(http.MethodPost, aiEndpoint, bodyBuf)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menginisialisasi HTTP Request ke Microservice AI.",
			"error":   err.Error(),
		})
		return
	}
	req.Header.Set("Content-Type", bodyWriter.FormDataContentType())

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat berkomunikasi dengan Microservice AI.",
			"error":   err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal membaca respons payload dari Microservice AI.",
			"error":   err.Error(),
		})
		return
	}

	// 8. Parsing respons JSON dari Microservice AI
	var aiResp AIDetectionResponse
	if err := json.Unmarshal(respBody, &aiResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":  false,
			"message":  "Gagal melakukan pemetikan (unmarshal) data JSON dari Microservice AI.",
			"error":    err.Error(),
			"raw_body": string(respBody),
		})
		return
	}

	if resp.StatusCode != http.StatusOK || !aiResp.Success {
		errMsg := aiResp.Detail
		if errMsg == "" {
			errMsg = "Microservice AI mengembalikan status error."
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": errMsg,
		})
		return
	}

	// 9. Mengidentifikasi Sesi Pengguna (Mendukung Guest & Authenticated User)
	var userIDPtr *uuid.UUID
	if userIDVal, exists := c.Get("user_id"); exists {
		if uid, ok := userIDVal.(uuid.UUID); ok {
			userIDPtr = &uid
		}
	}

	// 10. Menginstansiasi dan Menyimpan Rekam Data Deteksi ke Basis Data PostgreSQL/Supabase
	modelUsed := aiResp.Data.ModelUsed
	if modelUsed == "" {
		modelUsed = "ResNet50_ONNX"
	}

	detectionRecord := models.WasteDetection{
		UserID:            userIDPtr,
		ImageURL:          imageURL,
		PredictedCategory: aiResp.Data.PredictedCategory,
		WasteGroup:        aiResp.Data.WasteGroup,
		ConfidenceScore:   aiResp.Data.ConfidenceScore,
		ModelUsed:         modelUsed,
	}

	if err := config.DB.Create(&detectionRecord).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menyimpan rekam riwayat deteksi sampah ke basis data.",
			"error":   err.Error(),
		})
		return
	}

	// 11. Penambahan Hadiah Eco-Points untuk Pengguna Terautentikasi (Sistem Gamifikasi)
	if userIDPtr != nil {
		config.DB.Model(&models.User{}).
			Where("id = ?", *userIDPtr).
			UpdateColumn("eco_points", gorm.Expr("eco_points + ?", 10))
	}

	// 12. Mengembalikan Respons Sukses HTTP 201 Created
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Proses deteksi sampah menggunakan model ResNet50 ONNX berhasil dilakukan.",
		"data":    detectionRecord,
	})
}

// GetDetectionHistoryList mengambil riwayat hasil pemindaian sampah milik pengguna yang terautentikasi.
//
// HTTP Endpoint : GET /api/v1/detections/history
// Hak Akses     : Authenticated User (Bearer JWT)
func GetDetectionHistoryList(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Akses ditolak. Sesi otentikasi pengguna tidak ditemukan.",
		})
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Format identitas pengguna (User ID) tidak valid.",
		})
		return
	}

	var detections []models.WasteDetection
	err := config.DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&detections).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil riwayat deteksi sampah dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil seluruh riwayat deteksi sampah pengguna.",
		"data": gin.H{
			"total_count": len(detections),
			"detections":  detections,
		},
	})
}

// GetDetectionByID mengambil rincian data hasil deteksi sampah spesifik berdasarkan UUID Identifier.
//
// HTTP Endpoint : GET /api/v1/detections/:id
// Hak Akses     : Publik / Authenticated User
func GetDetectionByID(c *gin.Context) {
	detectionIDStr := c.Param("id")
	detectionID, err := uuid.Parse(detectionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format ID deteksi sampah tidak valid. Harap gunakan format UUID.",
		})
		return
	}

	var detection models.WasteDetection
	if err := config.DB.Preload("User").Where("id = ?", detectionID).First(&detection).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Data hasil deteksi sampah tidak ditemukan.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Terjadi kesalahan saat membaca data deteksi dari basis data.",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Berhasil mengambil rincian data deteksi sampah.",
		"data":    detection,
	})
}
