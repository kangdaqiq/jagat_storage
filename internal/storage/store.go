package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"storage_server/internal/model"
	"storage_server/internal/utils"
)

var (
	ErrFileNotFound = errors.New("file not found")
	ErrInvalidPath  = errors.New("invalid file path")
)

type FileStore struct {
	mu           sync.RWMutex
	baseDir      string
	metadataFile string
	files        map[string]model.FileMeta
}

func NewFileStore(baseDir string) (*FileStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("gagal membuat direktori upload: %w", err)
	}

	metadataPath := filepath.Join(baseDir, "metadata.json")
	store := &FileStore{
		baseDir:      baseDir,
		metadataFile: metadataPath,
		files:        make(map[string]model.FileMeta),
	}

	// Muat metadata yang sudah ada jika berkas ditemukan
	if err := store.loadMetadata(); err != nil {
		fmt.Printf("[Storage] Info: Inisialisasi metadata baru (%v)\n", err)
	}

	return store, nil
}

func (s *FileStore) loadMetadata() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.metadataFile)
	if err != nil {
		return err
	}

	var list []model.FileMeta
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	for _, meta := range list {
		s.files[meta.ID] = meta
	}
	return nil
}

func (s *FileStore) saveMetadata() error {
	list := make([]model.FileMeta, 0, len(s.files))
	for _, meta := range s.files {
		list = append(list, meta)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.metadataFile, data, 0644)
}

// sanitizeFolderName membersihkan nama folder dari karakter berbahaya dan path traversal
func sanitizeFolderName(folder string) string {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return "general"
	}
	// Ganti path separator dan cegah path traversal
	folder = strings.ReplaceAll(folder, "..", "")
	folder = strings.ReplaceAll(folder, "\\", "/")
	parts := strings.Split(folder, "/")
	var validParts []string
	for _, p := range parts {
		clean := strings.Trim(p, " .")
		if clean != "" {
			validParts = append(validParts, clean)
		}
	}
	if len(validParts) == 0 {
		return "general"
	}
	return strings.Join(validParts, "/")
}

// SaveUploadedFile menyimpan file multipart ke disk dan mencatat metadata
func (s *FileStore) SaveUploadedFile(fileHeader *multipart.FileHeader, folder string, customName string, hostURL string) (*model.FileMeta, error) {
	cleanFolder := sanitizeFolderName(folder)
	targetDir := filepath.Join(s.baseDir, cleanFolder)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("gagal membuat folder target: %w", err)
	}

	fileID := uuid.New().String()
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	origName := filepath.Base(fileHeader.Filename)

	var storedName string
	if customName != "" {
		cleanCustom := filepath.Base(customName)
		if filepath.Ext(cleanCustom) == "" && ext != "" {
			cleanCustom += ext
		}
		storedName = fmt.Sprintf("%s_%s", fileID[:8], cleanCustom)
	} else {
		storedName = fmt.Sprintf("%s%s", fileID, ext)
	}

	destPath := filepath.Join(targetDir, storedName)

	src, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	dest, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	defer dest.Close()

	hasher := sha256.New()
	multiWriter := io.MultiWriter(dest, hasher)

	copiedSize, err := io.Copy(multiWriter, src)
	if err != nil {
		_ = os.Remove(destPath)
		return nil, err
	}

	shaChecksum := hex.EncodeToString(hasher.Sum(nil))

	// Deteksi MIME type
	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = detectMimeType(destPath)
	}

	// URL Builder
	viewURL := fmt.Sprintf("%s/api/v1/files/%s/view", hostURL, fileID)
	downloadURL := fmt.Sprintf("%s/api/v1/files/%s/download", hostURL, fileID)

	meta := model.FileMeta{
		ID:           fileID,
		OriginalName: origName,
		StoredName:   storedName,
		Folder:       cleanFolder,
		Size:         copiedSize,
		HumanSize:    utils.FormatFileSize(copiedSize),
		MimeType:     mimeType,
		Extension:    ext,
		ChecksumSHA:  shaChecksum,
		ViewURL:      viewURL,
		DownloadURL:  downloadURL,
		CreatedAt:    time.Now().UTC(),
	}

	s.mu.Lock()
	s.files[fileID] = meta
	_ = s.saveMetadata()
	s.mu.Unlock()

	return &meta, nil
}

func (s *FileStore) GetFile(id string) (*model.FileMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	meta, exists := s.files[id]
	if !exists {
		return nil, ErrFileNotFound
	}
	return &meta, nil
}

func (s *FileStore) GetPhysicalPath(meta *model.FileMeta) string {
	return filepath.Join(s.baseDir, meta.Folder, meta.StoredName)
}

func (s *FileStore) ListFiles(folder, search string, page, limit int) ([]model.FileMeta, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []model.FileMeta
	for _, f := range s.files {
		if folder != "" && !strings.EqualFold(f.Folder, folder) {
			continue
		}
		if search != "" {
			query := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(f.OriginalName), query) &&
				!strings.Contains(strings.ToLower(f.Folder), query) &&
				!strings.Contains(strings.ToLower(f.ID), query) {
				continue
			}
		}
		filtered = append(filtered, f)
	}

	total := len(filtered)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	start := (page - 1) * limit
	if start >= total {
		return []model.FileMeta{}, total
	}

	end := start + limit
	if end > total {
		end = total
	}

	return filtered[start:end], total
}

func (s *FileStore) DeleteFile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, exists := s.files[id]
	if !exists {
		return ErrFileNotFound
	}

	filePath := filepath.Join(s.baseDir, meta.Folder, meta.StoredName)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return err
	}

	delete(s.files, id)
	_ = s.saveMetadata()
	return nil
}

func detectMimeType(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return "application/octet-stream"
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "application/octet-stream"
	}

	return http.DetectContentType(buffer[:n])
}
