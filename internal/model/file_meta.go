package model

import "time"

// FileMeta menyimpan informasi metadata berkas yang diunggah
type FileMeta struct {
	ID           string    `json:"id"`
	OriginalName string    `json:"original_name"`
	StoredName   string    `json:"stored_name"`
	Folder       string    `json:"folder"`
	Size         int64     `json:"size"`
	HumanSize    string    `json:"human_size"`
	MimeType     string    `json:"mime_type"`
	Extension    string    `json:"extension"`
	ChecksumSHA   string    `json:"checksum_sha256,omitempty"`
	ThumbnailName string    `json:"thumbnail_name,omitempty"`
	ViewURL       string    `json:"view_url"`
	DownloadURL   string    `json:"download_url"`
	ThumbnailURL  string    `json:"thumbnail_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// UploadResponse respon standar untuk file upload
type UploadResponse struct {
	Message string    `json:"message"`
	File    FileMeta  `json:"file"`
}

// MultiUploadResponse respon untuk upload beberapa file sekaligus
type MultiUploadResponse struct {
	Message string     `json:"message"`
	Total   int        `json:"total"`
	Files   []FileMeta `json:"files"`
}

// ListFilesResponse respon untuk daftar berkas
type ListFilesResponse struct {
	Total int        `json:"total"`
	Limit int        `json:"limit"`
	Page  int        `json:"page"`
	Files []FileMeta `json:"files"`
}
