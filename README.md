# 🚀 Android Storage Server API

Storage server backend berkecepatan tinggi berbasis **Go (Gin)** yang dirancang untuk kebutuhan aplikasi Android (Upload, Download, Streaming Gambar/Audio/Video, dan Manajemen File).

## ⚡ Fitur Utama

- 📤 **Upload Berkas**: Mendukung Single & Multiple file upload (`multipart/form-data`) dengan pembagian folder kategori.
- 📺 **Media Streaming & Preview**: Kompatibel langsung dengan **Glide**, **Coil**, dan **ExoPlayer** (mendukung header `Range` / HTTP 206 Partial Content).
- 💾 **Direct Download**: Header `attachment` dengan nama asli berkas untuk Android `DownloadManager`.
- 👥 **Multi-User & Storage Quota**: Pembagian kapasitas storage per pengguna dengan batasan kuota (JSON-based zero-dependency) dan isolasi berkas antar user.
- 📊 **Storage Monitoring**: Endpoint khusus untuk cek sisa kuota, kapasitas terpakai, dan persentase penggunaan (cocok untuk progress bar storage Android).
- 🔑 **Autentikasi API Key Fleksibel**: Token unik per pengguna via `X-API-Key` atau `Authorization: Bearer <token>`.
- 🛡️ **Proteksi Path Traversal & Sanitasi**: Aman dari serangan manipulasi path direktori (`../`).
- 📁 **Penyimpanan Metadata Terpadu**: Menyimpan info ukuran berkas, checksum SHA256, mime-type, dan timestamp tanpa perlu dependensi database eksternal yang berat.

---

## 📖 Dokumentasi API Lengkap

Silakan buka file dokumentasi lengkap di:
👉 **[API_DOCUMENTATION.md](file:///e:/PROJECT/storage_server/API_DOCUMENTATION.md)**

---

## 🏃 Cara Menjalankan

1. Salin konfigurasi environment jika diperlukan:
   ```bash
   cp .env.example .env
   ```
2. Jalankan server:
   ```bash
   go run ./cmd/server
   ```
   *Atau jalankan file executable:*
   ```bash
   .\storage_server.exe
   ```

Server default berjalan di **`http://0.0.0.0:8085`**.

* Untuk **Android Emulator**, gunakan: `http://10.0.2.2:8085`
* Untuk **Android HP Fisik**, gunakan IP LAN WiFi Anda: `http://192.168.x.x:8085`
