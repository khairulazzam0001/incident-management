# AGENTS.md — incident-management

Panduan untuk AI agent (OpenCode / Muse Spark) yang bekerja di repo ini.
Stack: **Go API (backend) + React (frontend)**. Bahasa komunikasi: Indonesia.

## 1. Gambaran Proyek

Sumber kebenaran produk: `PRD_Incident_Management_MVP(1).pdf` di root repo.
Jika AGENTS.md dan PRD bertentangan, **PRD yang menang** — dan selaraskan AGENTS.md.

Sistem Incident Management: pencatatan, triase, eskalasi, dan penyelesaian insiden operasional.
Workflow MVP: Create → Assign → Investigate → Fix → Verify → Close.

Entitas inti (domain, sesuai PRD §6–§7, §11):
- `Incident`: id, incident_no unik human-readable (`INC-2026-000123`), title, description, source, severity (**S1–S4**, = dampak), priority (**P1–P4**, = urgensi, dipisah dari severity), status, application, environment, reporter, timestamps (created/updated/resolved/closed).
- Status lifecycle: `NEW → ASSIGNED → INVESTIGATING → FIXING → VERIFYING → RESOLVED → CLOSED`. Khusus: `VERIFYING → RESOLVED` (PASS) atau `→ FIXING` (FAIL + reason wajib). `CLOSED` terminal (hanya via reopen khusus).
- `Activity/Timeline`: audit trail tiap status change, assignment change, komentar, verification (actor + timestamp wajib).
- `Investigation`: notes & findings per incident. `Fix`: fix description + referensi deployment/PR. `Verification`: PASS/FAIL + reason.
- Roles: User/Customer, Help Desk/Support, System Analyst, Developer, QA, DevOps, Manager/Lead. Satu incident = satu owner/PIC aktif + satu team (collaborator menyusul).
- `Notification`: in-app + email (MVP); trigger: created, assigned/reassigned, status changed, new comment, verification failed, resolved, closed. Kirim async bila volume naik.

Prioritas: backend API benar & teruji dulu, frontend menyusul.

## 2. Struktur Repo (target)

```
/backend            # Go API
  /cmd/api          # main.go entrypoint
  /internal
    /handler        # HTTP handlers (tipis, validasi + panggil service)
    /service        # business logic
    /repository     # akses DB
    /model          # struct domain + DTO
    /middleware     # auth, logging, recovery
  /migrations       # SQL migration (golang-migrate / goose)
  go.mod
/frontend           # React app (Vite + TypeScript)
  /src
    /pages
    /components
    /api            # client fetch ke backend
    /hooks
  package.json
AGENTS.md
```

Jika struktur belum ada, ikuti struktur di atas saat membuat file baru.
Jangan membuat struktur alternatif tanpa konfirmasi user.

## 3. Tech & Versi

Backend:
- Go 1.22+, stdlib `net/http` + `chi` router (prefer chi; alternatif: stdlib mux saja).
- DB: PostgreSQL 15+. Driver: `pgx/v5` + `pgxpool`.
- Migrasi: `golang-migrate/migrate` atau `pressly/goose` — pilih satu, jangan campur.
- Config via env (`DATABASE_URL`, `PORT`, `JWT_SECRET`). Validasi env saat startup, fail fast.
- Auth: JWT Bearer (nanti). Untuk tahap awal boleh tanpa auth, tapi pisahkan middleware.

Frontend:
- React 18 + TypeScript + Vite. Styling: Tailwind CSS.
- Data fetching: `fetch` + custom hook, atau TanStack Query (prefer TanStack Query untuk list/detail incident).
- Jangan pakai UI framework berat (MUI/AntD) kecuali diminta user.

## 4. Perintah Umum

Backend:
```bash
cd backend
go mod tidy
go run ./cmd/api
go build ./...
go test ./...
go vet ./...
gofmt -l .
```

Frontend:
```bash
cd frontend
npm install
npm run dev
npm run build
npm run lint
npm run test --if-present
```

