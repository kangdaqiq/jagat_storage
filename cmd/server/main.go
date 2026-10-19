package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"storage_server/internal/config"
	"storage_server/internal/handler"
	"storage_server/internal/middleware"
	"storage_server/internal/storage"
)

func main() {
	cfg := config.LoadConfig()

	// Inisialisasi storage engine
	store, err := storage.NewFileStore(cfg.UploadDir)
	if err != nil {
		log.Fatalf("Gagal inisialisasi storage: %v", err)
	}

	r := gin.Default()

	// Pasang middleware global CORS
	r.Use(middleware.CORSMiddleware())

	// Limit request body
	r.MaxMultipartMemory = 32 << 20 // 32 MB buffer in memory

	// Handler instances
	healthH := handler.NewHealthHandler(cfg)
	fileH := handler.NewFileHandler(store, cfg)

	// Root endpoint
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "Android Storage Server API",
			"status":  "online",
			"version": "1.0.0",
			"endpoints": gin.H{
				"health":          "GET /api/v1/health",
				"upload_single":   "POST /api/v1/files/upload (Form: 'file', 'folder', 'name')",
				"upload_multiple": "POST /api/v1/files/upload-multiple (Form: 'files', 'folder')",
				"list_files":      "GET /api/v1/files?folder=&search=&page=1&limit=20",
				"file_info":       "GET /api/v1/files/:id",
				"preview_stream":  "GET /api/v1/files/:id/view",
				"download":        "GET /api/v1/files/:id/download",
				"delete":          "DELETE /api/v1/files/:id",
			},
			"auth_info": gin.H{
				"header_key":           "X-API-Key atau Authorization: Bearer <key>",
				"required_for_upload":  cfg.RequireAuthForUpload,
				"required_for_download": cfg.RequireAuthForDownload,
			},
		})
	})

	api := r.Group("/api/v1")
	{
		api.GET("/health", healthH.HealthCheck)

		files := api.Group("/files")
		{
			// Endpoint yang memerlukan autentikasi jika dikonfigurasi
			uploadAuth := middleware.APIKeyAuth(cfg.APIKey, cfg.RequireAuthForUpload)
			downloadAuth := middleware.APIKeyAuth(cfg.APIKey, cfg.RequireAuthForDownload)
			actionAuth := middleware.APIKeyAuth(cfg.APIKey, true) // Hapus file selalu dilindungi

			// Upload
			files.POST("/upload", uploadAuth, fileH.UploadSingle)
			files.POST("/upload-multiple", uploadAuth, fileH.UploadMultiple)

			// Read & List
			files.GET("", fileH.ListFiles)
			files.GET("/:id", fileH.GetFileInfo)

			// Streaming & Download
			files.GET("/:id/view", downloadAuth, fileH.ViewFile)
			files.GET("/:id/download", downloadAuth, fileH.DownloadFile)

			// Delete
			files.DELETE("/:id", actionAuth, fileH.DeleteFile)
		}
	}

	address := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	log.Printf("=====================================================")
	log.Printf(" Storage Server siap berjalan di http://%s", address)
	log.Printf(" Storage Directory : %s", cfg.UploadDir)
	log.Printf(" Max Upload Size   : %d MB", cfg.MaxUploadSizeMB)
	log.Printf(" API Key           : %s", cfg.APIKey)
	log.Printf("=====================================================")

	if err := r.Run(address); err != nil {
		log.Fatalf("Server gagal berjalan: %v", err)
	}
}
