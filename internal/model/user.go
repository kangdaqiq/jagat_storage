package model

import "time"

// User merepresentasikan data pengguna storage server
type User struct {
	ID         string    `json:"id"`
	Username   string    `json:"username"`
	APIKey     string    `json:"api_key"`
	Role       string    `json:"role"` // "admin" atau "user"
	QuotaBytes int64     `json:"quota_bytes"` // 0 = unlimited, > 0 = batasan kuota dalam byte
	UsedBytes  int64     `json:"used_bytes"`  // Total kapasitas yang sedang dipakai dalam byte
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// UserStorageInfo menyajikan status kuota dan pemakaian penyimpanan pengguna
type UserStorageInfo struct {
	UserID          string  `json:"user_id"`
	Username        string  `json:"username"`
	Role            string  `json:"role"`
	QuotaBytes      int64   `json:"quota_bytes"`
	QuotaHuman      string  `json:"quota_human"`
	UsedBytes       int64   `json:"used_bytes"`
	UsedHuman       string  `json:"used_human"`
	FreeBytes       int64   `json:"free_bytes"`
	FreeHuman       string  `json:"free_human"`
	UsagePercentage float64 `json:"usage_percentage"`
	TotalFiles      int     `json:"total_files"`
}

// CreateUserRequest payload untuk membuat user baru
type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	QuotaMB  int64  `json:"quota_mb"`           // Batas kuota dalam MB (misal 500 untuk 500 MB). 0 = unlimited
	APIKey   string `json:"api_key,omitempty"`  // Opsional, jika kosong akan digenerate otomatis
	Role     string `json:"role,omitempty"`     // "user" (default) atau "admin"
}

// UpdateQuotaRequest payload untuk update batas kuota user
type UpdateQuotaRequest struct {
	QuotaMB int64 `json:"quota_mb"` // Batas kuota baru dalam MB (0 = unlimited)
}
