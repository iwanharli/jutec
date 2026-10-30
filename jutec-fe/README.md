# Jutec FE — Dashboard

Frontend dashboard untuk [Jutec Go](../jutec-go) — Judol Intelligence Engine.
Dibangun dengan Nuxt 3, Tailwind CSS, dan WebSocket real-time.

---

## Prasyarat

- Node.js 18+
- Backend [jutec-go](../jutec-go) berjalan di port `8080`

## Setup

```bash
npm install
npm run dev       # http://localhost:3000
npm run build     # production build
npm run preview   # preview production build
```

## Konfigurasi

Buat file `.env` (sudah disediakan defaultnya):

```env
NUXT_PUBLIC_API_BASE=http://localhost:8080
NUXT_PUBLIC_WS_BASE=ws://localhost:8080
```

---

## Halaman

| Halaman | URL | Keterangan |
|---|---|---|
| Dashboard | `/` | Overview stats + quick analyze |
| Analisis URL | `/analyze` | Scan real-time via WebSocket (SSE fallback) |
| Discovery | `/discovery` | Deep sweep multi-keyword + Google search |
| Intelligence | `/intelligence` | Reverse lookup, NS/IP/artifact cluster |
| Data Browser | `/data` | Tabel situs, artefak, apps |
| Keywords | `/keywords` | CRUD keyword detection & discovery |
| Blacklist | `/blacklist` | CRUD domain blacklist |
| Leak OSINT | `/leak` | Cek nomor HP / NIK via KomInfo & Dukcapil |

---

## Real-time Output

Halaman `/analyze` menggunakan **WebSocket** untuk streaming progress scan langkah per langkah.
Jika WebSocket tidak tersedia, otomatis fallback ke **Server-Sent Events (SSE)**.

```
Browser → WebSocket ws://localhost:8080/ws/analyze?url=...
              ↓ jika gagal
Browser → SSE      http://localhost:8080/sse/analyze?url=...
```

Format pesan dari backend:

```json
{ "type": "step",  "step": "crawling", "message": "Membuka headless browser...", "ts": 1234567890 }
{ "type": "step",  "step": "scored",   "message": "Score: 0.97 | Priority: High", "ts": ... }
{ "type": "done",  "step": "complete", "message": "Analisis selesai", "data": { ... } }
{ "type": "error", "step": "failed",   "message": "timeout", "ts": ... }
```

---

## Struktur

```
jutec-fe/
├── pages/
│   ├── index.vue          # Dashboard
│   ├── analyze.vue        # Analisis URL real-time
│   ├── discovery.vue      # Discovery & sweep
│   ├── intelligence.vue   # Network intelligence
│   ├── data.vue           # Data browser
│   ├── keywords.vue       # Keyword management
│   ├── blacklist.vue      # Blacklist management
│   └── leak.vue           # LeakOSINT
├── composables/
│   ├── useAnalyzeStream.ts  # WebSocket + SSE composable
│   └── useApi.ts            # REST API helper (get/post/put/del)
├── components/
│   ├── LiveLog.vue          # Real-time log viewer (auto-scroll)
│   ├── ScoreBar.vue         # Visualisasi breakdown skor
│   ├── PriorityBadge.vue    # Badge High/Medium/Low
│   └── StatCard.vue         # Kartu statistik dashboard
├── layouts/
│   └── default.vue          # Sidebar navigasi
├── assets/css/main.css      # Tailwind + custom classes
└── nuxt.config.ts
```
