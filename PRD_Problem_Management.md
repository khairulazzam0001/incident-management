# PRD — Problem Management (Enhancement)

**Produk:** Incident Management System
**Versi dokumen:** 0.1 (DRAFT untuk review)
**Tanggal:** 2026-10-03
**Status:** Menunggu review & keputusan user (lihat §19)
**Referensi:** PRD MVP §4 (Non-Goals: *Problem Management penuh*), §2 (*analisis incident berulang*), §21
**Bergantung pada:** Change Management (sudah ada) untuk permanent fix. Integrasi dengan Knowledge Base (PRD_Knowledge_Base.md) bersifat opsional.

---

## 1. TL;DR

**Incident** = gangguan yang dipulihkan secepatnya. **Problem** = *akar penyebab* di balik satu atau banyak incident.

Problem Management menambahkan **Problem Record** (`PRB-2026-000001`) untuk:
1. Mengelompokkan incident yang berulang.
2. Mencatat **analisis akar masalah (RCA)**.
3. Mendokumentasikan **workaround** (Known Error) agar Help Desk cepat memulihkan incident serupa.
4. Melacak **permanent fix** lewat Change Request sampai problem selesai.

Alur: **New → Investigating (RCA) → Known Error → Fix in Progress → Resolved → Closed**.

## 2. Problem Statement

- Incident yang sama (mis. "checkout timeout") bisa muncul berkali-kali; masing-masing ditutup terpisah tanpa ada yang memperbaiki akarnya.
- Temuan investigasi tersebar di banyak incident (`trans_incident_investigation`), tidak dirangkum.
- Workaround hanya diingat orang tertentu, dan Help Desk mengulang triase dari nol.
- Tidak ada data "berapa incident yang disebabkan masalah yang sama" untuk prioritas engineering.

## 3. Goals

- Satu tempat untuk RCA, workaround, dan status permanent fix.
- Mengurangi incident berulang (repeat incident rate).
- Mempercepat pemulihan incident serupa lewat workaround yang terdokumentasi.
- Keterkaitan penuh: Incident ↔ Problem ↔ Change.

## 4. Non-Goals (fase ini)

- Proactive problem management berbasis AI/anomaly detection (lihat PRD_AI_Assist.md).
- Template RCA kompleks (fishbone diagram visual, fault tree).
- Problem lintas organisasi/vendor (vendor ticket integration).
- Auto-close incident ketika problem resolved.

## 5. Users & Roles

| Role | Aksi |
|---|---|
| Help Desk | Buat problem dari incident, link incident, pakai workaround. |
| System Analyst | **Problem owner** default: RCA, known error, koordinasi fix. |
| Developer / DevOps | Kontributor RCA, buat change permanent fix. |
| QA | Read-only + komentar. |
| Manager/Lead | Prioritas problem, assign owner, close/cancel, laporan. |
| User/Customer | Tidak punya akses (problem bersifat internal). Workaround tetap bisa tampil di incident miliknya bila diputuskan (Q4). |

**Permission matrix ringkas:**
- Create: HelpDesk, Analyst, Developer, DevOps, Manager.
- Edit RCA/workaround: owner atau Manager.
- Ubah status: owner atau Manager.
- Close/Cancel: Manager atau Analyst.
- Link incident: semua role internal kecuali QA.

## 6. Lifecycle

| Status | Definisi | Transisi |
|---|---|---|
| `NEW` | Problem dicatat, belum dianalisis. | → INVESTIGATING, → CANCELLED |
| `INVESTIGATING` | RCA berjalan; owner wajib ada. | → KNOWN_ERROR, → FIX_IN_PROGRESS, → CANCELLED |
| `KNOWN_ERROR` | Root cause **dan** workaround terdokumentasi; fix belum ada/tertunda. | → FIX_IN_PROGRESS, → CLOSED (accepted risk, reason wajib) |
| `FIX_IN_PROGRESS` | Minimal satu change `FIX_FOR` ter-link. | → RESOLVED, → KNOWN_ERROR (fix gagal/dibatalkan) |
| `RESOLVED` | Change permanent fix CLOSED dengan outcome SUCCESS; menunggu masa observasi. | → CLOSED, → INVESTIGATING (incident terkait muncul lagi) |
| `CLOSED` | Selesai. | Terminal |
| `CANCELLED` | Duplikat/bukan problem (reason + `duplicate_of` opsional). | Terminal |

