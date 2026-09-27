# Incident Management

Aplikasi terpusat untuk mencatat, mengelola, dan menyelesaikan incident operasional.
Workflow MVP: Create → Assign → Investigate → Fix → Verify → Close.

Acuan produk: `PRD_Incident_Management_MVP(1).pdf`. Aturan kerja agent: `AGENTS.md`.

## Arsitektur

- `backend/` — Go API (`chi` + `pgx`), port `8080`
- `frontend/` — React 18 + TypeScript + Vite + Tailwind, port `5173`
- `docker-compose.yml` — Postgres 15, port `5432`

## Prasyarat

- Docker Desktop (berjalan)
- Node.js 20+ & npm
- Go 1.22+ — **hanya untuk compile/lint**, contoh: `winget install -e --id GoLang.Go`
  > Catatan: di mesin dengan kebijakan Application Control, binary Go hasil
  > kompilasi lokal tidak bisa dieksekusi. Karena itu backend dijalankan
  > lewat image Docker `golang` (sudah terverifikasi). Lihat Opsi B di bawah.

## 1. Database

```powershell
cd D:\My-dev\GITHUB\incident-management
docker compose up -d db
```

Cek sehat:

```powershell
docker inspect -f "{{.State.Health.Status}}" incident-db
# healthy
```

## 2. Migrasi database

Via Docker (terverifikasi):

```powershell
cd D:\My-dev\GITHUB\incident-management
docker run --rm -v "${PWD}:/app" -w /app/backend -v gomodcache:/go/pkg/mod `
  -e DATABASE_URL=postgres://incidents:incidents_dev_only@host.docker.internal:5432/incidents?sslmode=disable `
  golang:1.27-alpine sh -c "go tool goose -dir ./migrations postgres `$DATABASE_URL up"
```

## 3. Backend

### Opsi A — native (mesin tanpa blokir App Control)

```powershell
cd D:\My-dev\GITHUB\incident-management\backend
$env:DATABASE_URL = "postgres://incidents:incidents_dev_only@localhost:5432/incidents?sslmode=disable"
$env:PORT = "8080"
$env:JWT_SECRET = "dev-only-secret"
go run ./cmd/api
```

### Opsi B — via Docker (disarankan di mesin ini)

```powershell
cd D:\My-dev\GITHUB\incident-management
docker run -d --name incident-api -p 8080:8080 -v "${PWD}:/app" -w /app/backend -v gomodcache:/go/pkg/mod `
  -e DATABASE_URL=postgres://incidents:incidents_dev_only@host.docker.internal:5432/incidents?sslmode=disable `
  -e PORT=8080 `
  -e JWT_SECRET=dev-only-secret `
  golang:1.27-alpine sh -c "go run ./cmd/api"
```

Perintah bantuan:

```powershell
curl.exe http://localhost:8080/health
docker logs -f incident-api
docker stop incident-api
docker start incident-api
docker restart incident-api   # wajib setelah ubah kode Go (kompilasi ulang ±1 menit)
```

Variabel env backend (`backend/.env.example` sebagai contoh):

| Var | Contoh |
| --- | ------ |
| `DATABASE_URL` | `postgres://incidents:incidents_dev_only@localhost:5432/incidents?sslmode=disable` |
| `PORT` | `8080` |
| `JWT_SECRET` | `dev-only-secret` |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173` (default) |
| `SMTP_HOST/PORT/USER/PASS/FROM` | kosong = email notifikasi tercatat `skipped` |
| `UPLOAD_DIR` | `./uploads` (default); maks file 10 MiB, tipe png/jpg/gif/webp/pdf/txt/zip |

## 4. Frontend

```powershell
cd D:\My-dev\GITHUB\incident-management\frontend
npm install   # sekali saja
npm run dev
```

Buka `http://localhost:5173` (hard refresh `Ctrl+Shift+R` bila CSS basi).

Variabel env frontend (opsional, `frontend/.env`):

```
VITE_API_URL=http://localhost:8080
```

## 5. Login (seed DEV ONLY)

| Email | Role | Password |
| ----- | ---- | -------- |
| `helpdesk@example.com` | HelpDesk | `Password123!` |
| `analyst@example.com` | SystemAnalyst | `Password123!` |
| `developer@example.com` | Developer | `Password123!` |
| `qa@example.com` | QA | `Password123!` |
| `manager@example.com` | ManagerLead | `Password123!` |
| `user@example.com` | User | `Password123!` |

> Ganti/hapus di environment bersama. Jangan commit kredensial asli.

## Perintah verifikasi

```powershell
# backend (dari folder backend/)
go build ./...
go vet ./...
gofmt -l .
```

`go test` dieksekusi via Docker (lihat Opsi B, ganti `go run ./cmd/api`
dengan `go test ./...` + `-e TEST_DATABASE_URL=...`).

```powershell
# frontend (dari folder frontend/)
npm run typecheck
npm run build
```

## Troubleshooting

| Gejala | Solusi |
| ------ | ------ |
| `go: command not found` (terminal lama) | `$env:Path = "C:\Program Files\Go\bin;" + $env:Path`, atau tutup semua terminal lalu buka lagi |
| `Application Control policy has blocked` saat `go run`/`go test` | Pakai Opsi B (Docker) |
| `Bind for 0.0.0.0:8080 failed` | Port dipakai proses lama: matikan container lama (`docker stop incident-api`) atau cari PID via `Get-NetTCPConnection -LocalPort 8080` |
| `Port 5173 is in use` | Proses `vite` lama masih hidup: `Stop-Process` PID-nya, lalu `npm run dev` lagi hingga dapat 5173 |
| Halaman tanpa style (link biru polos) | Pastikan `curl.exe -s http://localhost:5173/src/index.css` tidak mengandung `@tailwind`, lalu `Ctrl+Shift+R` |
| Login gagal dari browser padahal curl OK | Cek CORS: `CORS_ALLOWED_ORIGINS` harus memuat origin frontend |