Jalankan perintah dari folder yang benar. Jangan `cd` di dalam command — pakai parameter `workdir`.

## 5. Konvensi Kode Go

- `gofmt` wajib bersih. `go vet` wajib lolos sebelum selesai.
- Handler tipis: parsing request, validasi, panggil service, tulis response JSON. Tanpa SQL di handler.
- Service berisi business logic + validasi transisi status. Repository hanya query DB.
- Response JSON konsisten. Error terstandarisasi (PRD §16): `{"code": "...", "message": "...", "details": ..., "request_id": "..."}`.
- Status code: `201` create, `200` ok, `400` validasi, `401` tanpa/invalid auth, `403` unauthorized transition/action, `404` not found, `409` konflik transisi status, `500` internal.
- Validasi transisi status di service (PRD §6, US-01–US-06):
  - `NEW → ASSIGNED` (via assign team + PIC)
  - `ASSIGNED → INVESTIGATING`
  - `INVESTIGATING → FIXING`
  - `FIXING → VERIFYING`
  - `VERIFYING → RESOLVED` (verification PASS) atau `VERIFYING → FIXING` (verification FAIL, reason wajib)
  - `RESOLVED → CLOSED` (closed_at + closed_by tercatat)
  - `CLOSED → (terminal; hanya via reopen khusus ber-permission, FR-12)`
  - Transisi invalid → tolak server-side, status tidak berubah; double submit tidak boleh duplikasi action.
- `incident_no` unik human-readable (`INC-2026-000123`), dibuat server-side saat create (cek race/uniqueness).
- ID pakai UUID v4 (`google/uuid`). Waktu pakai `timestamptz` / `time.Time` UTC.
- Jangan log secret (JWT secret, password, connection string).
- Error di-wrap dengan konteks (`fmt.Errorf("...: %w", err)`).

## 6. Konvensi Kode React/TypeScript

- TypeScript `strict`. Hindari `any` — pakai `unknown` + type guard atau tipe eksplisit.
- Komponen kecil, satu file satu komponen. Pisahkan `api/` (fetch) dari UI.
- State server via TanStack Query; state lokal via `useState`. Jangan duplikasi server state ke lokal.
- Pages (PRD §9): `Login`, `Dashboard`, `IncidentList`, `IncidentNew`, `IncidentDetail`, `MyIncidents`, `MasterData` (admin/authorized only). Komponen: `SeverityBadge`, `PriorityBadge`, `StatusBadge`, `IncidentForm`, `TimelineList`.
- Required states tiap page: loading/skeleton, empty (+ CTA Create bila berhak), success confirmation, error (jangan hilangkan input), retry (tanpa double-submit action).
- Form: controlled input + validasi sederhana sisi klien (title min 5 char, severity wajib, dst). Validasi utama tetap di backend.
- Base URL API dari env (`VITE_API_URL`), default `http://localhost:8080`.

## 7. API Contract (PRD §16 — jangan ubah tanpa alasan)

```
GET    /health
GET    /api/incidents?status=&severity=&priority=&q=&page=&limit=
POST   /api/incidents
GET    /api/incidents/:id
PATCH  /api/incidents/:id/status          # controlled status transition
PATCH  /api/incidents/:id/assignment      # assign/reassign team + PIC
POST   /api/incidents/:id/comments
GET    /api/incidents/:id/comments        # list komentar (tambahan Fase 1, di luar PRD §16)
GET    /api/incidents/:id/timeline        # audit trail (tambahan Fase 1, di luar PRD §16)
POST   /api/incidents/:id/investigations
POST   /api/incidents/:id/fixes
POST   /api/incidents/:id/verification    # PASS → RESOLVED, FAIL → FIXING
GET    /api/incidents/:id/verifications   # list verification (tambahan Fase 2)
POST   /api/incidents/:id/close
POST   /api/incidents/:id/reopen         # RESOLVED/CLOSED → INVESTIGATING, koordinator + reason (tambahan Fase 2, FR-12)
GET    /api/incidents/:id/investigations  # list (tambahan Fase 2, di luar PRD §16)
GET    /api/incidents/:id/fixes           # list (tambahan Fase 2, di luar PRD §16)
POST   /api/auth/login                    # {email,password} → {token,user} (tambahan Fase 1)
GET    /api/meta                          # master data untuk form/filter (tambahan Fase 1)
GET    /api/users                         # user aktif untuk assignment lookup (tambahan Fase 1)
```