**Aturan:**
- `→ KNOWN_ERROR` wajib mengisi `root_cause` dan `workaround`.
- `→ FIX_IN_PROGRESS` wajib ada change link `FIX_FOR` yang tidak REJECTED/CANCELLED.
- `→ RESOLVED` wajib ada change FIX_FOR berstatus CLOSED + outcome SUCCESS. Sistem **menyarankan** transisi ini saat change ditutup (notifikasi ke owner), tidak otomatis.
- Masa observasi (default 14 hari, Q5): bila incident baru di-link ke problem RESOLVED, problem kembali ke INVESTIGATING dan owner diberi notifikasi.
- Semua transisi: guard status (409 bila stale), actor + timestamp di timeline, tanpa duplikasi saat double submit.

## 7. Integrasi

### 7.1 Incident ↔ Problem
- Relasi: satu incident maksimal satu problem; satu problem banyak incident.
- "Buat Problem dari incident" di Incident Detail: pre-fill title, application, environment, dan link incident sumber.
- Multi-select di Incident List → "Kelompokkan ke Problem" (baru atau yang sudah ada).
- Incident Detail menampilkan panel **Known Error / Workaround** bila problem-nya punya workaround.
- Link/unlink tercatat di kedua timeline.

### 7.2 Kandidat problem (deteksi sederhana, tanpa AI)
Halaman "Kandidat Problem" menampilkan kelompok incident dengan:
- application + environment sama,
- judul mirip (pg_trgm `similarity ≥ 0.4`, Q6),
- ≥ 3 kejadian dalam 30 hari terakhir,
- belum ter-link ke problem.

Daftar ini hanya saran; manusia yang memutuskan.

### 7.3 Problem ↔ Change
- Relasi baru `FIX_FOR` change → problem, tabel terpisah dari link change ↔ incident.
- Change Detail menampilkan "Problem terkait".
- Change CLOSED SUCCESS → notifikasi owner problem: "permanent fix selesai, tandai RESOLVED?".

### 7.4 Problem → Knowledge Base (bila PRD KB dieksekusi)
Known Error dapat dipublikasikan sebagai artikel KB tipe `KNOWN_ERROR` dengan satu klik (draft, lewat review KB).

## 8. User Stories & Acceptance Criteria

- **PRB-01 Create.** Role berhak membuat problem → NEW + nomor `PRB-YYYY-NNNNNN`. Field wajib: title (≥ 5), description, application, impact (S1–S4), priority (P1–P4).
- **PRB-02 Link incident.** Link incident ke problem. Incident yang sudah punya problem lain → 409 (`INCIDENT_ALREADY_LINKED`). Tercatat di kedua timeline.
- **PRB-03 RCA.** Owner mengisi symptoms, root_cause, contributing_factors, rca_method (5-Whys / Timeline / Other); setiap revisi masuk timeline.
- **PRB-04 Known error.** Transisi ke KNOWN_ERROR tanpa root_cause/workaround → 400. Incident terkait yang masih terbuka mendapat notifikasi "workaround tersedia" ke PIC-nya.
- **PRB-05 Permanent fix.** Buat change dari problem (pre-fill, link FIX_FOR) → problem bisa FIX_IN_PROGRESS.
- **PRB-06 Resolved.** Tanpa change CLOSED SUCCESS → 409 `FIX_NOT_VERIFIED`.
- **PRB-07 Recurrence.** Incident baru di-link ke problem RESOLVED dalam masa observasi → problem kembali INVESTIGATING + notifikasi.
- **PRB-08 Cancel/duplicate.** Reason wajib; `duplicate_of` memindahkan link incident ke problem tujuan (opsional).

