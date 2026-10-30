#!/usr/bin/env bash
# Jutec runner — backend (Go :8080) + frontend (Nuxt :3000)
# Usage: ./run.sh [all|be|fe|infra|migrate|stop]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
BE="$ROOT/jutec-go"
FE="$ROOT/jutec-fe"

infra() {
  echo "▶ Checking local services..."
  if redis-cli ping >/dev/null 2>&1; then
    echo "✓ Redis sudah jalan"
  else
    echo "▶ Starting Redis (brew services)..."
    brew services start redis >/dev/null
  fi
  if command -v neo4j >/dev/null; then
    neo4j status >/dev/null 2>&1 || neo4j start
  else
    echo "ℹ Neo4j tidak terpasang — dilewati (opsional, graph intelligence nonaktif)"
  fi
  if command -v pg_isready >/dev/null && ! pg_isready -h localhost -p 5432 -q; then
    echo "⚠ PostgreSQL lokal (localhost:5432) belum jalan. Contoh: brew services start postgresql"
  fi
}

migrate() {
  echo "▶ Running migrations..."
  (cd "$BE" && go run cmd/migrate/main.go up)
}

port_busy() { lsof -ti :"$1" -sTCP:LISTEN >/dev/null 2>&1; }

# Matikan proses yang listen di port. `go run` menjalankan binary terpisah ("…/exe/main"),
# jadi pkill berdasarkan nama file sumber tidak cukup — hentikan lewat port.
free_port() {
  local pids
  pids=$(lsof -ti :"$1" -sTCP:LISTEN 2>/dev/null || true)
  [ -n "$pids" ] && kill $pids 2>/dev/null && echo "✓ Port $1 dibebaskan" || true
}

be() {
  if port_busy 8080; then
    echo "⚠ Port 8080 sudah dipakai. Jalankan './run.sh stop' dulu, atau 'lsof -i :8080' untuk melihat prosesnya."
    return 1
  fi
  echo "▶ Backend → http://localhost:8080"
  (cd "$BE" && go run cmd/server/main.go)
}

fe() {
  if port_busy 3000; then
    echo "⚠ Port 3000 sudah dipakai. Jalankan './run.sh stop' dulu."
    return 1
  fi
  [ -d "$FE/node_modules" ] || (cd "$FE" && npm install)
  echo "▶ Frontend → http://localhost:3000"
  (cd "$FE" && npm run dev)
}

stop() {
  echo "▶ Stopping backend (:8080) & frontend (:3000)..."
  free_port 8080
  free_port 3000
  echo "▶ Stopping Redis (+ Neo4j kalau ada)..."
  brew services stop redis >/dev/null || true
  command -v neo4j >/dev/null && neo4j stop || true
}

all() {
  infra
  trap 'kill 0' INT TERM EXIT
  be &
  fe &
  wait
}

case "${1:-all}" in
  all|be|fe|infra|migrate|stop) "${1:-all}" ;;
  *) echo "Usage: $0 [all|be|fe|infra|migrate|stop]"; exit 1 ;;
esac
