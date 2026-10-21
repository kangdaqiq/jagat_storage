package handler_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"storage_server/internal/config"
	"storage_server/internal/handler"
	"storage_server/internal/middleware"
	"storage_server/internal/model"
	"storage_server/internal/storage"
)

func setupTestServer(t *testing.T) (*gin.Engine, *storage.UserStore, string) {
	gin.SetMode(gin.TestMode)
	tempDir, err := os.MkdirTemp("", "handler_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := &config.Config{
		UploadDir:              tempDir,
		MaxUploadSizeMB:        10,
		APIKey:                 "admin-secret-key",
		AdminAPIKey:            "admin-secret-key",
		DefaultUserQuotaMB:     50,
		RequireAuthForUpload:   true,
		RequireAuthForDownload: false,
	}

	store, err := storage.NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	usersFile := filepath.Join(tempDir, "users.json")
	userStore, err := storage.NewUserStore(usersFile, cfg.AdminAPIKey, cfg.DefaultUserQuotaMB)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}

	r := gin.New()
	fileH := handler.NewFileHandler(store, userStore, cfg)
	userH := handler.NewUserHandler(userStore, store, cfg)

	api := r.Group("/api/v1")
	{
		userGroup := api.Group("/user", middleware.UserAuth(userStore, true))
		{
			userGroup.GET("/storage", userH.GetMyStorage)
		}

		adminGroup := api.Group("/admin", middleware.UserAuth(userStore, true), middleware.AdminOnly())
		{
			adminGroup.GET("/users", userH.ListUsers)
			adminGroup.POST("/users", userH.CreateUser)
			adminGroup.PUT("/users/:id/quota", userH.UpdateUserQuota)
			adminGroup.DELETE("/users/:id", userH.DeleteUser)
		}

		files := api.Group("/files")
		{
			uploadAuth := middleware.UserAuth(userStore, true)
			optionalAuth := middleware.UserAuth(userStore, false)
			actionAuth := middleware.UserAuth(userStore, true)

			files.POST("/upload", uploadAuth, fileH.UploadSingle)
			files.GET("", optionalAuth, fileH.ListFiles)
			files.GET("/:id", optionalAuth, fileH.GetFileInfo)
			files.DELETE("/:id", actionAuth, fileH.DeleteFile)
		}
	}

	return r, userStore, tempDir
}

func createMultipartUpload(fieldName, fileName string, content []byte) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile(fieldName, fileName)
	_, _ = io.Copy(part, bytes.NewReader(content))
	_ = writer.Close()
	return body, writer.FormDataContentType()
}

func TestMultiUser_FullFlow(t *testing.T) {
	router, userStore, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	// 1. Admin creates user "bob" with 1 MB quota (1048576 bytes)
	createReq := model.CreateUserRequest{
		Username: "bob",
		QuotaMB:  1,
	}
	reqBody, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/api/v1/admin/users", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "admin-secret-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for create user, got %d: %s", w.Code, w.Body.String())
	}

	bob, err := userStore.GetUserByUsername("bob")
	if err != nil {
		t.Fatalf("failed to find bob: %v", err)
	}

	// 2. Bob checks his storage quota
	req = httptest.NewRequest("GET", "/api/v1/user/storage", nil)
	req.Header.Set("X-API-Key", bob.APIKey)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for bob storage, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Bob uploads 600 KB file (should succeed, within 1 MB quota)
	content600KB := bytes.Repeat([]byte("A"), 600*1024)
	uploadBody, contentType := createMultipartUpload("file", "test600k.txt", content600KB)

	req = httptest.NewRequest("POST", "/api/v1/files/upload", uploadBody)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-API-Key", bob.APIKey)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for bob upload, got %d: %s", w.Code, w.Body.String())
	}

	var uploadResp struct {
		Data model.FileMeta `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &uploadResp)
	uploadedFileID := uploadResp.Data.ID

	// Verify bob's quota usage is updated
	bobUpdated, _ := userStore.GetUserByID(bob.ID)
	if bobUpdated.UsedBytes != 600*1024 {
		t.Errorf("expected 600KB used, got %d", bobUpdated.UsedBytes)
	}

	// 4. Bob uploads another 600 KB file (should FAIL with 413, because 600KB + 600KB = 1.2MB > 1MB)
	uploadBody2, contentType2 := createMultipartUpload("file", "test600k_2.txt", content600KB)
	req = httptest.NewRequest("POST", "/api/v1/files/upload", uploadBody2)
	req.Header.Set("Content-Type", contentType2)
	req.Header.Set("X-API-Key", bob.APIKey)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 Quota Exceeded, got %d: %s", w.Code, w.Body.String())
	}

	// 5. Admin creates user "alice", alice uploads a file, and verify Bob cannot see or delete alice's file
	alice, _ := userStore.CreateUser(model.CreateUserRequest{Username: "alice", QuotaMB: 10}, 10)
	aliceContent := []byte("alice private file")
	aliceBody, aliceContentType := createMultipartUpload("file", "alice.txt", aliceContent)

	req = httptest.NewRequest("POST", "/api/v1/files/upload", aliceBody)
	req.Header.Set("Content-Type", aliceContentType)
	req.Header.Set("X-API-Key", alice.APIKey)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var aliceUploadResp struct {
		Data model.FileMeta `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &aliceUploadResp)
	aliceFileID := aliceUploadResp.Data.ID

	// Bob lists files -> should only see 1 file (his own)
	req = httptest.NewRequest("GET", "/api/v1/files", nil)
	req.Header.Set("X-API-Key", bob.APIKey)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var listResp struct {
		Data model.ListFilesResponse `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &listResp)
	if listResp.Data.Total != 1 {
		t.Errorf("bob should only see 1 file, got %d", listResp.Data.Total)
	}

	// Bob attempts to delete Alice's file -> should be 403 Forbidden
	req = httptest.NewRequest("DELETE", "/api/v1/files/"+aliceFileID, nil)
	req.Header.Set("X-API-Key", bob.APIKey)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden when deleting another user's file, got %d: %s", w.Code, w.Body.String())
	}

	// 6. Bob deletes his own file -> quota usage should drop back to 0
	req = httptest.NewRequest("DELETE", "/api/v1/files/"+uploadedFileID, nil)
	req.Header.Set("X-API-Key", bob.APIKey)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on deleting own file, got %d: %s", w.Code, w.Body.String())
	}

	bobAfterDelete, _ := userStore.GetUserByID(bob.ID)
	if bobAfterDelete.UsedBytes != 0 {
		t.Errorf("expected 0 bytes used after deletion, got %d", bobAfterDelete.UsedBytes)
	}
}