## 9. Functional Requirements

| ID | Requirement | Priority |
|---|---|---|
| PRB-FR-01 | CRUD problem + nomor unik + lifecycle terkontrol. | Must |
| PRB-FR-02 | Link/unlink incident (1 problem per incident). | Must |
| PRB-FR-03 | RCA fields + workaround + timeline + komentar. | Must |
| PRB-FR-04 | Link change FIX_FOR ↔ problem + gate RESOLVED. | Must |
| PRB-FR-05 | Panel workaround di Incident Detail. | Must |
| PRB-FR-06 | List/filter/search problem + pagination. | Must |
| PRB-FR-07 | Notifikasi (owner assigned, known error, fix selesai, recurrence). | Should |
| PRB-FR-08 | Halaman kandidat problem (rule-based). | Should |
| PRB-FR-09 | Dashboard problem (aktif, umur, top problem by incident count). | Should |
| PRB-FR-10 | Attachment problem (reuse aturan FR-09). | Could |
| PRB-FR-11 | Publish known error ke KB. | Could (tergantung PRD KB) |

## 10. Data Model (migrasi baru)

- `master_problem_status` (code, name, sort_order, is_terminal). Seed sesuai §6.
- `trans_problem`:
  - id, problem_no (UNIQUE, sequence `problem_no_seq`), title, description
  - application_id, environment_code, impact_code (FK severity), priority_code, status_code
  - owner_id, created_by
  - symptoms, root_cause, contributing_factors, rca_method, workaround
  - duplicate_of, observation_until
  - created_at, updated_at, resolved_at, closed_at, closed_by
- `trans_problem_incident`: problem_id, incident_id **UNIQUE** (1 problem per incident), created_by, created_at.
- `trans_problem_activity` (pola sama dengan activity incident/change, `created_at DEFAULT clock_timestamp()`).
- `trans_problem_comment`.
- `trans_change_problem`: change_id, problem_id, relation (`FIX_FOR`), UNIQUE pair.
- `trans_notification`: tambah kolom nullable `problem_id` + perluas CHECK (minimal satu subjek).
- Index: status, owner, application, `created_at DESC`, `trans_problem_incident(problem_id)`.
- Ekstensi `pg_trgm` untuk kandidat problem (Q6).

## 11. API Contract (tambahan)

```
GET    /api/problems?status=&priority=&application_id=&owner=&q=&page=&limit=
POST   /api/problems                       # incident_ids[] opsional → link awal
GET    /api/problems/:id
PATCH  /api/problems/:id                   # field + RCA/workaround (owner/Manager)
POST   /api/problems/:id/status            # {status, reason?}
POST   /api/problems/:id/incidents         # {incident_id}
DELETE /api/problems/:id/incidents/:incidentId
GET    /api/problems/:id/incidents
GET    /api/problems/:id/timeline
POST   /api/problems/:id/comments | GET
POST   /api/problems/:id/changes           # {change_id} link FIX_FOR
GET    /api/problems/:id/changes
GET    /api/incidents/:id/problem          # problem + workaround untuk Incident Detail
GET    /api/problems/candidates            # §7.2
GET    /api/problems/summary               # dashboard
```

`POST /api/changes` menerima `problem_id` opsional, sejajar dengan `incident_id`. Penambahan ini backward compatible.

## 12. UX / Pages

| Page | Isi |
|---|---|
| Sidebar "Problems" | Untuk role internal. |
| `ProblemList` | Filter status/priority/app/owner, kolom jumlah incident terkait. |
| `ProblemNew` | Form; dari incident dengan `?incident=`. |
| `ProblemDetail` | Header + aksi status, tab RCA (form terstruktur), Workaround, Incidents, Changes, Timeline, Komentar. |
| `ProblemCandidates` | Kelompok incident berulang + tombol "Buat Problem". |
| Incident Detail | Panel "Problem & Workaround" + tombol buat/link problem. |
| Change Detail | "Problem terkait". |
| Dashboard | Section Problem. |

