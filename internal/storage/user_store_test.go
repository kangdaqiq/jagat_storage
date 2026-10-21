package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"storage_server/internal/model"
	"storage_server/internal/storage"
)

func TestUserStore_CRUD_And_Quota(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "user_store_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	usersFile := filepath.Join(tempDir, "users.json")
	store, err := storage.NewUserStore(usersFile, "test-admin-key", 100)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}

	// 1. Verify default admin user was created
	admin, err := store.GetUserByAPIKey("test-admin-key")
	if err != nil {
		t.Fatalf("failed to get admin by key: %v", err)
	}
	if admin.Role != "admin" {
		t.Errorf("expected role admin, got %s", admin.Role)
	}

	// 2. Create standard user with 10 MB quota
	newUser, err := store.CreateUser(model.CreateUserRequest{
		Username: "alice",
		QuotaMB:  10, // 10 MB = 10485760 bytes
	}, 100)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if newUser.Username != "alice" {
		t.Errorf("expected username alice, got %s", newUser.Username)
	}
	expectedQuota := int64(10 * 1024 * 1024)
	if newUser.QuotaBytes != expectedQuota {
		t.Errorf("expected quota %d, got %d", expectedQuota, newUser.QuotaBytes)
	}

	// 3. Test CheckQuota
	// 5 MB should be allowed
	if err := store.CheckQuota(newUser.ID, 5*1024*1024); err != nil {
		t.Errorf("5MB should be allowed, got error: %v", err)
	}

	// Simulate upload of 8 MB
	if err := store.UpdateUsage(newUser.ID, 8*1024*1024); err != nil {
		t.Fatalf("UpdateUsage failed: %v", err)
	}

	// Another 3 MB should exceed 10 MB quota (8MB + 3MB = 11MB > 10MB)
	if err := store.CheckQuota(newUser.ID, 3*1024*1024); err != storage.ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}

	// Another 1 MB should be allowed (8MB + 1MB = 9MB <= 10MB)
	if err := store.CheckQuota(newUser.ID, 1*1024*1024); err != nil {
		t.Errorf("1MB should be allowed, got error: %v", err)
	}

	// 4. Test GetStorageInfo
	info, err := store.GetStorageInfo(newUser.ID, 2)
	if err != nil {
		t.Fatalf("GetStorageInfo failed: %v", err)
	}
	if info.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", info.TotalFiles)
	}
	if info.FreeBytes != 2*1024*1024 {
		t.Errorf("expected 2MB free, got %d", info.FreeBytes)
	}

	// 5. Test Delete file / decrease usage
	if err := store.UpdateUsage(newUser.ID, -4*1024*1024); err != nil {
		t.Fatalf("UpdateUsage negative failed: %v", err)
	}
	u, _ := store.GetUserByID(newUser.ID)
	if u.UsedBytes != 4*1024*1024 {
		t.Errorf("expected 4MB used, got %d", u.UsedBytes)
	}

	// 6. Test Persistence: reload store from file
	store2, err := storage.NewUserStore(usersFile, "test-admin-key", 100)
	if err != nil {
		t.Fatalf("reloading store failed: %v", err)
	}
	uReloaded, err := store2.GetUserByID(newUser.ID)
	if err != nil {
		t.Fatalf("failed to find user after reload: %v", err)
	}
	if uReloaded.UsedBytes != 4*1024*1024 {
		t.Errorf("expected 4MB used after reload, got %d", uReloaded.UsedBytes)
	}
}
