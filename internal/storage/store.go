package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"storage_server/internal/model"
	"storage_server/internal/utils"
)

var (
	ErrFileNotFound      = errors.New("file not found")
	ErrThumbnailNotFound = errors.New("thumbnail not found")
	ErrInvalidPath       = errors.New("invalid file path")
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
func (s *FileStore) SaveUploadedFile(fileHeader *multipart.FileHeader, folder string, customName string, hostURL string, userID string) (*model.FileMeta, error) {
	cleanFolder := sanitizeFolderName(folder)

	var targetDir string
	if userID != "" {
		targetDir = filepath.Join(s.baseDir, userID, cleanFolder)
	} else {
		targetDir = filepath.Join(s.baseDir, cleanFolder)
	}

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

	var thumbName string
	var thumbURL string

	if IsVideoFile(mimeType, ext) {
		candidateThumb := fmt.Sprintf("%s_thumb.jpg", fileID)
		thumbDestPath := filepath.Join(targetDir, candidateThumb)
		if err := GenerateVideoThumbnail(destPath, thumbDestPath); err != nil {
			log.Printf("[Thumbnail] Info: Gagal membuat thumbnail video %s: %v", origName, err)
		} else {
			thumbName = candidateThumb
			thumbURL = fmt.Sprintf("%s/api/v1/files/%s/thumbnail", hostURL, fileID)
			log.Printf("[Thumbnail] Berhasil membuat thumbnail video: %s", candidateThumb)
		}
	} else if IsImageFile(mimeType, ext) {
		thumbURL = fmt.Sprintf("%s/api/v1/files/%s/thumbnail", hostURL, fileID)
	}

	meta := model.FileMeta{
		ID:            fileID,
		UserID:        userID,
		OriginalName:  origName,
		StoredName:    storedName,
		ThumbnailName: thumbName,
		Folder:        cleanFolder,
		Size:          copiedSize,
		HumanSize:     utils.FormatFileSize(copiedSize),
		MimeType:      mimeType,
		Extension:     ext,
		ChecksumSHA:   shaChecksum,
		ViewURL:       viewURL,
		DownloadURL:   downloadURL,
		ThumbnailURL:  thumbURL,
		CreatedAt:     time.Now().UTC(),
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
	if meta.UserID != "" {
		p := filepath.Join(s.baseDir, meta.UserID, meta.Folder, meta.StoredName)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join(s.baseDir, meta.Folder, meta.StoredName)
}

// GetPhysicalThumbnailPath mengambil path file thumbnail di disk
func (s *FileStore) GetPhysicalThumbnailPath(meta *model.FileMeta) (string, string, error) {
	if meta.ThumbnailName != "" {
		if meta.UserID != "" {
			p := filepath.Join(s.baseDir, meta.UserID, meta.Folder, meta.ThumbnailName)
			if _, err := os.Stat(p); err == nil {
				return p, "image/jpeg", nil
			}
		}
		p := filepath.Join(s.baseDir, meta.Folder, meta.ThumbnailName)
		if _, err := os.Stat(p); err == nil {
			return p, "image/jpeg", nil
		}
	}

	// Jika file asli adalah gambar, langsung gunakan file aslinya
	if IsImageFile(meta.MimeType, meta.Extension) {
		p := s.GetPhysicalPath(meta)
		if _, err := os.Stat(p); err == nil {
			return p, meta.MimeType, nil
		}
	}

	return "", "", ErrThumbnailNotFound
}

func (s *FileStore) ListFiles(userID, folder, search string, page, limit int) ([]model.FileMeta, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []model.FileMeta
	for _, f := range s.files {
		if userID != "" && f.UserID != userID {
			continue
		}
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

func (s *FileStore) CountFilesByUser(userID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, f := range s.files {
		if userID == "" || f.UserID == userID {
			count++
		}
	}
	return count
}

func (s *FileStore) GetTotalBytesByUser(userID string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total int64
	for _, f := range s.files {
		if userID == "" || f.UserID == userID {
			total += f.Size
		}
	}
	return total
}

func (s *FileStore) DeleteFile(id string) (*model.FileMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, exists := s.files[id]
	if !exists {
		return nil, ErrFileNotFound
	}

	filePath := filepath.Join(s.baseDir, meta.Folder, meta.StoredName)
	if meta.UserID != "" {
		userPath := filepath.Join(s.baseDir, meta.UserID, meta.Folder, meta.StoredName)
		if _, err := os.Stat(userPath); err == nil {
			filePath = userPath
		}
	}

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Hapus thumbnail jika ada
	if meta.ThumbnailName != "" {
		if meta.UserID != "" {
			_ = os.Remove(filepath.Join(s.baseDir, meta.UserID, meta.Folder, meta.ThumbnailName))
		}
		_ = os.Remove(filepath.Join(s.baseDir, meta.Folder, meta.ThumbnailName))
	}

	delete(s.files, id)
	_ = s.saveMetadata()
	return &meta, nil
}

// IsVideoFile mengecek apakah MIME type atau ekstensi adalah format video
func IsVideoFile(mimeType, ext string) bool {
	if strings.HasPrefix(strings.ToLower(mimeType), "video/") {
		return true
	}
	switch strings.ToLower(ext) {
	case ".mp4", ".mov", ".mkv", ".webm", ".avi", ".3gp", ".flv", ".wmv", ".m4v":
		return true
	}
	return false
}

// IsImageFile mengecek apakah MIME type atau ekstensi adalah format gambar
func IsImageFile(mimeType, ext string) bool {
	if strings.HasPrefix(strings.ToLower(mimeType), "image/") {
		return true
	}
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".svg":
		return true
	}
	return false
}

// GenerateVideoThumbnail mengekstrak 1 frame dari video menggunakan ffmpeg
func GenerateVideoThumbnail(videoPath, thumbPath string) error {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg tidak ditemukan di PATH: %w", err)
	}

	// Coba ambil frame di detik ke-1 dengan resolusi proporsional (maks lebar 640px)
	cmd := exec.Command(ffmpegPath, "-y", "-ss", "00:00:01", "-i", videoPath, "-vframes", "1", "-vf", "scale='min(640,iw)':-2", "-q:v", "3", thumbPath)
	if _, err := cmd.CombinedOutput(); err != nil {
		// Jika durasi video sangat pendek (<1 detik), coba ambil frame di detik 0
		cmdRetry := exec.Command(ffmpegPath, "-y", "-ss", "00:00:00", "-i", videoPath, "-vframes", "1", "-vf", "scale='min(640,iw)':-2", "-q:v", "3", thumbPath)
		if out2, err2 := cmdRetry.CombinedOutput(); err2 != nil {
			return fmt.Errorf("ffmpeg gagal: %v (detail: %s)", err2, string(out2))
		}
	}

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
