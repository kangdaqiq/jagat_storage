package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"storage_server/internal/config"
	"storage_server/internal/utils"
)

var startTime = time.Now()

type HealthHandler struct {
	cfg *config.Config
}

func NewHealthHandler(cfg *config.Config) *HealthHandler {
	return &HealthHandler{cfg: cfg}
}

func (h *HealthHandler) HealthCheck(c *gin.Context) {
	uptime := time.Since(startTime).Round(time.Second).String()

	utils.SendSuccess(c, http.StatusOK, "Storage server running properly", gin.H{
		"service":          "Android Storage Server",
		"version":          "1.0.0",
		"status":           "healthy",
		"uptime":           uptime,
		"max_upload_size":  utils.FormatFileSize(h.cfg.MaxUploadSizeMB * 1024 * 1024),
		"upload_directory": h.cfg.UploadDir,
	})
}
