# Jutec Go — Judol Intelligence Engine

Engine intelijen pendeteksi situs judi online (judol) berbasis Go. Melakukan crawling otomatis dengan headless Chrome, analisis multi-sinyal, ekstraksi artefak transaksi, pemetaan jaringan operator, dan enrichment via URLScan.io serta VirusTotal.

---

## Daftar Isi

- [Tujuan](#tujuan)
- [Arsitektur](#arsitektur)
- [Fitur Lengkap](#fitur-lengkap)
- [Pipeline Analisis](#pipeline-analisis)
- [Skema Database](#skema-database)
- [Setup](#setup)
- [API Reference](#api-reference)
- [Enrichment: URLScan & VirusTotal](#enrichment-urlscan--virustotal)
- [Background Jobs](#background-jobs)
- [Struktur Project](#struktur-project)
- [Variabel Lingkungan](#variabel-lingkungan)

---

## Tujuan

Operator judol di Indonesia beroperasi dalam jaringan — satu operator biasanya menjalankan puluhan hingga ratusan domain yang berbagi infrastruktur (IP, nameserver, SSL), nomor rekening, template HTML, atau platform hosting yang sama.

Jutec Go dirancang untuk:

1. **Mendeteksi** situs judol secara otomatis lewat analisis konten, network request, dan sinyal domain
2. **Menemukan** domain baru melalui pencarian Google, Certificate Transparency, dan sub-crawl
3. **Memetakan jaringan** antar situs via shared infrastructure (IP, NS, SSL, rekening, template SimHash)
4. **Mengidentifikasi operator** melalui tracking ID, platform UUID (AsiaSigma, GigaGaming), dan app distribution
5. **Memperkaya data** via URLScan.io (screenshot, tech stack) dan VirusTotal (reputasi 70+ engine)
6. **Menghasilkan intelligence** yang siap dipakai untuk pelaporan ke Kominfo atau lembaga terkait

---

## Arsitektur

```
┌──────────────────────────────────────────────────────────────────┐
│                        HTTP API (Gin)                            │
│  /analyze  /analyze/bulk  /discovery/*  /intelligence/*          │
│  /network/*  /artifacts/*  /keywords/*  /blacklist  /leak/*      │
└──────────────────────────┬───────────────────────────────────────┘
                           │
          ┌────────────────┼────────────────┐
          │                │                │
   ┌──────▼──────┐  ┌──────▼──────┐  ┌─────▼───────┐
   │   Crawler   │  │  Discovery  │  │  Infra Scan │
   │ (Chromedp   │  │ (Serper/    │  │ (DNS, SSL,  │
   │  headless)  │  │  crt.sh)    │  │  WHOIS)     │
   └──────┬──────┘  └──────┬──────┘  └─────┬───────┘
          │                │                │
          └────────────────┼────────────────┘
                           │
          ┌────────────────┼────────────────┐
          │                │                │
   ┌──────▼──────┐  ┌──────▼──────┐  ┌─────▼───────┐
   │   Neo4j     │  │  PostgreSQL │  │Redis+Asynq  │
   │  (Graph     │  │  (Source of │  │(Background  │
   │intelligence)│  │   Truth)    │  │   Jobs)     │
   └─────────────┘  └─────────────┘  └─────────────┘
                                             │
                              ┌──────────────┼──────────────┐
                              │              │              │
                       ┌──────▼─────┐ ┌──────▼──────┐ ┌────▼──────┐
                       │  URLScan   │ │  VirusTotal │ │    OCR    │
                       │ Enrichment │ │  Enrichment │ │  (Image)  │
                       └────────────┘ └─────────────┘ └───────────┘
```

| Komponen | Peran |
|---|---|
| **Chromedp** | Headless Chrome — crawl halaman JS-rendered, intercept network request |
| **PostgreSQL** | Source of truth — semua situs (gambling maupun tidak) |
| **Neo4j** | Graph database — hubungan antar situs judol via shared attributes |
| **Redis + Asynq** | Background job queue — OCR, enrichment URLScan/VT, async scan |
| **Serper API** | Google Search API untuk discovery domain |
| **crt.sh** | Certificate Transparency untuk menemukan subdomain/domain baru |
| **URLScan.io** | Screenshot, tech stack, verdict, history scan |
| **VirusTotal** | Reputasi domain/URL dari 70+ engine, analisis APK |

---

## Fitur Lengkap

### Crawling & Analisis Konten

- Headless Chrome via Chromedp (5 detik wait — JS-rendered content terambil)
- Intercept semua network request selama navigasi
- Scoring berbasis keyword berbobot dari database (bukan hardcode)
- Pattern matching: `livechat`, `whatsapp`, `tele`, `rtp-live`
- Analisis URL path & hostname (`/slot`, `/daftar`, `/deposit`, `gacor`, `maxwin`, dll)
- Sub-crawl hingga 10 external link per situs untuk enrichment artifact

### Composite Scoring

Skor akhir dihitung dari tiga komponen berbobot:

```
Final Score = Content(45%) + Network(30%) + Domain(25%)
```

| Sinyal | Kontribusi |
|---|---|
| Keyword score (dari DB) | Content |
| URL path/hostname terms | Content (+0.15/kata, max +0.45) |
| Game provider network request | Network (+0.3/provider) |
| SimHash template match dengan judol dikenal | Network (+0.35) |
| WHOIS domain risk score | Domain |
| TLD berisiko (`.top`, `.vip`, `.pw`, `.cc`, dll) | Domain (floor 0.40–0.50) |

### Ekstraksi Artefak Transaksi

Regex-based extraction dari HTML halaman dan gambar (via OCR):

| Tipe | Sumber | Contoh |
|---|---|---|
| **Bank** | BCA, Mandiri, BNI, BRI, CIMB, Danamon, BSI, BTN, Permata, Mega, Maybank, Bukopin, Sinarmas, OCBC, Panin, Neo, Jago | `BCA: 1234567890` |
| **E-Wallet** | DANA, OVO, GOPAY, LINKAJA, ShopeePay, Jenius, DOKU, iSaku, AstraPay | `DANA: 081234567890` |
| **Virtual Account** | Konteks VA + 13–17 digit | `VA BCA: 1234567890123` |
| **Crypto** | ETH/ERC20, BTC (legacy + SegWit), USDT TRC20 (Tron) | `0xAbCd...`, `T9xKm...` |
| **QRIS** | Merchant ID prefix `ID` + 13–15 digit | `ID1234567890123` |
| **Contact** | WhatsApp (`wa.me`), Telegram (`t.me`), Line, Discord | `wa.me/628xxx` |
| **Phone** | Nomor HP Indonesia dalam konteks pembayaran/kontak | `08xxxxxxxxxx` |

Nomor dengan prefix `0` otomatis dinormalisasi ke `628xxx` sebelum dikirim ke LeakOSINT.

### Deteksi Platform Judol

| Platform | Sinyal yang Dideteksi |
|---|---|
| **AsiaSigma** | CDN URL `cloudstoragesigma*.com/production/agent/{UUID}` — UUID unik per operator |
| **GigaGaming** | CDN URL `sitestatic.net/...__BRANDNAME__...` — brand code unik per operator |
| **Pragmatic Play** | Network request ke `pragmaticplay.com`, `pragmatic-external-platform.com` |
| **PG Soft** | Network request ke `pg-api.com`, `pgsoft.com` |
| **Evolution Gaming** | Network request ke `evolutiongaming.com`, `evolution.com` |
| +10 provider lain | Habanero, NetEnt, Playtech, CQ9, JDB, Fastspin, Nextspin, dll |

### Ekstraksi Link & Aplikasi

- `<a href>`, `<iframe src>`, `<img src>`, `data-*` attributes, JS string URLs
- Protocol-relative URLs (`//domain.com/...`)
- `<link rel="canonical/alternate/amphtml">` — domain terkait operator
- **APK/IPA detection** — `.apk`, `.ipa`, `.aab`, Play Store, App Store
- **URL shortener resolution** — `rebrand.ly`, `bit.ly`, `s.id`, dll yang mengarah ke APK di-resolve inline (timeout 3 detik)
- App install page detection — `*_install.html`, `/install/`, `download.html`

### Tracking ID Extraction

| Platform | Pattern |
|---|---|
| Google Analytics | `UA-XXXXXX-X` |
| Google Tag | `G-XXXXXXXX` |
| Google Tag Manager | `GTM-XXXXXXX` |
| Facebook Pixel | `fbq('init', 'XXXXXXXX')` |
| LiveChat | `license_id=XXXXXX` |
| Tawk.to | `tawk.to/{hash}/{id}` |
| Crisp | `crisp_website_id` |
| Hotjar | `hjid:XXXXX` |
| **AsiaSigma UUID** | Operator UUID dari CDN URL |
| **GigaGaming Brand** | Brand code dari CDN URL |

### Meta Verification Tokens

Ekstraksi token verifikasi domain dari `<meta>` tag:
- Google Site Verification
- Facebook Domain Verification
- Microsoft Validate
- Yandex Verification
- Apple Domain Verification

Token disimpan bersama DNS TXT records — memungkinkan cross-referencing domain milik satu operator yang sama.

### SimHash — Template Clustering

64-bit fuzzy fingerprint dari struktur DOM (tag+class, tag+id, frekuensi tag). Dua situs dengan Hamming distance ≤ 8 dianggap menggunakan template yang sama.

Digunakan untuk:
- Identifikasi situs baru yang dibuat dari template judol yang sudah dikenal
- Boost confidence score (+0.35) jika template mirip situs judol terdaftar

### Infra Scan (DNS, SSL, NS, TXT)

Dijalankan untuk **semua** situs (bukan hanya judol):

| Data | Kegunaan |
|---|---|
| IP addresses | Cluster situs berdasarkan subnet /24 |
| Nameservers | Identifikasi registrar/hosting yang sering dipakai judol |
| SSL thumbprint | Situs berbagi SSL cert = satu operator |
| DNS TXT records | Token verifikasi domain |

### WHOIS Domain Intelligence

- Umur domain (domain baru = risiko lebih tinggi)
- Privacy protection status
- Registrar reputation
- Country code TLD risk scoring

### Discovery Pipeline

| Metode | Endpoint | Keterangan |
|---|---|---|
| Google Search | `/discovery/google` | Cari via Serper API |
| Certificate Transparency | `/discovery/certs` | Query crt.sh untuk subdomain baru |
| Sweep | `/discovery/sweep` | Multi-keyword Google + dedup + opsional auto-analyze |
| Deep Sweep | `/discovery/deep-sweep` | Recursive multi-source — lihat diagram di bawah |

#### Deep Sweep — Flow Lengkap

Deep Sweep adalah pipeline discovery paling komprehensif. Setelah Depth 0 (Google), **5 sumber enrichment berjalan paralel** untuk menemukan domain terkait dari berbagai arah:

```
Input: keywords []string
  │
  ├─ Depth 0: Google Search (Serper) → 50–200 domain
  │     │
  │     └─ Bulk Analyze semua domain baru
  │           │
  │           ├─ 🔐 CT Enrichment (crt.sh, paralel)
  │           │     Brand name tiap situs judol → crt.sh("%brand%")
  │           │     → domain baru yang belum di-Google
  │           │     → badak500.website → badak500.net, badak500.xyz, m.badak500.top
  │           │
  │           ├─ 🏷️  Tracking ID Cross-reference (DB, paralel)
  │           │     AsiaSigma UUID / GigaGaming brand / GTM / GA / FB Pixel
  │           │     → situs lain di DB yang pakai ID yang sama
  │           │     → AsiaSigma UUID unik per operator = 100% satu operator
  │           │
  │           ├─ 🌐 NS Cluster Follow (DB, paralel)
  │           │     Nameserver yang sama = satu hosting panel
  │           │     → situs lain di DB yang pakai NS yang sama
  │           │
  │           └─ 🔗 Alternate Link Follow (HTML, paralel)
  │                 <link rel="alternate/canonical"> di dalam HTML
  │                 → domain yang dideklarasikan eksplisit oleh operator sendiri
  │
  └─ Depth 1: Gabungan semua sumber di atas → Bulk Analyze
        │
        └─ External links + iframes dari situs judol Depth 1
              → Depth 2 → ... → max_depth
```

**Mengapa ini efektif:**
- Operator judol memisahkan domain agar tidak mudah diblokir sekaligus, tapi mereka sering lalai menggunakan tracking ID yang sama di semua domain
- AsiaSigma UUID adalah pengenal unik per operator — satu UUID = satu operator, berapapun domain yang mereka punya
- Alternate link adalah "pengakuan sendiri" dari operator — mereka menulis domain lain-nya di dalam HTML-nya sendiri
- CT logs mencatat setiap SSL cert dalam hitungan jam — domain baru operator muncul di CT jauh sebelum terindeks Google

### Enrichment Eksternal

| Service | Data yang Didapat | Kegunaan |
|---|---|---|
| **URLScan.io** | Screenshot, verdict, score, technology stack (Wappa), categories | Bukti visual, identifikasi platform builder |
| **VirusTotal** | Malicious/suspicious count dari 70+ engine, domain categories, permalink | Validasi reputasi, boost confidence scoring |
| **LeakOSINT** | Data KomInfo (NIK, provider, tanggal registrasi) + Dukcapil (nama, alamat) | Identifikasi pemilik nomor HP dari artefak |

### OCR — Analisis Gambar

Tesseract OCR via CLI — ekstrak teks dari gambar external dan analisis dengan keyword detection + artifact extraction. Dijalankan sebagai background job Asynq per gambar.

---

## Pipeline Analisis

Setiap URL yang dianalisis melewati 9 langkah:

```
URL Input
  │
  ├─ 1. Crawl (Chromedp headless, 5s wait, network intercept)
  │
  ├─ 2. WHOIS domain intelligence
  │
  ├─ 3. SimHash cluster check (query DB → boost jika mirip judol dikenal)
  │
  ├─ 4. Composite scoring (content 45% + network 30% + domain 25%)
  │
  ├─ 5. Persist → PostgreSQL (semua situs)
  │       ├─ artifacts, links, alternate links, apps, images
  │       ├─ tracking IDs, meta tokens
  │       └─ simhash update
  │
  ├─ 6. Persist → Neo4j (hanya judol — graph intelligence)
  │
  ├─ 7. Infra Scan (DNS, NS, SSL, TXT) → PostgreSQL + Neo4j
  │
  ├─ 8. Sub-crawl (max 10 external links → artifact enrichment)
  │
  └─ 9. Enqueue background enrichment (URLScan + VirusTotal)
```

---

## Skema Database

PostgreSQL (13 migrasi, 20+ tabel):

| Tabel | Isi |
|---|---|
| `sites` | Data utama situs (URL, is_gambling, priority, confidence, simhash, status) |
| `scan_logs` | Log JSONB hasil setiap scan |
| `keywords` | Keyword detection (berbobot) + keyword search discovery |
| `artifacts` | Artefak transaksi (bank, e-wallet, VA, crypto, QRIS, kontak) |
| `links` + `site_links` | Link internal/external/iframe/alternate per situs |
| `images` + `site_images` | URL gambar eksternal (dipisah dari links) |
| `apps` + `site_apps` | Link unduhan APK/IPA dan app store |
| `ips` + `site_ips` | IP address per situs + subnet clustering |
| `nameservers` + `site_nameservers` | Nameserver per situs |
| `ssl_certs` + `site_ssl` | SSL thumbprint per situs |
| `txt_tokens` + `site_txt_tokens` | DNS TXT records + meta verification tokens |
| `tracking_ids` + `site_tracking` | Tracking ID (GA, GTM, FB Pixel, platform UUID) |
| `domain_blacklist` | Domain CDN/sosmed yang difilter dari sweep |
| `leak_kominfo` | Data KomInfo dari LeakOSINT (NIK, provider, tanggal) |
| `leak_dukcapil` | Data Dukcapil dari LeakOSINT (nama, alamat, gender) |
| `urlscan_results` | Hasil URLScan (verdict, score, screenshot, technologies) |
| `virustotal_results` | Hasil VirusTotal (malicious/suspicious count, categories) |

---

## Setup

### Prasyarat

- Go 1.21+
- Docker & Docker Compose
- Google Chrome (untuk Chromedp headless)
- Tesseract OCR: `brew install tesseract` (untuk fitur OCR gambar)

### 1. Clone & konfigurasi

```bash
git clone <repo-url>
cd jutec-go
cp .env.example .env
# Edit .env sesuai konfigurasi lokal
```

### 2. Isi `.env`

```env
PORT=8080
GIN_MODE=debug

# Neo4j
NEO4J_URI=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=password123

# PostgreSQL
POSTGRES_URL=postgres://user:password@localhost:5432/db_jutec

# Redis
REDIS_URL=localhost:6379

# Google Search (Serper)
SERPER_API_KEY=your_serper_key
SERPER_PAGES=5

# LeakOSINT
LEAKOSINT_TOKEN=your_token

# URLScan.io (https://urlscan.io — free tier: 100 scan/hari)
URLSCAN_API_KEY=your_urlscan_key

# VirusTotal (https://virustotal.com — free tier: 4 req/menit)
VIRUSTOTAL_API_KEY=your_virustotal_key
```

> Semua API key bersifat opsional — server tetap berjalan tanpa keduanya. Enrichment di-skip secara graceful jika key tidak dikonfigurasi atau rate limit tercapai.

### 3. Jalankan infrastruktur

```bash
docker-compose up -d
```

Menjalankan: PostgreSQL (5432), Neo4j (7687 Bolt / 7474 Browser), Redis (6379).

### 4. Jalankan migrasi database

```bash
go run cmd/migrate/main.go up
```

### 5. Jalankan server

```bash
go run cmd/server/main.go
```

Server berjalan di `http://localhost:8080`. Background worker (OCR + enrichment) otomatis aktif jika Redis tersedia.

---

## API Reference

### Health

```
GET /health
```

---

### Analisis

#### Analisis Satu URL

```
GET /analyze?url={url}&depth={0|1}
```

| Parameter | Default | Keterangan |
|---|---|---|
| `url` | — | URL target (wajib) |
| `depth` | `0` | `1` = ikut sub-crawl external links |

```bash
curl "http://localhost:8080/analyze?url=https://target-site.com"
```

<details>
<summary>Contoh Response</summary>

```json
{
  "url": "https://target-site.com",
  "saved": true,
  "result": {
    "is_gambling": true,
    "confidence": 0.95,
    "priority": "High",
    "found_keywords": ["slot", "gacor", "maxwin", "deposit pulsa"],
    "found_patterns": ["livechat", "whatsapp"],
    "network_signals": {
      "game_providers": ["Pragmatic Play", "PG Soft"],
      "total_requests": 142,
      "suspicious_requests": 23
    },
    "fingerprint": {
      "html_hash": "a1b2c3d4...",
      "title": "Slot Gacor Maxwin 2025",
      "favicon_url": "/favicon.ico",
      "scripts_hash": "e5f6g7h8..."
    },
    "artifacts": [
      { "type": "Bank",        "value": "1234567890",   "name": "BCA" },
      { "type": "E-Wallet",    "value": "081234567890", "name": "DANA" },
      { "type": "VirtualAccount","value": "1234567890123","name": "VA" },
      { "type": "Contact",     "value": "6281234567890","name": "WhatsApp" }
    ],
    "links": {
      "internal":  ["https://target-site.com/daftar"],
      "external":  ["https://otherdomain.com"],
      "iframes":   ["https://game-provider.com/embed"],
      "alternate": ["https://target-site.net"],
      "apps":      ["https://storetn.in/BRAND/app.apk"],
      "total": 47
    },
    "tracking_ids": [
      { "platform": "GoogleTag",  "value": "G-XXXXXXXX" },
      { "platform": "AsiaSigma", "value": "uuid-xxxx-xxxx" }
    ],
    "meta_tokens": [
      { "service": "GoogleVerification", "token": "xxxxx" }
    ],
    "simhash": "a3f2c1b4e5d6f7a8",
    "redirect_chain": ["https://short.url/abc", "https://target-site.com"]
  },
  "domain_intel": {
    "domain_age_days": 12,
    "privacy_protected": true,
    "risk_score": 0.75
  },
  "score": {
    "final": 0.97,
    "is_gambling": true,
    "priority": "High",
    "breakdown": {
      "content":  0.80,
      "network":  1.00,
      "domain":   0.75,
      "url_path": 0.30,
      "tld_risk": 0.50,
      "simhash":  0.00
    }
  }
}
```
</details>

#### Analisis Bulk

```
POST /analyze/bulk
Content-Type: application/json

{"urls": ["https://site1.com", "https://site2.net"]}
```

Maksimum 5 URL diproses paralel, sisanya mengantri otomatis.

---

### Discovery

#### Cari via Google

```
GET /discovery/google?q={keyword}
```

#### Cari via Certificate Transparency

```
GET /discovery/certs?domain={domain}
```

Mencari subdomain dan domain terkait di crt.sh — berguna menemukan domain baru milik operator sebelum terindeks Google.

#### Sweep Multi-Keyword

```
POST /discovery/sweep
Content-Type: application/json

{
  "keywords": ["slot gacor deposit pulsa"],
  "use_defaults": true,
  "auto_analyze": true
}
```

| Field | Keterangan |
|---|---|
| `use_defaults` | Gunakan ~20 keyword judol bawaan sistem |
| `auto_analyze` | Langsung analisis semua domain baru yang ditemukan |

#### Deep Sweep

```
POST /discovery/deep-sweep
Content-Type: application/json
```

Pipeline discovery rekursif multi-sumber. Setelah Depth 0 (Google), menemukan domain terkait via CT logs, shared tracking ID, shared nameserver, dan alternate links — semuanya paralel.

| Field | Default | Keterangan |
|---|---|---|
| `keywords` | `[]` | Keyword tambahan selain default |
| `use_defaults` | `false` | Gunakan ~20 keyword judol bawaan DB |
| `max_depth` | `10` | Kedalaman rekursi maksimal |
| `max_urls` | `5000` | Total URL maksimal yang diproses |

```bash
curl -X POST http://localhost:8080/discovery/deep-sweep \
  -H "Content-Type: application/json" \
  -d '{"use_defaults": true, "max_depth": 3, "max_urls": 500}'
```

<details>
<summary>Contoh Response</summary>

```json
{
  "keywords": ["slot gacor hari ini", "..."],
  "depth_reached": 2,
  "total_found": 312,
  "total_analyzed": 298,
  "total_gambling": 241,
  "total_skipped": 14,
  "time_taken": "8m43s",
  "per_depth": [
    {
      "depth": 0,
      "urls_found": 47,
      "new_urls": 44,
      "analyzed": 44,
      "gambling_found": 38,
      "domains": ["https://..."]
    },
    {
      "depth": 1,
      "urls_found": 267,
      "new_urls": 254,
      "analyzed": 254,
      "gambling_found": 203,
      "domains": ["https://..."]
    }
  ]
}
```
</details>

> **Sumber Depth 1 pool** — setelah Depth 0 selesai, pool untuk Depth 1 diisi dari 5 sumber paralel:
> `external links` + `CT (crt.sh)` + `shared tracking ID` + `shared NS` + `alternate links`

---

### Intelligence

#### Reverse Lookup

Temukan semua situs yang berbagi satu atribut:

```
GET /intelligence/reverse-lookup?type={type}&value={value}
```

| `type` | Keterangan |
|---|---|
| `IP` | Situs dengan IP yang sama |
| `NS` | Situs dengan nameserver yang sama |
| `SSL` | Situs dengan SSL thumbprint yang sama |
| `Fingerprint` | Situs dengan HTML hash yang sama (template identik) |
| `Bank` / `E-Wallet` / `QRIS` / `Crypto` | Situs dengan artefak pembayaran yang sama |
| `Contact` | Situs dengan nomor WA/Telegram/Line yang sama |

```bash
# Cari semua situs yang pakai rekening BCA yang sama
curl "http://localhost:8080/intelligence/reverse-lookup?type=Bank&value=1234567890"

# Cari semua situs di-hosting di IP yang sama
curl "http://localhost:8080/intelligence/reverse-lookup?type=IP&value=103.45.67.89"
```

---

### Network Intelligence

```
GET /network/tracking/lookup?platform={platform}&value={value}
GET /network/tracking/top
GET /network/ip/lookup?ip={ip}
GET /network/ip/subnet?ip={ip}
GET /network/ip/top
GET /network/template/similar?simhash={hash}&threshold={0-64}
GET /network/ns/lookup?ns={nameserver}
GET /network/ns/top
GET /network/ssl/lookup?thumb={thumbprint}
GET /network/ssl/top
GET /network/txt/lookup?token={token}
GET /network/txt/top
```

---

### Artifacts

```
GET /artifacts/lookup?type={type}&value={value}
GET /artifacts/top
```

---

### Keywords

```
GET    /keywords/detection          # List keyword deteksi
POST   /keywords/detection          # Tambah keyword
PUT    /keywords/detection/:id      # Update
DELETE /keywords/detection/:id      # Hapus

GET    /keywords/search             # List keyword discovery
POST   /keywords/search
PUT    /keywords/search/:id
DELETE /keywords/search/:id
```

Keyword menggunakan sistem bobot — `weight` antara `0.1` (lemah) hingga `1.0` (pasti judol):

```json
{ "keyword": "slot gacor maxwin", "weight": 0.8, "type": "detection" }
```

---

### Blacklist

Domain yang masuk blacklist dilewati saat DeepSweep:

```
GET    /blacklist
POST   /blacklist
PUT    /blacklist/:id
DELETE /blacklist/:id
```

---

### LeakOSINT

```
GET  /leak/phone?q={phone}    # Cek nomor HP (auto-normalisasi 08→628)
GET  /leak/nik?q={nik}        # Cari data berdasarkan NIK
POST /leak/enrich             # Enrich semua artifact HP yang belum dicek
```

---

## Enrichment: URLScan & VirusTotal

Enrichment dijalankan otomatis sebagai background job setelah setiap scan. Tidak memblokir crawl utama.

### Rate Limit Handling

Semua kondisi API ditangani secara graceful — engine tidak crash atau retry agresif:

| Kondisi | Perilaku |
|---|---|
| API key tidak dikonfigurasi | Skip enrichment, tidak error |
| HTTP 429 (rate limit) | Log warning, skip — tidak retry segera |
| HTTP 404 VT (URL belum ada di database VT) | Submit URL untuk analisis, lanjut |
| URLScan scan belum siap | Poll 3× @ 15 detik, simpan UUID jika masih pending |
| Error network | Log, skip — crawl utama tidak terganggu |

### URLScan Strategy

1. Search data existing untuk domain (tidak pakai kuota scan)
2. Jika scan ada dan ≤ 7 hari → pakai data existing
3. Jika tidak ada → submit scan baru (`visibility: unlisted`)
4. Poll hasil maksimal 3× dengan interval 15 detik

### Kegunaan Hasil URLScan

| Field | Kegunaan |
|---|---|
| `screenshot_url` | Bukti visual untuk pelaporan ke Kominfo (capture tampilan situs) |
| `technologies` | Identifikasi platform builder via Wappa — AsiaSigma/GigaGaming stack |
| `verdict` + `score` | Cross-check dengan scoring lokal; `malicious` memperkuat label |
| `categories` | Label dari URLScan community |
| `scan_uuid` | Link ke `https://urlscan.io/result/{uuid}/` untuk laporan lengkap |

### Kegunaan Hasil VirusTotal

| Field | Kegunaan |
|---|---|
| `malicious_count` | Jumlah engine yang flag domain berbahaya dari 70+ engine |
| `suspicious_count` | Flag mencurigakan — kombinasi tinggi = gambling confirmed |
| `categories` | `gambling`, `phishing`, `malware` langsung memperkuat label |
| `permalink` | Link ke halaman VT untuk laporan resmi / bukti |

### Query Data Enrichment

```sql
-- Situs dengan VT malicious count tertinggi
SELECT s.url, v.malicious_count, v.total_engines, v.categories
FROM virustotal_results v
JOIN sites s ON s.id = v.site_id
WHERE v.resource_type = 'domain'
ORDER BY v.malicious_count DESC;

-- Screenshot semua situs High priority
SELECT s.url, u.screenshot_url, u.verdict, u.scanned_at
FROM urlscan_results u
JOIN sites s ON s.id = u.site_id
WHERE s.priority = 'High'
ORDER BY u.scanned_at DESC;

-- Domain yang di-flag gambling oleh VirusTotal
SELECT s.url, v.categories
FROM virustotal_results v
JOIN sites s ON s.id = v.site_id
WHERE 'gambling' = ANY(v.categories);
```

---

## Background Jobs

| Task | Queue | Trigger | Keterangan |
|---|---|---|---|
| `scan:url` | `default` | API / sweep | Scan URL secara async |
| `ocr:image` | `ocr` | Setelah crawl (per gambar) | Ekstrak teks dari gambar external |
| `enrich:site` | `enrichment` | Setelah setiap scan | URLScan + VirusTotal lookup |

Worker berjalan otomatis bersama server jika Redis tersedia. Semua task idempotent dan aman untuk di-retry.

---

## Visualisasi Jaringan (Neo4j)

Buka Neo4j Browser di `http://localhost:7474`:

```cypher
-- Situs judol High priority
MATCH (s:Site {priority: "High"}) RETURN s LIMIT 50

-- Kluster: situs yang berbagi nomor rekening
MATCH (s:Site)-[:USES_PAYMENT]->(a:Artifact {type: "Bank"})
WITH a, collect(s) AS sites WHERE size(sites) > 1
RETURN a, sites

-- Kluster: situs di-hosting di IP yang sama
MATCH (s:Site)-[:RESOLVES_TO]->(i:IP)
WITH i, collect(s) AS sites WHERE size(sites) > 1
RETURN i, sites

-- Rantai domain terhubung via external link
MATCH path = (a:Site)-[:LINKS_TO*1..3]->(b:Site)
WHERE a.is_gambling = true
RETURN path LIMIT 30
```

---

## Alur Kerja Rekomendasi

### Operasi harian

```bash
# Sweep semua keyword default + analisis otomatis
curl -X POST http://localhost:8080/discovery/sweep \
  -H "Content-Type: application/json" \
  -d '{"use_defaults": true, "auto_analyze": true}'
```

### Investigasi jaringan operator

```bash
# 1. Analisis situs target
curl "http://localhost:8080/analyze?url=https://target-site.com"

# 2. Cari situs lain yang pakai rekening yang sama
curl "http://localhost:8080/intelligence/reverse-lookup?type=Bank&value=1234567890"

# 3. Cari situs dengan template serupa (SimHash cluster)
curl "http://localhost:8080/network/template/similar?simhash=a3f2c1b4e5d6f7a8&threshold=8"

# 4. Enrich nomor HP dari artefak via LeakOSINT
curl -X POST http://localhost:8080/leak/enrich

# 5. Cari subdomain baru via Certificate Transparency
curl "http://localhost:8080/discovery/certs?domain=target-site.com"
```

---

## Struktur Project

```
jutec-go/
├── cmd/
│   ├── server/main.go          # Entry point server + worker
│   └── migrate/main.go         # Database migration runner
├── internal/
│   ├── api/
│   │   ├── handlers/           # HTTP handler per domain
│   │   └── router/router.go    # Route registration
│   ├── config/config.go        # Environment config
│   ├── crawler/
│   │   ├── crawler.go          # Headless Chrome + network intercept
│   │   ├── analyzer.go         # Scoring logic + AnalysisResult
│   │   ├── extractor.go        # Artifact extraction (bank, ewallet, crypto, dll)
│   │   ├── links.go            # Link extraction + app URL detection + shortener resolve
│   │   ├── tracking.go         # Tracking ID + meta token extraction
│   │   ├── fingerprint.go      # HTML/script fingerprinting
│   │   ├── simhash.go          # 64-bit DOM SimHash + Hamming distance
│   │   └── network.go          # Game provider detection via network requests
│   ├── intelligence/
│   │   └── scorer.go           # Composite score engine (content+network+domain)
│   ├── discovery/
│   │   ├── google.go           # Serper API client
│   │   └── certs.go            # crt.sh Certificate Transparency
│   ├── infra/
│   │   └── scanner.go          # DNS, SSL, TXT lookup
│   ├── whois/                  # WHOIS domain intelligence
│   ├── leakosint/client.go     # LeakOSINT API client
│   ├── urlscan/client.go       # URLScan.io API client
│   ├── virustotal/client.go    # VirusTotal API client
│   ├── service/
│   │   ├── scan.go             # Scan service (9-step pipeline)
│   │   ├── ocr.go              # OCR service
│   │   └── discovery.go        # Discovery + sweep pipeline
│   ├── store/
│   │   ├── postgres.go         # Connection + base methods
│   │   ├── neo4j.go            # Neo4j graph operations
│   │   ├── artifacts.go        # Artifact store
│   │   ├── links.go            # Links store
│   │   ├── images.go           # Images store
│   │   ├── apps.go             # APK/App links store
│   │   ├── ips.go              # IP + SimHash cluster store
│   │   ├── infra_store.go      # NS, SSL store
│   │   ├── tracking.go         # Tracking ID store
│   │   ├── txt.go              # TXT tokens + meta tokens store
│   │   ├── keywords.go         # Keywords store
│   │   ├── leakosint.go        # LeakOSINT results store
│   │   ├── enrichment.go       # URLScan + VT results store
│   │   └── blacklist.go        # Domain blacklist store
│   └── worker/
│       ├── worker.go           # Asynq server setup
│       └── tasks/
│           ├── scan_task.go    # Async URL scan task
│           ├── ocr_task.go     # OCR image task
│           └── enrich_task.go  # URLScan + VT enrichment task
└── migrations/
    ├── 000001_init_schema.*    # Sites + scan_logs
    ├── 000002_keywords.*       # Detection + search keywords
    ├── 000003_artifacts.*      # Transaction artifacts
    ├── 000004_links.*          # Links + site_links
    ├── 000005_network_intel.*  # IPs + tracking IDs
    ├── 000006_txt_records.*    # DNS TXT + meta tokens
    ├── 000007_ns_ssl.*         # Nameservers + SSL certs
    ├── 000008_images.*         # External images
    ├── 000009_domain_blacklist.*
    ├── 000010_leak_intel.*     # LeakOSINT tables
    ├── 000011_site_status.*    # Status column
    ├── 000012_apps.*           # APK/App links
    └── 000013_enrichment.*     # URLScan + VT results
```

---

## Variabel Lingkungan

| Variabel | Default | Wajib | Keterangan |
|---|---|---|---|
| `PORT` | `8080` | — | Port HTTP server |
| `GIN_MODE` | `debug` | — | `debug` atau `release` |
| `NEO4J_URI` | — | — | Bolt URI Neo4j |
| `NEO4J_USER` | — | — | Username Neo4j |
| `NEO4J_PASSWORD` | — | — | Password Neo4j |
| `POSTGRES_URL` | — | ✓ | DSN PostgreSQL |
| `REDIS_URL` | `localhost:6379` | — | Address Redis |
| `SERPER_API_KEY` | — | — | API key [serper.dev](https://serper.dev) |
| `SERPER_PAGES` | `5` | — | Jumlah halaman Google per keyword |
| `LEAKOSINT_TOKEN` | — | — | Token LeakOSINT API |
| `URLSCAN_API_KEY` | — | — | API key [urlscan.io](https://urlscan.io) — free: 100 scan/hari |
| `VIRUSTOTAL_API_KEY` | — | — | API key [virustotal.com](https://virustotal.com) — free: 4 req/menit |

> Semua komponen eksternal (Neo4j, Redis, Serper, LeakOSINT, URLScan, VirusTotal) bersifat opsional. Server tetap berjalan dan fitur menyesuaikan konfigurasi yang tersedia.
