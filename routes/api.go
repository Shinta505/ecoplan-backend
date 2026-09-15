package routes

import (
	"net/http"

	"ecoplan-backend/controllers"
	"ecoplan-backend/middleware"
	"ecoplan-backend/models"

	"github.com/gin-gonic/gin"
)

// SetupRouter mendaftarkan seluruh jalur RESTful API ke Engine Gin Router,
// mengelompokkan API publik dan API terproteksi middleware sesuai spesifikasi sistem EcoPlan.
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// Global Middleware (opsional jika dibutuhkan, seperti CORS)
	r.Use(middleware.CORSMiddleware())

	// ==========================================
	// 0. API DOCUMENTATION ROUTE
	// ==========================================
	// Memuat seluruh templat antarmuka HTML dari direktori 'views'
	r.LoadHTMLGlob("views/*")

	// Menyajikan halaman dokumentasi API pada endpoint root (/) dan (/api/docs)
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "documentation.html", nil)
	})

	r.GET("/api/docs", func(c *gin.Context) {
		c.HTML(http.StatusOK, "documentation.html", nil)
	})

	// API Versioning Group (/api/v1)
	v1 := r.Group("/api/v1")
	{
		// ==========================================
		// 1. AUTHENTICATION ROUTES (Public)
		// ==========================================
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", controllers.Register)
			authGroup.POST("/login", controllers.Login)
		}

		// ==========================================
		// 2. WASTE DETECTION ROUTES (Machine Learning)
		// ==========================================
		detectionGroup := v1.Group("/detections")
		{
			// Publik (Guest) maupun Terautentikasi dapat melakukan deteksi sampah
			detectionGroup.POST("", middleware.OptionalAuthMiddleware(), controllers.DetectWaste)
			detectionGroup.GET("/:id", controllers.GetDetectionByID)

			// Endpoint terproteksi khusus riwayat deteksi pengguna login
			authenticatedDetections := detectionGroup.Group("")
			authenticatedDetections.Use(middleware.AuthMiddleware())
			{
				authenticatedDetections.GET("/history", controllers.GetDetectionHistoryList)
			}
		}

		// ==========================================
		// 3. WASTE GUIDES & GEMINI AI ROUTES
		// ==========================================
		guideGroup := v1.Group("/guides")
		{
			guideGroup.GET("", controllers.NewGuideController().GetAllGuides)
			guideGroup.GET("/:category", controllers.NewGuideController().GetGuideByCategory)
			guideGroup.POST("/ai-suggestion", controllers.NewGuideController().AskGemini)
			guideGroup.POST("/ask", controllers.NewGuideController().AskGemini)
		}

		// ==========================================
		// 4. EDUCATION & PUBLICATION ARTICLES ROUTES
		// ==========================================
		articleGroup := v1.Group("/articles")
		{
			// Publik dapat melihat daftar artikel dan detail artikel
			articleGroup.GET("", controllers.GetAllArticles)
			articleGroup.GET("/:id", controllers.GetArticleByID)

			// Terproteksi Middleware untuk pengajuan draf, pembaruan, dan penghapusan
			protectedArticles := articleGroup.Group("")
			protectedArticles.Use(middleware.AuthMiddleware())
			{
				protectedArticles.POST("", controllers.CreateArticle)
				protectedArticles.GET("/my-articles", controllers.GetMyArticles)
				protectedArticles.PUT("/:id", controllers.UpdateArticle)
				protectedArticles.DELETE("/:id", controllers.DeleteArticle)

				// Khusus Administrator untuk verifikasi dan review Turnitin
				adminArticleReview := protectedArticles.Group("")
				adminArticleReview.Use(middleware.RequireRoles(models.RoleAdmin))
				{
					adminArticleReview.PATCH("/:id/review", controllers.ReviewArticle)
				}
			}
		}

		// ==========================================
		// 5. MARKETPLACE, STORES & PRODUCTS ROUTES
		// ==========================================
		storeGroup := v1.Group("/stores")
		{
			// Publik dapat melihat detail toko
			storeGroup.Use(middleware.AuthMiddleware())
			{
				storeGroup.POST("", controllers.CreateStore)
				storeGroup.GET("/my-store", controllers.GetMyStore)
				storeGroup.PUT("", controllers.UpdateStore)
				storeGroup.GET("/my-products", controllers.GetMyStoreProducts)
			}
		}

		marketplaceGroup := v1.Group("/marketplace")
		{
			// Publik dapat melihat katalog produk marketplace
			marketplaceGroup.GET("/products", controllers.GetAllProducts)
			marketplaceGroup.GET("/products/:id", controllers.GetProductByID)

			// Terproteksi Seller / Admin untuk manajemen produk
			protectedMarketplace := marketplaceGroup.Group("")
			protectedMarketplace.Use(middleware.AuthMiddleware(), middleware.RequireRoles(models.RoleSeller, models.RoleAdmin))
			{
				protectedMarketplace.POST("/products", controllers.CreateProduct)
				protectedMarketplace.PUT("/products/:id", controllers.UpdateProduct)
				protectedMarketplace.DELETE("/products/:id", controllers.DeleteProduct)
			}
		}

		// ==========================================
		// 6. ESCROW TRANSACTIONS & PAYMENT ROUTES
		// ==========================================
		transactionGroup := v1.Group("/transactions")
		{
			// Webhook Midtrans bersifat publik tanpa autentikasi token JWT
			transactionGroup.POST("/webhook", controllers.PaymentNotificationWebhook)

			// Terproteksi untuk pembeli, penjual, dan admin
			protectedTransactions := transactionGroup.Group("")
			protectedTransactions.Use(middleware.AuthMiddleware())
			{
				protectedTransactions.POST("", controllers.CreateTransaction)
				protectedTransactions.GET("", controllers.GetAllTransactions)
				protectedTransactions.GET("/:id", controllers.GetTransactionByID)
				protectedTransactions.PATCH("/:id/status", controllers.UpdateTransactionStatus)
			}
		}

		paymentGroup := v1.Group("/payment")
		{
			paymentGroup.Use(middleware.AuthMiddleware())
			{
				paymentGroup.GET("/status/:order_id", controllers.CheckPaymentStatus)
				paymentGroup.POST("/withdraw", controllers.WithdrawSellerBalance)
			}
		}

		// ==========================================
		// 7. LOGISTICS & SHIPPING ROUTES (RajaOngkir)
		// ==========================================
		logisticsGroup := v1.Group("/logistics")
		{
			logisticsGroup.GET("/destinations", (&controllers.LogisticsController{}).SearchDestination)
			logisticsGroup.POST("/shipping-cost", controllers.GetShippingCost)
			logisticsGroup.GET("/tracking", controllers.TrackShipment)
		}

		shippingGroup := v1.Group("/shipping")
		{
			shippingGroup.Use(middleware.AuthMiddleware())
			{
				shippingGroup.POST("/calculate", controllers.CalculateShipping)
				shippingGroup.GET("/tracking/:transaction_id", controllers.TrackOrder)
				shippingGroup.PATCH("/receipt/:transaction_id", middleware.RequireRoles(models.RoleSeller, models.RoleAdmin), controllers.UpdateReceipt)
			}
		}

		// ==========================================
		// 8. USER PROFILE & GAMIFICATION REWARDS
		// ==========================================
		profileGroup := v1.Group("/profile")
		{
			profileGroup.Use(middleware.AuthMiddleware())
			{
				profileGroup.GET("", controllers.GetProfile)
				profileGroup.PUT("", controllers.UpdateProfile)
				profileGroup.GET("/transactions", controllers.GetProfileTransactions)
				profileGroup.GET("/detections", controllers.GetDetectionHistory)
			}
		}

		// ==========================================
		// 9. ADMIN YOUTUBE VIDEO MANAGEMENT ROUTES
		// ==========================================
		videoGroup := v1.Group("/videos")
		{
			// Publik dapat menonton dan memfilter daftar video edukasi
			videoGroup.GET("", controllers.GetAllVideos)
			videoGroup.GET("/:id", controllers.GetVideoByID)

			// Khusus Administrator untuk menambah, mengubah, dan menghapus video edukasi
			adminVideos := videoGroup.Group("")
			adminVideos.Use(middleware.AuthMiddleware(), middleware.RequireRoles(models.RoleAdmin))
			{
				adminVideos.POST("", controllers.CreateVideo)
				adminVideos.PUT("/:id", controllers.UpdateVideo)
				adminVideos.DELETE("/:id", controllers.DeleteVideo)
			}
		}

		// ==========================================
		// 10. SYSTEM STATISTICAL SUMMARY ROUTE
		// ==========================================
		statsGroup := v1.Group("/stats")
		{
			statsGroup.GET("/summary", controllers.GetStatsSummary)
		}
	}

	return r
}
