#!/usr/bin/env bash
# Jalankan Deep Sweep dari terminal dan ikuti log-nya langsung.
#
#   ./scripts/deepsweep.sh [level] [maks_url]     contoh: ./scripts/deepsweep.sh 2 500
#   ./scripts/deepsweep.sh follow                 ikuti lagi sweep yang sedang berjalan
#
# Sweep berjalan di server: Ctrl+C hanya berhenti MENGIKUTI log, sweep tetap jalan.
# Jeda / lanjut / batal:  ./scripts/deepsweep.sh pause|resume|cancel
set -euo pipefail

API="${JUTEC_API:-http://localhost:8080}"
USER_NAME="${JUTEC_USER:-$(whoami)}"

json() { python3 -c "import json,sys; d=json.load(sys.stdin); print($1)"; }

active_id() { curl -fsS "$API/sweep/active" | json "(d.get('job') or {}).get('id','')"; }

case "${1:-}" in
  pause|resume|cancel)
    id=$(active_id); [ -z "$id" ] && { echo "Tidak ada sweep yang berjalan."; exit 1; }
    curl -fsS -X POST "$API/sweep/$id/$1" -H "X-Jutec-User: $USER_NAME"; echo; exit 0 ;;
  follow)
    id=$(active_id); [ -z "$id" ] && { echo "Tidak ada sweep yang berjalan."; exit 1; } ;;
  *)
    depth="${1:-2}"; max="${2:-500}"
    curl -fsS -m 3 "$API/health" >/dev/null || { echo "Backend tidak merespons di $API — nyalakan dulu (./run.sh be)."; exit 1; }
    resp=$(curl -sS -X POST "$API/sweep/start" -H 'Content-Type: application/json' -H "X-Jutec-User: $USER_NAME" \
      -d "{\"use_defaults\":true,\"max_depth\":$depth,\"max_urls\":$max}")
    id=$(echo "$resp" | json "d.get('job_id','')")
    [ -z "$id" ] && { echo "Gagal memulai: $resp"; exit 1; }
    echo "▶ Deep Sweep $id dimulai (level $depth, maks $max URL)" ;;
esac

# Warna per jenis log
R=$'\e[0m'; B=$'\e[1;34m'; G=$'\e[32m'; Y=$'\e[33m'; D=$'\e[2m'
since=0
echo "${D}Mengikuti log $id — Ctrl+C untuk berhenti mengikuti (sweep tetap jalan)${R}"
while true; do
  out=$(curl -fsS "$API/sweep/$id?since=$since" 2>/dev/null) || { echo "${Y}Backend tidak merespons, mencoba lagi…${R}"; sleep 5; continue; }
  # Cetak log baru, lalu baris terakhir: "<seq> <status> <processed>/<total> <fase>"
  echo "$out" | python3 -c "
import json,sys
d=json.load(sys.stdin); c={'phase':'$B','found':'$G','warn':'$Y','skip':'$D','info':''}
for e in d.get('events',[]):
    print(e['at'][11:19], c.get(e['kind'],'')+e['text']+'$R')
j=d['job']; live=d.get('live') or {}
seq=d['events'][-1]['seq'] if d.get('events') else $since
print('__STATE__', seq, j['status'], j['processed'], j['total'], live.get('phase',''))
" > /tmp/jutec-sweep-$$.txt
  grep -v '^__STATE__' /tmp/jutec-sweep-$$.txt || true
  read -r _ since status processed total phase < <(grep '^__STATE__' /tmp/jutec-sweep-$$.txt)
  rm -f /tmp/jutec-sweep-$$.txt
  case "$status" in
    running|paused) ;;
    *) echo "${B}■ Sweep $status · $processed/$total situs tahap terakhir${R}"; break ;;
  esac
  sleep 2
done
