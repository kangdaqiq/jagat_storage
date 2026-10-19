# 📱 Dokumentasi API Storage Server (Untuk Aplikasi Android)

Backend API Storage Server berperforma tinggi berbasis **Go (Gin)** yang dirancang khusus untuk integrasi aplikasi Android (upload berkas gambar/dokumen/video, streaming media, dan unduhan berkas).

---

## 🌐 1. Konfigurasi Host & Base URL Android

Saat menghubungkan aplikasi Android ke server lokal di komputer pengembang:

| Lingkungan Android | Host / Base URL | Keterangan |
| :--- | :--- | :--- |
| **Android Emulator (AVD)** | `http://10.0.2.2:8085` | Alias emulator khusus untuk mengakses `localhost` komputer host |
| **Device Fisik (WiFi LAN)** | `http://<IP-Laptop-Anda>:8085` | Contoh: `http://192.168.1.50:8085` (HP dan Laptop berada di WiFi yang sama) |
| **Produksi / VPS** | `https://storage.domainanda.com` | Menggunakan domain publik / reverse proxy Nginx |

> **Catatan Penting Android**:
> Jika menggunakan koneksi `http://` (belum HTTPS), pastikan menambahkan `android:usesCleartextTraffic="true"` pada tag `<application>` di `AndroidManifest.xml`.

---

## 🔐 2. Autentikasi

Autentikasi menggunakan **API Key** yang dapat dikirimkan melalui salah satu header berikut:

```http
X-API-Key: android-storage-secret-key
```
atau:
```http
Authorization: Bearer android-storage-secret-key
```

* Nilai default API Key: `android-storage-secret-key` (dapat diubah di file `.env`).
* Upload dan Hapus file memerlukan API Key.
* Preview & Download dapat diakses secara publik (direct URL) agar kompatibel dengan **Glide**, **Coil**, dan **ExoPlayer**.

---

## 📋 3. Format Respons Standar

Semua respons API menggunakan format JSON standar:

### Respons Berhasil (Success)
```json
{
  "success": true,
  "message": "Pesan deskripsi aksi",
  "data": { ... }
}
```

### Respons Gagal (Error)
```json
{
  "success": false,
  "message": "Pesan error",
  "error": { ... }
}
```

---

## 🚀 4. Daftar Lengkap Endpoint API

### 4.1 Health Check & Server Info
Memeriksa status server dan informasi limit storage.

- **URL**: `/api/v1/health`
- **Method**: `GET`
- **Auth**: Tidak diperlukan
- **Contoh Respons (200 OK)**:
```json
{
  "success": true,
  "message": "Storage server running properly",
  "data": {
    "service": "Android Storage Server",
    "status": "healthy",
    "version": "1.0.0",
    "uptime": "15m",
    "max_upload_size": "100.00 MB",
    "upload_directory": "./uploads"
  }
}
```

---

### 4.2 Upload File Tunggal (Single Upload)
Mengunggah satu berkas ke storage server.

- **URL**: `/api/v1/files/upload`
- **Method**: `POST`
- **Auth**: `X-API-Key`
- **Content-Type**: `multipart/form-data`
- **Form Fields**:
  - `file` *(Wajib, File)*: Berkas yang akan diunggah.
  - `folder` *(Opsional, String)*: Nama folder kategori (contoh: `avatars`, `chat_images`, `documents`). Default: `general`.
  - `name` *(Opsional, String)*: Nama berkas kustom jika ingin mengubah nama asli.

- **Contoh cURL**:
```bash
curl -X POST "http://localhost:8085/api/v1/files/upload" \
  -H "X-API-Key: android-storage-secret-key" \
  -F "file=@foto_profil.jpg" \
  -F "folder=avatars"
```

- **Contoh Respons (201 Created)**:
```json
{
  "success": true,
  "message": "File berhasil diunggah",
  "data": {
    "id": "8693267a-dfbb-4368-8813-d63e338252f9",
    "original_name": "foto_profil.jpg",
    "stored_name": "8693267a-dfbb-4368-8813-d63e338252f9.jpg",
    "folder": "avatars",
    "size": 245100,
    "human_size": "239.36 KB",
    "mime_type": "image/jpeg",
    "extension": ".jpg",
    "checksum_sha256": "d4e9cc44097204f8da3ada79816c38566cc338d39628c818fcb8129c6c4ef4eb",
    "view_url": "http://10.0.2.2:8085/api/v1/files/8693267a-dfbb-4368-8813-d63e338252f9/view",
    "download_url": "http://10.0.2.2:8085/api/v1/files/8693267a-dfbb-4368-8813-d63e338252f9/download",
    "created_at": "2026-10-19T15:24:20.033Z"
  }
}
```