Mengikuti DESIGN.md. Workaround ditampilkan menonjol (kartu lilac) agar mudah dipakai Help Desk.

## 13. Notifications

| Event | Penerima |
|---|---|
| `problem_assigned` | Owner baru |
| `problem_known_error` | PIC incident terkait yang masih terbuka |
| `problem_fix_completed` | Owner (change permanent fix CLOSED SUCCESS) |
| `problem_recurred` | Owner + ManagerLead |
| `problem_closed` | Pembuat + owner |

## 14. Success Metrics

| Kategori | Metric |
|---|---|
| Outcome | Repeat incident rate (incident ter-link ke problem yang sama dalam 90 hari) turun; # problem resolved/bulan. |
| Leading | % incident S1/S2 ter-link ke problem; mean time to RCA (NEW → KNOWN_ERROR); # known error dengan workaround. |
| Guardrail | Umur problem INVESTIGATING > 30 hari; problem RESOLVED yang recur. |

## 15. Milestones

| Fase | Estimasi | Scope |
|---|---|---|
| PRB-1 | ± 1–1,5 minggu | Problem CRUD + lifecycle + link incident + RCA/workaround + timeline/komentar + UI list/detail + panel di Incident Detail. |
| PRB-2 | ± 1 minggu | Link change FIX_FOR + gate RESOLVED + observasi/recurrence + notifikasi. |
| PRB-3 | ± 0,5–1 minggu | Kandidat problem (pg_trgm), dashboard, attachment, publish ke KB (bila ada). |

## 16. Acceptance Test Matrix (ringkas)

- Create → NEW + nomor; field kurang → 400; role User/QA → 403.
- Link incident yang sudah punya problem lain → 409; unlink → 200, lalu lagi → 404.
- KNOWN_ERROR tanpa workaround → 400; FIX_IN_PROGRESS tanpa change → 409; RESOLVED tanpa change CLOSED SUCCESS → 409.
- Double submit status → 409 STALE_STATUS tanpa activity ganda.
- Recurrence: link incident baru ke RESOLVED dalam masa observasi → INVESTIGATING + notifikasi.
- Kandidat: 3 incident mirip/app sama/30 hari → muncul; sudah ter-link → hilang.

## 17. Risiko

- **Overlap dengan Change/Incident:** batas tegas. Incident = pemulihan; Problem = akar; Change = eksekusi perbaikan.
- **Problem jadi "kuburan":** metric umur + review bulanan Manager.
- **Kualitas RCA rendah:** field terstruktur + wajib saat KNOWN_ERROR.

## 18. Dependensi

- Ekstensi PostgreSQL `pg_trgm` (contrib, umumnya tersedia di image `postgres:15-alpine`).
- Tidak ada dependensi Go/npm baru.

## 19. Keputusan yang Dibutuhkan

| # | Pertanyaan | Usulan default |
|---|---|---|
| Q1 | Satu incident boleh ter-link ke lebih dari satu problem? | Tidak (1 problem per incident), agar metrik bersih. |
| Q2 | Siapa owner default problem? | Pembuat bila Analyst/Manager; selain itu wajib dipilih (Analyst/Manager). |
| Q3 | RESOLVED otomatis saat change CLOSED SUCCESS, atau manual? | Manual + notifikasi saran (manusia memastikan). |
| Q4 | Workaround boleh terlihat oleh User/Customer di incident miliknya? | Tidak di fase ini (internal saja). |
| Q5 | Lama masa observasi setelah RESOLVED? | 14 hari. |
| Q6 | Boleh mengaktifkan ekstensi `pg_trgm`? | Ya (dipakai juga oleh KB & AI similar search). |
| Q7 | Problem harus punya incident, atau boleh proaktif tanpa incident? | Boleh tanpa incident (proaktif). |
