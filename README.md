Berikut adalah dokumentasi repositori GitHub (`README.md`) yang sangat lengkap, profesional, dan terstruktur untuk proyek **EcoPlan Backend**, disesuaikan dengan standar penulisan skripsi S1 Informatika serta menggunakan **Lisensi MIT**.

---

# 🌿 EcoPlan Backend API

**Backend Service untuk Platform Pengelolaan Sampah Berbasis Ekonomi Sirkular, Klasifikasi Citra AI, dan Rekening Bersama (Escrow)**

---

## 📖 Daftar Isi

1. Tentang Proyek
2. Arsitektur & Struktur Proyek
3. Fitur Utama Sistem
4. Persyaratan Sistem
5. Instalasi & Menjalankan Proyek
6. Konfigurasi Lingkungan (`.env`)
7. Dokumentasi Endpoints API
8. Pengujian (Testing)
9. Lisensi
---

## 🍃 Tentang Proyek

**EcoPlan** adalah platform digital berbasis web dan mobile yang mengintegrasikan kecerdasan buatan (*Artificial Intelligence*), ekonomi sirkular (*circular economy*), serta sistem e-commerce berkelanjutan. Repositori ini (`ecoplan-backend`) merupakan inti dari layanan server-side yang dibangun menggunakan bahasa **Go (Golang)** dengan *framework* **Gin**, serta didukung oleh basis data relasional **PostgreSQL / Supabase**.

Sistem backend ini dirancang untuk menangani berbagai modul krusial, antara lain:

* **AI Waste Detection Integration:** Meneruskan citra sampah ke microservice AI (ResNet50 / ONNX) untuk klasifikasi otomatis ke dalam 10 kategori sampah resmi.
* **Marketplace Daur Ulang & Kompos:** Memfasilitasi transaksi jual-beli barang daur ulang, sampah layak pakai, dan pupuk kompos.


* **Sistem Rekening Bersama (Escrow & Midtrans):** Menjamin keamanan transaksi finansial dengan integrasi *Payment Gateway* Midtrans Snap & Core API.


* **Logistik Terintegrasi (RajaOngkir Komerce v1):** Kalkulasi ongkos kirim multi-ekspedisi secara *real-time* dan pelacakan resi (*waybill tracking*).


* **Edukasi & Gamifikasi:** Manajemen artikel ilmiah/lingkungan dengan validasi skor Turnitin, video edukasi YouTube, serta sistem *reward Eco-Points*.



---

## 📁 Arsitektur & Struktur Proyek

Struktur direktori proyek dirancang menggunakan pola arsitektur berlapis (*layered architecture*) yang bersih dan termodulisasi:

```text
ecoplan-backend/
├── cmd/
│   └── main.go              # Titik masuk utama aplikasi (Entrypoint)
├── config/
│   ├── env.go               # Manajemen variabel lingkungan (.env)
│   ├── database.go          # Konfigurasi koneksi GORM & PostgreSQL
│   └── midtrans.go          # Inisialisasi Midtrans SDK (Snap & Core API)
├── services/
│   ├── rajaongkir_service.go # Integrasi logika API RajaOngkir Komerce v1
│   └── midtrans_service.go  # Logika bisnis pembayaran & webhook escrow
├── models/
│   ├── user.go              # Entitas data pengguna & RBAC (Admin, Seller, User)
│   ├── store.go             # Entitas toko penjual (Seller)
│   ├── product.go           # Entitas produk daur ulang / kompos
│   ├── transaction.go       # Entitas transaksi escrow & *transaction items*
│   ├── detection.go         # Entitas riwayat pemindaian sampah AI
│   ├── article.go           # Entitas artikel edukasi & skor Turnitin
│   └── video.go             # Entitas video edukasi YouTube
├── controllers/
│   ├── auth_controller.go   # Logika registrasi, login, & hashing bcrypt
│   ├── detection_controller.go # Pengolahan citra & komunikasi ke AI Microservice
│   ├── guide_controller.go  # Panduan 10 kategori sampah & integrasi Gemini AI
│   ├── article_controller.go # CRUD artikel, moderasi admin, & sistem view
│   ├── marketplace_controller.go # Manajemen toko & katalog produk daur ulang
│   ├── transaction_controller.go # Logika *checkout*, mutasi stok, & *escrow release*
│   ├── logistics_controller.go # Pencarian wilayah & kalkulasi ongkir publik
│   ├── profile_controller.go   # Manajemen profil, saldo, & statistik aktivitas
│   ├── payment_controller.go   # Webhook Midtrans & penarikan saldo toko (*withdrawal*)
│   ├── shipping_controller.go  # Input resi & pelacakan pengiriman berbasis transaksi
│   └── video_controller.go   # Manajemen video YouTube khusus Admin
├── middleware/
│   ├── auth_middleware.go   # Verifikasi JWT Bearer Token & RBAC
│   └── auth_middleware_test.go # Unit testing untuk keamanan middleware
├── routes/
│   └── api.go               # Definisi routing grup API v1 & middleware
├── uploads/                 # Direktori penyimpanan lokal citra deteksi sampah
├── api.http                 # Berkas pengujian REST Client (VS Code)
├── .env                     # Berkas konfigurasi lokal (ignore)
├── .env.example             # Template konfigurasi environment
├── .gitignore               # Aturan pengecualian Git
├── go.mod                   # Manajemen dependencies Go
└── go.sum                   # Checksum dependencies

```

