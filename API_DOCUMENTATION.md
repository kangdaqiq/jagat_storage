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

## 🔐 2. Autentikasi & Multi-User

Server mendukung arsitektur **Multi-User** di mana setiap pengguna memiliki **API Key unik** dan **Batas Kuota Penyimpanan (Storage Quota)** masing-masing.

Header yang didukung:
```http
X-API-Key: <api_key_user>
```
atau:
```http
Authorization: Bearer <api_key_user>
```
*(Bisa juga via parameter query `?api_key=<key>` untuk link langsung/streaming).*

* **Isolasi Berkas**: Setiap user hanya dapat melihat (`GET /api/v1/files`) dan menghapus (`DELETE /api/v1/files/:id`) berkas milik mereka sendiri.
* **Akun Admin Utama**: Dibuat otomatis saat inisialisasi menggunakan nilai `ADMIN_API_KEY` atau `API_KEY` di file `.env`. Admin memiliki hak untuk membuat user baru, mengatur batas kuota, serta melihat semua file.
* **Storage Quota Enforcement**: Jika ukuran file yang diunggah melebihi sisa kuota user, server akan menolak dengan status HTTP `413 Payload Too Large` beserta rincian kuota.
* **Preview & Download Publik**: Direct URL preview/download dapat diakses tanpa auth jika `REQUIRE_AUTH_FOR_DOWNLOAD=false` agar kompatibel dengan **Glide**, **Coil**, dan **ExoPlayer**.

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
    "thumbnail_url": "http://10.0.2.2:8085/api/v1/files/8693267a-dfbb-4368-8813-d63e338252f9/thumbnail",
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

### 4.7 Thumbnail Gambar & Video (Inline / Cover)
Mengambil gambar thumbnail ringan (JPEG) untuk video maupun gambar. Sangat optimal untuk galeri Web (`<img src="...">` / `<video poster="...">`) dan list/grid di Android Compose.

- **URL**: `/api/v1/files/:id/thumbnail`
- **Method**: `GET`
- **Auth**: Mengikuti aturan download (`REQUIRE_AUTH_FOR_DOWNLOAD`, default: publik)
- **Header Respons**:
  - `Content-Type`: `image/jpeg` (atau MIME gambar asli jika sumbernya gambar)
  - `Cache-Control`: `public, max-age=86400`
- **Catatan Teknis**:
  - Untuk berkas **video** (`.mp4`, `.mov`, `.webm`, dll): server Go otomatis memanggil `ffmpeg` untuk memotong 1 frame pada detik ke-1 dan menyimpannya sebagai thumbnail ringan (resolusi maks lebar 640px). Jika di server belum terinstal `ffmpeg`, server tetap berfungsi normal tanpa thumbnail.
  - Untuk berkas **gambar** (`.jpg`, `.png`, dll): endpoint ini langsung mengembalikan berkas gambar tersebut.

---

### 4.8 Unduh File (Download Attachment)
Memaksa browser atau Android DownloadManager untuk mengunduh berkas sebagai attachment.

- **URL**: `/api/v1/files/:id/download`
- **Method**: `GET`
- **Header Respons**:
  - `Content-Disposition`: `attachment; filename="nama_asli.ext"`

---

### 4.9 Hapus File
Menghapus berkas dari disk storage dan database metadata (termasuk thumbnail terkait jika ada) serta mengembalikan kapasitas kuota user.

- **URL**: `/api/v1/files/:id`
- **Method**: `DELETE`
- **Auth**: `X-API-Key`
- **Contoh Respons (200 OK)**:
```json
{
  "success": true,
  "message": "File berhasil dihapus",
  "data": {
    "id": "8693267a-dfbb-4368-8813-d63e338252f9",
    "name": "foto_profil.jpg",
    "size": "2.50 MB"
  }
}
```

---

### 4.10 Informasi Kuota Penyimpanan Pengguna (User Storage Info)
Mengambil informasi pemakaian kapasitas, sisa kuota, dan persentase penggunaan untuk akun user yang sedang aktif. Sangat ideal untuk menampilkan storage indicator / progress bar di aplikasi Android.

- **URL**: `/api/v1/user/storage`
- **Method**: `GET`
- **Auth**: `X-API-Key` (Wajib)
- **Contoh Respons (200 OK)**:
```json
{
  "success": true,
  "message": "Informasi kuota penyimpanan pengguna",
  "data": {
    "user_id": "usr_a1b2c3d4",
    "username": "budi_santoso",
    "role": "user",
    "quota_bytes": 1048576000,
    "quota_human": "1000.00 MB",
    "used_bytes": 262144000,
    "used_human": "250.00 MB",
    "free_bytes": 786432000,
    "free_human": "750.00 MB",
    "usage_percentage": 25.0,
    "total_files": 14
  }
}
```

---

### 4.11 Manajemen Pengguna & Kuota (Khusus Admin)
Endpoint khusus akun Administrator untuk mengelola pendaftaran user, batas kuota, dan API Key.

#### A. Daftar Seluruh Pengguna
- **URL**: `/api/v1/admin/users`
- **Method**: `GET`
- **Auth**: `X-API-Key` (Admin Key)

#### B. Buat Pengguna Baru
- **URL**: `/api/v1/admin/users`
- **Method**: `POST`
- **Auth**: `X-API-Key` (Admin Key)
- **Request Body (JSON)**:
```json
{
  "username": "budi_santoso",
  "quota_mb": 500,
  "api_key": "sk_budi_custom_token", 
  "role": "user"
}
```
*(Field `api_key`, `quota_mb`, dan `role` bersifat opsional. Jika dikosongkan, API key digenerate otomatis dan kuota mengikuti `DEFAULT_USER_QUOTA_MB`).*

#### C. Ubah Batas Kuota Pengguna
- **URL**: `/api/v1/admin/users/:id/quota`
- **Method**: `PUT`
- **Auth**: `X-API-Key` (Admin Key)
- **Request Body (JSON)**:
```json
{
  "quota_mb": 2000
}
```

#### D. Hapus Pengguna
- **URL**: `/api/v1/admin/users/:id`
- **Method**: `DELETE`
- **Auth**: `X-API-Key` (Admin Key)

#### E. Sinkronisasi Ulang Pemakaian Kuota (Maintenance)
- **URL**: `/api/v1/admin/users/recalculate`
- **Method**: `POST`
- **Auth**: `X-API-Key` (Admin Key)


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
    val download_url: String,
    val thumbnail_url: String? = null
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
        println("URL Thumbnail: ${uploadedFile?.thumbnail_url}")
    } else {
        println("Gagal upload: ${response.errorBody()?.string()}")
    }
}
```

### 5.4 Menampilkan Gambar / Cover Video (Android & Web)

Cukup gunakan `thumbnail_url ?: view_url`:

```kotlin
// Android Compose (Coil) — bisa menampilkan cover video maupun gambar langsung!
AsyncImage(
    model = fileMeta.thumbnail_url ?: fileMeta.view_url,
    contentDescription = "Cover Media",
    contentScale = ContentScale.Crop,
    modifier = Modifier.fillMaxWidth().height(200.dp)
)
```

```html
<!-- Web Dashboard / Galeri HTML -->
<!-- Menampilkan cover video dengan poster -->
<video controls poster="https://storage.domainanda.com/api/v1/files/VIDEO_ID/thumbnail">
    <source src="https://storage.domainanda.com/api/v1/files/VIDEO_ID/view" type="video/mp4">
</video>

<!-- Atau di kartu galeri Web -->
<img src="https://storage.domainanda.com/api/v1/files/VIDEO_ID/thumbnail" alt="Thumbnail Video" />
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
