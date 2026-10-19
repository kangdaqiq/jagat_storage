package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"storage_server/internal/config"
	"storage_server/internal/model"
	"storage_server/internal/storage"
	"storage_server/internal/utils"
)

type FileHandler struct {
	store *storage.FileStore
	cfg   *config.Config
}

func NewFileHandler(store *storage.FileStore, cfg *config.Config) *FileHandler {
	return &FileHandler{
		store: store,
		cfg:   cfg,
	}
}

// resolveBaseURL mengambil base URL dari config atau mendeteksi dari request header host
func (h *FileHandler) resolveBaseURL(c *gin.Context) string {
	if h.cfg.BaseURL != "" {
		return h.cfg.BaseURL
	}
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

// UploadSingle mengunggah satu berkas
func (h *FileHandler) UploadSingle(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "File tidak ditemukan dalam form (field: 'file')", err.Error())
		return
	}

	maxBytes := h.cfg.MaxUploadSizeMB * 1024 * 1024
	if fileHeader.Size > maxBytes {
		utils.SendError(c, http.StatusRequestEntityTooLarge, fmt.Sprintf("Ukuran file melebihi batas maksimum (%d MB)", h.cfg.MaxUploadSizeMB), nil)
		return
	}

	folder := c.DefaultPostForm("folder", "general")
	customName := c.PostForm("name")
	baseURL := h.resolveBaseURL(c)

	meta, err := h.store.SaveUploadedFile(fileHeader, folder, customName, baseURL)
	if err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Gagal menyimpan file ke storage", err.Error())
		return
	}

	utils.SendSuccess(c, http.StatusCreated, "File berhasil diunggah", meta)
}

// UploadMultiple mengunggah beberapa berkas sekaligus
func (h *FileHandler) UploadMultiple(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "Form multipart tidak valid", err.Error())
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		// Cek fallback jika client mengirim dengan key "file" berulang
		files = form.File["file"]
	}

	if len(files) == 0 {
		utils.SendError(c, http.StatusBadRequest, "Tidak ada file yang dipilih (field: 'files')", nil)
		return
	}

	folder := c.DefaultPostForm("folder", "general")
	baseURL := h.resolveBaseURL(c)
	maxBytes := h.cfg.MaxUploadSizeMB * 1024 * 1024

	var uploaded []model.FileMeta
	var uploadErrors []string

	for _, fileHeader := range files {
		if fileHeader.Size > maxBytes {
			uploadErrors = append(uploadErrors, fmt.Sprintf("%s: ukuran melebihi batas", fileHeader.Filename))
			continue
		}

		meta, err := h.store.SaveUploadedFile(fileHeader, folder, "", baseURL)
		if err != nil {
			uploadErrors = append(uploadErrors, fmt.Sprintf("%s: gagal disimpan (%v)", fileHeader.Filename, err))
			continue
		}
		uploaded = append(uploaded, *meta)
	}

	response := gin.H{
		"total_requested": len(files),
		"total_uploaded":  len(uploaded),
		"files":           uploaded,
	}
	if len(uploadErrors) > 0 {
		response["errors"] = uploadErrors
	}

	utils.SendSuccess(c, http.StatusCreated, fmt.Sprintf("%d dari %d file berhasil diunggah", len(uploaded), len(files)), response)
}

// ListFiles mengambil daftar file dengan filter & pagination
func (h *FileHandler) ListFiles(c *gin.Context) {
	folder := c.Query("folder")
	search := c.Query("search")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	files, total := h.store.ListFiles(folder, search, page, limit)

	// Pastikan URL selalu up to date dengan host saat ini
	baseURL := h.resolveBaseURL(c)
	for i := range files {
		files[i].ViewURL = fmt.Sprintf("%s/api/v1/files/%s/view", baseURL, files[i].ID)
		files[i].DownloadURL = fmt.Sprintf("%s/api/v1/files/%s/download", baseURL, files[i].ID)
	}

	utils.SendSuccess(c, http.StatusOK, "Berhasil memuat daftar file", model.ListFilesResponse{
		Total: total,
		Limit: limit,
		Page:  page,
		Files: files,
	})
}

// GetFileInfo mengambil metadata berkas berdasarkan ID
func (h *FileHandler) GetFileInfo(c *gin.Context) {
	id := c.Param("id")
	meta, err := h.store.GetFile(id)
	if err != nil {
		if errors.Is(err, storage.ErrFileNotFound) {
			utils.SendError(c, http.StatusNotFound, "File tidak ditemukan", nil)
			return
		}
		utils.SendError(c, http.StatusInternalServerError, "Gagal mengambil data file", err.Error())
		return
	}

	baseURL := h.resolveBaseURL(c)
	copied := *meta
	copied.ViewURL = fmt.Sprintf("%s/api/v1/files/%s/view", baseURL, copied.ID)
	copied.DownloadURL = fmt.Sprintf("%s/api/v1/files/%s/download", baseURL, copied.ID)

	utils.SendSuccess(c, http.StatusOK, "Detail metadata file", copied)
}

// ViewFile menampilkan atau streaming file secara inline (cocok untuk Glide, Coil, ExoPlayer Android)
func (h *FileHandler) ViewFile(c *gin.Context) {
	id := c.Param("id")
	meta, err := h.store.GetFile(id)
	if err != nil {
		utils.SendError(c, http.StatusNotFound, "File tidak ditemukan", nil)
		return
	}

	filePath := h.store.GetPhysicalPath(meta)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		utils.SendError(c, http.StatusNotFound, "File fisik tidak ditemukan pada disk", nil)
		return
	}

	c.Header("Content-Type", meta.MimeType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", meta.OriginalName))
	c.Header("Accept-Ranges", "bytes")

	// http.ServeFile menangani HTTP Range (206 Partial Content) otomatis untuk video/audio streaming Android
	http.ServeFile(c.Writer, c.Request, filePath)
}

// DownloadFile mengunduh berkas sebagai attachment
func (h *FileHandler) DownloadFile(c *gin.Context) {
	id := c.Param("id")
	meta, err := h.store.GetFile(id)
	if err != nil {
		utils.SendError(c, http.StatusNotFound, "File tidak ditemukan", nil)
		return
	}

	filePath := h.store.GetPhysicalPath(meta)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		utils.SendError(c, http.StatusNotFound, "File fisik tidak ditemukan pada disk", nil)
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", meta.OriginalName))
	c.Header("Content-Type", meta.MimeType)
	c.Header("Accept-Ranges", "bytes")

	http.ServeFile(c.Writer, c.Request, filePath)
}

// DeleteFile menghapus berkas dari disk dan metadata
func (h *FileHandler) DeleteFile(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.DeleteFile(id); err != nil {
		if errors.Is(err, storage.ErrFileNotFound) {
			utils.SendError(c, http.StatusNotFound, "File tidak ditemukan", nil)
			return
		}
		utils.SendError(c, http.StatusInternalServerError, "Gagal menghapus file", err.Error())
		return
	}

	utils.SendSuccess(c, http.StatusOK, "File berhasil dihapus", gin.H{"id": id})
}