---

## ⚡ Fitur Utama System

1. **Autentikasi & Otorisasi Berbasis JWT & RBAC:**
* Mendukung enkripsi kata sandi menggunakan `bcrypt`.
* Pembagian hak akses (*Role-Based Access Control*) yang ketat untuk tingkat `admin`, `seller`, dan `user`.




2. **Klasifikasi Sampah Cerdas (AI-Powered):**
* Mengunggah gambar sampah untuk dideteksi secara otomatis menggunakan model berbasis *ResNet50 ONNX*.
* Memberikan hadiah *Eco-Points* otomatis kepada pengguna yang aktif melakukan pemindaian.


3. **Sistem Rekening Bersama (Escrow):**
* Dana pembeli ditahan hingga pesanan dikonfirmasi selesai (`completed`) oleh pembeli atau admin, baru kemudian dicairkan secara otomatis ke saldo toko penjual (`Seller Balance`).


4. **Logistik Domestik Real-Time:**
* Terintegrasi dengan API RajaOngkir Komerce v1 untuk pencarian destinasi domestik (*autocomplete*) dan kalkulasi ongkir multi-ekspedisi (JNE, J&T, SiCepat, POS, dll.).





---

## 💻 Persyaratan Sistem

Pastikan perangkat Anda telah terinstal perangkat lunak berikut sebelum menjalankan proyek:

* **Go** versi 1.27.1 atau lebih baru.
* **PostgreSQL** (atau instance basis data cloud seperti Supabase).
* **Git**.

---

## 🚀 Instalasi & Menjalankan Proyek

1. **Kloning Repositori:**
```bash
git clone https://github.com/username/ecoplan-backend.git
cd ecoplan-backend

```


2. **Unduh Dependencies Go:**
```bash
go mod tidy

```


3. **Atur Berkas Konfigurasi Lingkungan (`.env`):**
Salin berkas `.env.example` menjadi `.env`, lalu sesuaikan kredensial basis data dan kunci API Anda.
```bash
cp .env.example .env

```


4. **Jalankan Aplikasi:**
```bash
go run cmd/main.go

```


*Server HTTP akan berjalan secara otomatis pada port `8080` (default).*

---

## 🔐 Konfigurasi Lingkungan (`.env`)

Buat berkas `.env` pada akar direktori proyek dengan format variabel berikut:

```env
PORT=8080
GIN_MODE=debug

# Konfigurasi Basis Data PostgreSQL / Supabase
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=db_name
DB_SSLMODE=disable
GIN_MODE=debug

# Konfigurasi Keamanan JWT
JWT_SECRET=ecoplan_jwt_secret_key_very_secure

# Konfigurasi Microservice AI & Gemini API
AI_SERVICE_URL=https://ecoplan-ai-service-production.up.railway.app
API_KEY_GEMINI=your_gemini_api_key

# Konfigurasi Midtrans Payment Gateway
MIDTRANS_ID=your_midtrans_merchant_id
MIDTRANS_SERVER_KEY=SB-Mid-server-xxxxxx
MIDTRANS_CLIENT_KEY=SB-Mid-client-xxxxxx
MIDTRANS_IS_PRODUCTION=false

# Konfigurasi RajaOngkir Komerce API
RAJAONGKIR_API_KEY=your_rajaongkir_api_key

```