---

### 4.3 Upload Banyak File Sekaligus (Multiple Upload)
Mengunggah beberapa berkas dalam satu request.

- **URL**: `/api/v1/files/upload-multiple`
- **Method**: `POST`
- **Auth**: `X-API-Key`
- **Content-Type**: `multipart/form-data`
- **Form Fields**:
  - `files` *(Wajib, File array)*: Daftar berkas yang diunggah.
  - `folder` *(Opsional, String)*: Nama folder tujuan.

- **Contoh cURL**:
```bash
curl -X POST "http://localhost:8085/api/v1/files/upload-multiple" \
  -H "X-API-Key: android-storage-secret-key" \
  -F "files=@gambar1.png" \
  -F "files=@gambar2.png" \
  -F "folder=gallery"
```

- **Contoh Respons (201 Created)**:
```json
{
  "success": true,
  "message": "2 dari 2 file berhasil diunggah",
  "data": {
    "total_requested": 2,
    "total_uploaded": 2,
    "files": [
      {
        "id": "11111111-2222-3333-4444-555555555555",
        "original_name": "gambar1.png",
        "view_url": "http://localhost:8085/api/v1/files/11111111-2222-3333-4444-555555555555/view"
      },
      {
        "id": "66666666-7777-8888-9999-000000000000",
        "original_name": "gambar2.png",
        "view_url": "http://localhost:8085/api/v1/files/66666666-7777-8888-9999-000000000000/view"
      }
    ]
  }
}
```

---

### 4.4 Melihat Daftar File (List Files)
Mendapatkan daftar berkas dengan filter dan paginasi.

- **URL**: `/api/v1/files`
- **Method**: `GET`
- **Auth**: Tidak diperlukan
- **Query Parameters**:
  - `folder` *(Opsional)*: Filter berdasarkan folder (contoh: `?folder=avatars`).
  - `search` *(Opsional)*: Cari nama berkas (contoh: `?search=profil`).
  - `page` *(Opsional, default: 1)*: Nomor halaman.
  - `limit` *(Opsional, default: 20)*: Jumlah per halaman.

- **Contoh Respons (200 OK)**:
```json
{
  "success": true,
  "message": "Berhasil memuat daftar file",
  "data": {
    "total": 1,
    "limit": 20,
    "page": 1,
    "files": [
      {
        "id": "8693267a-dfbb-4368-8813-d63e338252f9",
        "original_name": "foto_profil.jpg",
        "folder": "avatars",
        "size": 245100,
        "human_size": "239.36 KB",
        "mime_type": "image/jpeg",
        "view_url": "http://localhost:8085/api/v1/files/8693267a-dfbb-4368-8813-d63e338252f9/view",
        "download_url": "http://localhost:8085/api/v1/files/8693267a-dfbb-4368-8813-d63e338252f9/download"
      }
    ]
  }
}
```

---

### 4.5 Detail Metadata File
Mendapatkan metadata lengkap satu berkas berdasarkan ID.

- **URL**: `/api/v1/files/:id`
- **Method**: `GET`
- **Auth**: Tidak diperlukan

---

### 4.6 Preview / Streaming File (Inline)
Menampilkan berkas secara langsung atau streaming media (audio/video).

- **URL**: `/api/v1/files/:id/view`
- **Method**: `GET`
- **Auth**: Tidak diperlukan (secara default)
- **Header Respons**:
  - `Content-Type`: MIME type berkas (contoh: `image/jpeg`, `video/mp4`, `audio/mpeg`)
  - `Content-Disposition`: `inline; filename="..."`
  - `Accept-Ranges`: `bytes` (Mendukung seek video/audio & HTTP 206 Partial Content untuk Android MediaPlayer / ExoPlayer)

---

### 4.7 Unduh File (Download Attachment)
Memaksa browser atau Android DownloadManager untuk mengunduh berkas sebagai attachment.

