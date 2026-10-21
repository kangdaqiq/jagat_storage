package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	ErrUserNotFound      = errors.New("pengguna tidak ditemukan")
	ErrUsernameExists    = errors.New("username sudah digunakan")
	ErrAPIKeyExists      = errors.New("API key sudah digunakan")
	ErrCannotDeleteAdmin = errors.New("tidak dapat menghapus akun admin utama")
	ErrQuotaExceeded     = errors.New("kapasitas penyimpanan melebihi batas kuota")
)

type UserStore struct {
	mu           sync.RWMutex
	filePath     string
	users        map[string]model.User // Key: User.ID
	apiKeyLookup map[string]string     // Key: APIKey -> Value: User.ID
}

// NewUserStore membuat instance UserStore baru dan memuat users.json
func NewUserStore(filePath string, defaultAdminAPIKey string, defaultQuotaMB int64) (*UserStore, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("gagal membuat direktori users.json: %w", err)
	}

	store := &UserStore{
		filePath:     filePath,
		users:        make(map[string]model.User),
		apiKeyLookup: make(map[string]string),
	}

	if err := store.load(); err != nil {
		// Jika file belum ada atau kosong, inisialisasi user default admin
		if errors.Is(err, os.ErrNotExist) || len(store.users) == 0 {
			if defaultAdminAPIKey == "" {
				defaultAdminAPIKey = generateRandomKey("admin_key_")
			}
			quotaBytes := defaultQuotaMB * 1024 * 1024

			adminUser := model.User{
				ID:         "usr_admin",
				Username:   "admin",
				APIKey:     defaultAdminAPIKey,
				Role:       "admin",
				QuotaBytes: quotaBytes,
				UsedBytes:  0,
				CreatedAt:  time.Now().UTC(),
				UpdatedAt:  time.Now().UTC(),
			}

			store.users[adminUser.ID] = adminUser
			store.apiKeyLookup[adminUser.APIKey] = adminUser.ID
			if err := store.save(); err != nil {
				return nil, fmt.Errorf("gagal menyimpan user admin default: %w", err)
			}
			fmt.Printf("[UserStore] Inisialisasi admin default: %s (Key: %s)\n", adminUser.Username, adminUser.APIKey)
		} else {
			return nil, fmt.Errorf("gagal membaca data pengguna: %w", err)
		}
	}

	return store, nil
}

func (s *UserStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}

	var list []model.User
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	s.users = make(map[string]model.User)
	s.apiKeyLookup = make(map[string]string)
	for _, u := range list {
		s.users[u.ID] = u
		s.apiKeyLookup[u.APIKey] = u.ID
	}
	return nil
}