---

## 📌 Dokumentasi Endpoints API

Seluruh endpoint dikelompokkan di bawah prefix `/api/v1`. Anda dapat menggunakan berkas `api.http` di dalam proyek bersama ekstensi *REST Client* (VS Code) untuk pengujian cepat.

### 1. Autentikasi (`/api/v1/auth`)

* `POST /api/v1/auth/register` — Pendaftaran akun baru (`user` / `seller`)
* `POST /api/v1/auth/login` — Otentikasi masuk dan penerbitan JWT Token

### 2. Deteksi Sampah AI (`/api/v1/detections`)

* `POST /api/v1/detections` — Unggah citra sampah untuk diklasifikasikan (Publik / Auth)


* `GET /api/v1/detections/history` — Riwayat deteksi pengguna (Protected)
* `GET /api/v1/detections/:id` — Detail hasil deteksi berdasarkan ID



### 3. Panduan & Gemini AI (`/api/v1/guides`)

* `GET /api/v1/guides` — Daftar lengkap panduan 10 kategori sampah resmi
* `GET /api/v1/guides/:category` — Panduan penanganan berdasarkan kategori spesifik
* `POST /api/v1/guides/ask` — Tanya jawab interaktif daur ulang dengan Gemini AI

### 4. Artikel Edukasi (`/api/v1/articles`)

* `GET /api/v1/articles` — Daftar artikel terpublikasi (Mendukung *search* & *pagination*)
* `POST /api/v1/articles` — Pengajuan draf artikel dengan skor Turnitin (Protected)


* `PATCH /api/v1/articles/:id/review` — Moderasi dan publikasi artikel (Khusus Admin)



### 5. Marketplace & Toko (`/api/v1/marketplace` & `/api/v1/stores`)

* `POST /api/v1/stores` — Registrasi toko baru (Mengaktifkan hak akses *Seller*)
* `GET /api/v1/marketplace/products` — Katalog produk daur ulang & kompos (Publik)
* `POST /api/v1/marketplace/products` — Tambah produk baru (Khusus Seller / Admin)

### 6. Transaksi Escrow & Pembayaran (`/api/v1/transactions` & `/api/v1/payment`)

* `POST /api/v1/transactions` — *Checkout* pesanan dan penerbitan *Midtrans Snap Token*
* `PATCH /api/v1/transactions/:id/status` — Perbarui status pesanan (*completed* mencairkan saldo penjual)
* `POST /api/v1/transactions/webhook` — *Callback* otomatis dari server Midtrans


* `POST /api/v1/payment/withdraw` — Penarikan saldo toko (*withdrawal*) ke rekening bank

### 7. Logistik & Pengiriman (`/api/v1/logistics` & `/api/v1/shipping`)

* `GET /api/v1/logistics/destinations` — Pencarian wilayah domestik (*autocomplete*)


* `POST /api/v1/shipping/calculate` — Kalkulasi estimasi ongkos kirim *checkout*

* `PATCH /api/v1/shipping/receipt/:transaction_id` — Input nomor resi oleh penjual (Mengubah status ke `shipped`)

---

## 🧪 Pengujian (Testing)

Proyek ini dilengkapi dengan *Unit Testing* untuk memastikan keandalan komponen middleware:

```bash
go test -v ./middleware/...

```

---

## 📝 Lisensi

Proyek ini dilisensikan di bawah ketentuan [MIT License](https://www.google.com/search?q=LICENSE). Anda bebas menggunakan, memodifikasi, dan mendistribusikan perangkat lunak ini untuk keperluan akademis maupun komersial dengan tetap mencantumkan atribusi pengembang asli.

---

*Dokumentasi ini dikelola sebagai bagian portfolio*