Body create incident (field lengkap menyusul master data):
```json
{ "title": "Checkout gagal 500", "description": "...", "severity": "S1", "priority": "P1", "application_id": "...", "environment_id": "...", "source_id": "..." }
```

Jangan ubah contract tanpa update frontend `api/` dan sebutkan di ringkasan akhir.

## 8. Database & Migrasi (PRD §11)

- Konvensi nama: tabel master/reference pakai prefix `master_`, tabel transaksi/event pakai prefix `trans_`.
- Semua perubahan skema via file migrasi di `backend/migrations/`, format `0001_<nama>.up.sql` / `.down.sql`.
- Jangan edit migrasi yang sudah di-merge. Buat migrasi baru.
- Master: `master_user`, `master_team`, `master_application`, `master_environment`, `master_incident_source`, `master_incident_priority` (P1–P4), `master_incident_severity` (S1–S4), `master_incident_status` (NEW…CLOSED). Seed data master via migrasi.
- Transaksi: `trans_incident` (termasuk `incident_no` unik), `trans_incident_assignment` (riwayat team/PIC), `trans_incident_comment`, `trans_incident_activity` (audit trail), `trans_incident_investigation`, `trans_incident_fix`, `trans_incident_verification`, `trans_incident_attachment`.
- Index: `trans_incident(status_id)`, `trans_incident(severity_id)`, `trans_incident(priority_id)`, `trans_incident(incident_no)` unik, `trans_incident_activity(incident_id)`; hindari query N+1 di timeline.
- Semua status/assignment change wajib simpan actor + timestamp.

## 9. Testing

- Backend: unit test untuk transisi status service + handler test (httptest) untuk happy path & 404/400. Target: `go test ./...` hijau.
- Frontend: minimal smoke test render list page jika ada test runner. Jangan over-engineer test di awal.
- Setelah implementasi/fix: jalankan `go test ./...` + `go vet ./...` (backend) dan `npm run build` (frontend jika disentuh), laporkan hasilnya.

## 10. Git Workflow

- Jangan commit/push/PR kecuali diminta eksplisit.
- Sebelum commit (jika diminta): cek `git status`, `git diff`, stage hanya file yang dimaksud, jangan commit secret/`.env`.
- Pesan commit singkat ala Conventional Commits: `feat: ...`, `fix: ...`, `chore: ...`.

## 11. Aturan Kerja Agent

1. Baca file relevan dulu (`read`/`grep`/`glob`) sebelum edit. Jangan berasumsi isi file.
2. Prefer edit file existing daripada buat file baru. Jangan buat file docs (`*.md`) selain yang diminta.
3. Jaga perubahan minimal dan fokus pada permintaan. Jangan refactor tak diminta.
4. Verifikasi dengan eksekusi: run build/test yang relevan, laporkan bukti (lolos/gagal + output ringkas).
5. Jika ada ambiguitas (pilih lib, skema DB, contract), berhenti dan tanya user via pertanyaan singkat. Jangan menebak hal load-bearing.
6. Ringkasan akhir untuk user: file diubah + `path:line`, perintah verifikasi yang dijalankan, dan follow-up yang belum dikerjakan.

## 12. Hal yang Dilarang

- Menaruh secret/kredensial di kode atau di AGENTS.md.
- Menambah dependensi besar tanpa konfirmasi (ORM berat, framework UI, dsb).
- Mengubah versi Go/Node atau toolchain tanpa diminta.
- Membuat file di luar workspace (`C:\Users\Lenovo\AppData\Local\Temp\opencode` hanya untuk file sementara).