- **URL**: `/api/v1/files/:id/download`
- **Method**: `GET`
- **Header Respons**:
  - `Content-Disposition`: `attachment; filename="nama_asli.ext"`

---

### 4.8 Hapus File
Menghapus berkas dari disk storage dan database metadata.

- **URL**: `/api/v1/files/:id`
- **Method**: `DELETE`
- **Auth**: `X-API-Key`
- **Contoh Respons (200 OK)**:
```json
{
  "success": true,
  "message": "File berhasil dihapus",
  "data": {
    "id": "8693267a-dfbb-4368-8813-d63e338252f9"
  }
}
```

---

## 📱 5. Contoh Integrasi Android (Kotlin)

### 5.1 Model Data Kotlin

```kotlin
data class ApiResponse<T>(
    val success: Boolean,
    val message: String,
    val data: T?
)

data class FileMeta(
    val id: String,
    val original_name: String,
    val folder: String,
    val size: Long,
    val human_size: String,
    val mime_type: String,
    val view_url: String,
    val download_url: String
)
```

### 5.2 Retrofit Interface

```kotlin
import okhttp3.MultipartBody
import okhttp3.RequestBody
import retrofit2.Response
import retrofit2.http.*

interface StorageApiService {
    @Multipart
    @POST("api/v1/files/upload")
    suspend fun uploadFile(
        @Header("X-API-Key") apiKey: String = "android-storage-secret-key",
        @Part file: MultipartBody.Part,
        @Part("folder") folder: RequestBody? = null
    ): Response<ApiResponse<FileMeta>>

    @GET("api/v1/files")
    suspend fun getFiles(
        @Query("folder") folder: String? = null,
        @Query("page") page: Int = 1,
        @Query("limit") limit: Int = 20
    ): Response<ApiResponse<Map<String, Any>>>

    @DELETE("api/v1/files/{id}")
    suspend fun deleteFile(
        @Header("X-API-Key") apiKey: String = "android-storage-secret-key",
        @Path("id") fileId: String
    ): Response<ApiResponse<Map<String, String>>>
}
```

### 5.3 Fungsi Upload Menggunakan OkHttp / Retrofit

```kotlin
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.MultipartBody
import okhttp3.RequestBody.Companion.asRequestBody
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.File

suspend fun uploadImageFile(service: StorageApiService, localFile: File) {
    val mimeType = "image/jpeg".toMediaTypeOrNull()
    val requestFile = localFile.asRequestBody(mimeType)
    val bodyPart = MultipartBody.Part.createFormData("file", localFile.name, requestFile)
    val folderPart = "avatars".toRequestBody("text/plain".toMediaTypeOrNull())

    val response = service.uploadFile(
        file = bodyPart,
        folder = folderPart
    )

    if (response.isSuccessful && response.body()?.success == true) {
        val uploadedFile = response.body()?.data
        println("Upload sukses! URL Gambar: ${uploadedFile?.view_url}")
    } else {
        println("Gagal upload: ${response.errorBody()?.string()}")
    }
}
```

### 5.4 Menampilkan Gambar dengan Glide atau Coil

Cukup gunakan `view_url` langsung ke ImageView:

```kotlin
// Menggunakan Glide
Glide.with(context)
    .load("http://10.0.2.2:8085/api/v1/files/8693267a-dfbb-4368-8813-d63e338252f9/view")
    .placeholder(R.drawable.ic_placeholder)
    .into(imageView)

// Atau menggunakan Coil (Jetpack Compose)
AsyncImage(
    model = fileMeta.view_url,
    contentDescription = "Gambar",
    modifier = Modifier.fillMaxWidth()
)
```

---

## ⚙️ 6. Variabel Lingkungan (`.env`)

Anda dapat menyesuaikan konfigurasi server pada file `.env`:

```env
PORT=8085
HOST=0.0.0.0
UPLOAD_DIR=./uploads
MAX_UPLOAD_SIZE_MB=100
API_KEY=android-storage-secret-key
REQUIRE_AUTH_FOR_UPLOAD=true
REQUIRE_AUTH_FOR_DOWNLOAD=false
```

---

## 🛠️ 7. Menjalankan Server

### Menjalankan langsung via Go:
```bash
go run ./cmd/server
```

### Menjalankan binary yang sudah dikompilasi:
```bash
.\storage_server.exe
```
