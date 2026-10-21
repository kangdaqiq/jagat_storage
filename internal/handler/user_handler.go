package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"storage_server/internal/config"
	"storage_server/internal/middleware"
	"storage_server/internal/model"
	"storage_server/internal/storage"
	"storage_server/internal/utils"
)

type UserHandler struct {
	userStore *storage.UserStore
	fileStore *storage.FileStore
	cfg       *config.Config
}

func NewUserHandler(userStore *storage.UserStore, fileStore *storage.FileStore, cfg *config.Config) *UserHandler {
	return &UserHandler{
		userStore: userStore,
		fileStore: fileStore,
		cfg:       cfg,
	}
}

// GetMyStorage mengambil informasi kuota dan pemakaian penyimpanan untuk user yang sedang aktif
func (h *UserHandler) GetMyStorage(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		utils.SendError(c, http.StatusUnauthorized, "Unauthorized: Pengguna tidak terotentikasi", nil)
		return
	}

	totalFiles := h.fileStore.CountFilesByUser(user.ID)
	info, err := h.userStore.GetStorageInfo(user.ID, totalFiles)
	if err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Gagal mengambil informasi kuota", err.Error())
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Informasi kuota penyimpanan pengguna", info)
}

// ListUsers (Admin) mengambil seluruh daftar pengguna terdaftar beserta statistik kuota
func (h *UserHandler) ListUsers(c *gin.Context) {
	users := h.userStore.GetAllUsers()
	var result []model.UserStorageInfo

	for _, u := range users {
		totalFiles := h.fileStore.CountFilesByUser(u.ID)
		info, _ := h.userStore.GetStorageInfo(u.ID, totalFiles)
		if info != nil {
			result = append(result, *info)
		}
	}

	utils.SendSuccess(c, http.StatusOK, "Daftar seluruh pengguna", result)
}

// CreateUser (Admin) membuat akun user baru beserta batas kuota dan API key
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req model.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Payload tidak valid (perlu field 'username')", err.Error())
		return
	}

	newUser, err := h.userStore.CreateUser(req, h.cfg.DefaultUserQuotaMB)
	if err != nil {
		if errors.Is(err, storage.ErrUsernameExists) || errors.Is(err, storage.ErrAPIKeyExists) {
			utils.SendError(c, http.StatusConflict, err.Error(), nil)
			return
		}
		utils.SendError(c, http.StatusInternalServerError, "Gagal membuat pengguna", err.Error())
		return
	}

	totalFiles := h.fileStore.CountFilesByUser(newUser.ID)
	info, _ := h.userStore.GetStorageInfo(newUser.ID, totalFiles)

	response := gin.H{
		"user":         newUser,
		"storage_info": info,
	}

	utils.SendSuccess(c, http.StatusCreated, "Pengguna berhasil dibuat", response)
}

// UpdateUserQuota (Admin) memperbarui kuota penyimpanan user (dalam Megabytes)
func (h *UserHandler) UpdateUserQuota(c *gin.Context) {
	userID := c.Param("id")
	var req model.UpdateQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Payload tidak valid (perlu field 'quota_mb')", err.Error())
		return
	}

	quotaBytes := req.QuotaMB * 1024 * 1024
	if err := h.userStore.UpdateQuota(userID, quotaBytes); err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			utils.SendError(c, http.StatusNotFound, "Pengguna tidak ditemukan", nil)
			return
		}
		utils.SendError(c, http.StatusInternalServerError, "Gagal mengubah kuota", err.Error())
		return
	}

	totalFiles := h.fileStore.CountFilesByUser(userID)
	info, _ := h.userStore.GetStorageInfo(userID, totalFiles)

	utils.SendSuccess(c, http.StatusOK, "Batas kuota berhasil diperbarui", info)
}

// DeleteUser (Admin) menghapus user dari sistem
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	if err := h.userStore.DeleteUser(userID); err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			utils.SendError(c, http.StatusNotFound, "Pengguna tidak ditemukan", nil)
			return
		}
		if errors.Is(err, storage.ErrCannotDeleteAdmin) {
			utils.SendError(c, http.StatusBadRequest, err.Error(), nil)
			return
		}
		utils.SendError(c, http.StatusInternalServerError, "Gagal menghapus pengguna", err.Error())
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Pengguna berhasil dihapus", gin.H{"user_id": userID})
}

// RecalculateAllUsage (Admin) menyinkronkan pemakaian disk aktual tiap user dari metadata
func (h *UserHandler) RecalculateAllUsage(c *gin.Context) {
	users := h.userStore.GetAllUsers()
	for _, u := range users {
		actualBytes := h.fileStore.GetTotalBytesByUser(u.ID)
		_ = h.userStore.SetUsage(u.ID, actualBytes)
	}

	utils.SendSuccess(c, http.StatusOK, "Sinkronisasi kapasitas storage seluruh user selesai", gin.H{
		"total_users": len(users),
	})
}