func (s *UserStore) save() error {
	list := make([]model.User, 0, len(s.users))
	for _, u := range s.users {
		list = append(list, u)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

// GetUserByAPIKey mencari user berdasarkan API Key
func (s *UserStore) GetUserByAPIKey(apiKey string) (*model.User, error) {
	if apiKey == "" {
		return nil, ErrUserNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, exists := s.apiKeyLookup[apiKey]
	if !exists {
		return nil, ErrUserNotFound
	}

	user, exists := s.users[userID]
	if !exists {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

// GetUserByID mencari user berdasarkan ID
func (s *UserStore) GetUserByID(id string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

// GetUserByUsername mencari user berdasarkan username
func (s *UserStore) GetUserByUsername(username string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.users {
		if strings.EqualFold(u.Username, username) {
			return &u, nil
		}
	}
	return nil, ErrUserNotFound
}

// GetAllUsers mengambil daftar semua pengguna
func (s *UserStore) GetAllUsers() []model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]model.User, 0, len(s.users))
	for _, u := range s.users {
		list = append(list, u)
	}
	return list
}

// CreateUser membuat user baru dengan kuota penyimpanan
func (s *UserStore) CreateUser(req model.CreateUserRequest, fallbackQuotaMB int64) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanUsername := strings.TrimSpace(req.Username)
	if cleanUsername == "" {
		return nil, errors.New("username tidak boleh kosong")
	}

	// Validasi username unik
	for _, u := range s.users {
		if strings.EqualFold(u.Username, cleanUsername) {
			return nil, ErrUsernameExists
		}
	}

	// Buat atau validasi API Key
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		apiKey = generateRandomKey("sk_")
	} else {
		if _, exists := s.apiKeyLookup[apiKey]; exists {
			return nil, ErrAPIKeyExists
		}
	}

	// Tentukan kuota (jika 0 dan fallback > 0, gunakan fallback)
	quotaMB := req.QuotaMB
	if quotaMB <= 0 && fallbackQuotaMB > 0 {
		quotaMB = fallbackQuotaMB
	}
	quotaBytes := quotaMB * 1024 * 1024

	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role != "admin" {
		role = "user"
	}

	user := model.User{
		ID:         fmt.Sprintf("usr_%s", uuid.New().String()[:8]),
		Username:   cleanUsername,
		APIKey:     apiKey,
		Role:       role,
		QuotaBytes: quotaBytes,
		UsedBytes:  0,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	s.users[user.ID] = user
	s.apiKeyLookup[user.APIKey] = user.ID

	if err := s.save(); err != nil {
		delete(s.users, user.ID)
		delete(s.apiKeyLookup, user.APIKey)
		return nil, fmt.Errorf("gagal menyimpan pengguna: %w", err)
	}

	return &user, nil
}

// UpdateQuota mengubah batas kuota user
func (s *UserStore) UpdateQuota(userID string, quotaBytes int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[userID]
	if !exists {
		return ErrUserNotFound
	}

	user.QuotaBytes = quotaBytes
	user.UpdatedAt = time.Now().UTC()
	s.users[userID] = user

	return s.save()
}

// UpdateUsage memperbarui kapasitas terpakai (deltaBytes bisa positif saat upload atau negatif saat hapus)
func (s *UserStore) UpdateUsage(userID string, deltaBytes int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[userID]
	if !exists {
		return ErrUserNotFound
	}

	newUsed := user.UsedBytes + deltaBytes
	if newUsed < 0 {
		newUsed = 0
	}
	user.UsedBytes = newUsed
	user.UpdatedAt = time.Now().UTC()
	s.users[userID] = user

	return s.save()
}

// SetUsage mengubah pemakaian langsung (misal saat rekalkulasi)
func (s *UserStore) SetUsage(userID string, usedBytes int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[userID]
	if !exists {
		return ErrUserNotFound
	}

	if usedBytes < 0 {
		usedBytes = 0
	}
	user.UsedBytes = usedBytes
	user.UpdatedAt = time.Now().UTC()
	s.users[userID] = user

	return s.save()
}

// CheckQuota mengecek apakah file baru dapat disimpan oleh user berdasarkan kuotanya
func (s *UserStore) CheckQuota(userID string, incomingBytes int64) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[userID]
	if !exists {
		return ErrUserNotFound
	}

	// 0 berarti unlimited quota
	if user.QuotaBytes <= 0 {
		return nil
	}

	if user.UsedBytes+incomingBytes > user.QuotaBytes {
		return ErrQuotaExceeded
	}

	return nil
}

// DeleteUser menghapus user
func (s *UserStore) DeleteUser(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[userID]
	if !exists {
		return ErrUserNotFound
	}

	if user.Role == "admin" && user.ID == "usr_admin" {
		return ErrCannotDeleteAdmin
	}

	delete(s.apiKeyLookup, user.APIKey)
	delete(s.users, userID)

	return s.save()
}

// GetStorageInfo menghitung ringkasan pemakaian dan sisa kuota user
func (s *UserStore) GetStorageInfo(userID string, totalFiles int) (*model.UserStorageInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[userID]
	if !exists {
		return nil, ErrUserNotFound
	}

	var quotaHuman string
	var freeBytes int64
	var freeHuman string
	var percentage float64

	if user.QuotaBytes <= 0 {
		quotaHuman = "Unlimited"
		freeBytes = -1
		freeHuman = "Unlimited"
		percentage = 0
	} else {
		quotaHuman = utils.FormatFileSize(user.QuotaBytes)
		freeBytes = user.QuotaBytes - user.UsedBytes
		if freeBytes < 0 {
			freeBytes = 0
		}
		freeHuman = utils.FormatFileSize(freeBytes)
		if user.QuotaBytes > 0 {
			percentage = (float64(user.UsedBytes) / float64(user.QuotaBytes)) * 100.0
			if percentage > 100.0 {
				percentage = 100.0
			}
		}
	}

	return &model.UserStorageInfo{
		UserID:          user.ID,
		Username:        user.Username,
		Role:            user.Role,
		QuotaBytes:      user.QuotaBytes,
		QuotaHuman:      quotaHuman,
		UsedBytes:       user.UsedBytes,
		UsedHuman:       utils.FormatFileSize(user.UsedBytes),
		FreeBytes:       freeBytes,
		FreeHuman:       freeHuman,
		UsagePercentage: float64(int(percentage*100)) / 100, // 2 desimal
		TotalFiles:      totalFiles,
	}, nil
}

func generateRandomKey(prefix string) string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	return fmt.Sprintf("%s%s", prefix, hex.EncodeToString(bytes))
}
