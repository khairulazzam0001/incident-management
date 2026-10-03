# PRD — Knowledge Base (Enhancement)

**Produk:** Incident Management System
**Versi dokumen:** 0.1 (DRAFT untuk review)
**Tanggal:** 2026-10-03
**Status:** Menunggu review & keputusan user (lihat §19)
**Referensi:** PRD MVP §4 (Non-Goals: *Knowledge Base sebagai produk terpisah*), §21
**Bergantung pada:** MVP. Integrasi opsional dengan Problem Management (Known Error → artikel) dan AI Assist (saran artikel).

---

## 1. TL;DR

Knowledge Base (KB) adalah kumpulan **artikel solusi** yang ditulis tim dan **terhubung dengan incident**:
- Saat user/Help Desk membuat incident, sistem menyarankan artikel yang relevan, sehingga sebagian masalah bisa selesai tanpa tiket (*deflection*).
- Saat PIC menyelesaikan incident, solusinya bisa diubah jadi artikel dengan satu klik.
- Artikel melewati review sebelum terbit, punya versi, dan bisa **internal** (tim) atau **publik** (User/Customer).

KB tidak dibuat sebagai produk terpisah; ia modul di aplikasi yang sama. Ini konsisten dengan non-goal PRD MVP.

## 2. Problem Statement

- Solusi dari incident yang sudah selesai hanya tersimpan di `fix`/komentar; sulit ditemukan lagi.
- Help Desk bergantung pada ingatan senior untuk masalah yang sama.
- User melaporkan hal yang sebenarnya bisa diselesaikan sendiri (reset cache, cara akses, dst).

## 3. Goals

- Mempercepat triase & penyelesaian lewat pengetahuan yang bisa dicari.
- Mengurangi incident yang bisa diselesaikan sendiri oleh user.
- Menjaga kualitas artikel (review, versi, kedaluwarsa).

## 4. Non-Goals (fase ini)

- Portal KB publik untuk pihak luar tanpa login.
- Editor WYSIWYG kaya (tabel, embed video). Cukup Markdown.
- Multi-bahasa/terjemahan artikel.
- Pencarian semantik berbasis embedding (lihat PRD_AI_Assist.md).

## 5. Users & Roles

| Role | Aksi |
|---|---|
| User/Customer | Baca & cari artikel **PUBLIC** yang PUBLISHED; beri feedback "membantu?". |
| Help Desk, Developer, DevOps, QA | Baca semua artikel PUBLISHED; tulis draft; ajukan review; link artikel ke incident. |
| System Analyst, Manager/Lead | Semua di atas + **review/publish/archive** (reviewer ≠ penulis versi tersebut). |

## 6. Konsep

### 6.1 Tipe artikel
- `HOW_TO`: langkah melakukan sesuatu.
- `TROUBLESHOOTING`: gejala → penyebab → solusi.
- `KNOWN_ERROR`: berasal dari Problem; berisi workaround.
- `FAQ`: tanya-jawab singkat.

### 6.2 Visibilitas
- `INTERNAL`: role selain User.
- `PUBLIC`: termasuk User/Customer.

Artikel PUBLIC wajib lolos checklist review "tidak memuat data sensitif" (PRD MVP §15).

### 6.3 Lifecycle

```
DRAFT ──submit──▶ IN_REVIEW ──publish──▶ PUBLISHED ──archive──▶ ARCHIVED
   ▲                 │ request changes          │ edit (versi baru = DRAFT revisi)
   └─────────────────┘                          ▼
                                     PUBLISHED tetap tampil sampai revisi terbit
```

- **Versi:**
  - Setiap publish menyimpan snapshot ke `trans_kb_article_version`.
  - Mengedit artikel PUBLISHED membuat revisi draft tanpa menurunkan versi yang sedang terbit.
- **Review:** publisher ≠ penulis revisi, dengan segregation of duties seperti approval change.
- **Kedaluwarsa:** `review_due_at` (default 180 hari setelah publish). Artikel lewat tanggal ditandai "perlu ditinjau" dan owner mendapat notifikasi.

### 6.4 Pencarian
- PostgreSQL full-text search:
  - kolom `search_vector` (title berbobot A, summary B, body C), konfigurasi `simple` karena tidak ada stemmer Bahasa Indonesia bawaan;
  - digabung dengan `pg_trgm` pada title untuk toleransi typo.
- Filter: tipe, aplikasi, kategori, tag, visibilitas.
- Hasil diurutkan berdasarkan relevansi lalu `helpful_ratio`.

### 6.5 Integrasi incident
- **Saran saat create incident:** saat title diketik (debounce 400 ms), panel "Mungkin ini membantu" menampilkan 3 artikel teratas. Klik dicatat (`kb_suggestion_click`). User bisa menandai "Masalah selesai, tidak jadi buat incident". Ini data deflection.
- **Link artikel ↔ incident:**
  - relasi `USED_FOR` (artikel dipakai untuk menyelesaikan) atau `CREATED_FROM` (artikel dibuat dari incident);
  - tampil di Incident Detail dan Article Detail.
- **"Jadikan artikel"** di incident RESOLVED/CLOSED: draft TROUBLESHOOTING dengan pre-fill gejala (deskripsi), penyebab (findings investigation), dan solusi (fix description).
- **Known Error → KB:** bila PRD Problem dieksekusi, workaround dipublikasikan sebagai artikel KNOWN_ERROR.

## 7. User Stories & Acceptance Criteria

- **KB-01 Tulis draft.** Role internal membuat artikel → DRAFT, slug unik, `article_no` `KB-000123`.
- **KB-02 Review & publish.** Penulis submit → IN_REVIEW. Analyst/Manager (bukan penulis) publish → PUBLISHED + versi 1. Penulis publish sendiri → 403.
- **KB-03 Revisi.** Edit artikel PUBLISHED → revisi draft. Pembaca tetap melihat versi terbit sampai revisi dipublish (versi 2).
- **KB-04 Visibilitas.** User/Customer hanya melihat PUBLIC+PUBLISHED. Akses artikel INTERNAL → 404 (tidak membocorkan keberadaan).
- **KB-05 Cari.** Query "checkout timeout" mengembalikan artikel relevan < 300 ms untuk 10.000 artikel (indeks GIN).
- **KB-06 Saran saat create incident.** Mengetik title menampilkan ≤ 3 saran sesuai visibilitas role.
- **KB-07 Feedback.** Satu feedback per user per versi artikel (helpful yes/no + komentar opsional).
- **KB-08 Dari incident.** Tombol "Jadikan artikel" mengisi template dan menautkan `CREATED_FROM`.
- **KB-09 Kedaluwarsa.** Artikel melewati `review_due_at` → badge "perlu ditinjau" + notifikasi owner.

## 8. Functional Requirements

| ID | Requirement | Priority |
|---|---|---|
| KB-FR-01 | CRUD artikel + kategori + tag + Markdown body. | Must |
| KB-FR-02 | Lifecycle review/publish/archive + segregation of duties. | Must |
| KB-FR-03 | Versi artikel + revisi tanpa menurunkan versi terbit. | Must |
| KB-FR-04 | Visibilitas INTERNAL/PUBLIC ditegakkan server-side. | Must |
| KB-FR-05 | Full-text search + filter. | Must |
| KB-FR-06 | Link artikel ↔ incident + "jadikan artikel". | Must |
| KB-FR-07 | Saran artikel di form create incident + tracking deflection. | Should |
| KB-FR-08 | Feedback helpful + view count. | Should |
| KB-FR-09 | Review due/kedaluwarsa + notifikasi. | Should |
| KB-FR-10 | Lampiran gambar di artikel (reuse aturan FR-09). | Could |
| KB-FR-11 | Known Error → KB (bila Problem ada). | Could |

## 9. Data Model (migrasi baru)

- `master_kb_category` (id, code, name, parent_id nullable).
- `trans_kb_article`:
  - id, article_no (UNIQUE, sequence), slug UNIQUE
  - type, visibility, status
  - title, summary, body_md
  - category_id, application_id nullable, tags TEXT[]
  - author_id, owner_id, current_version, review_due_at
  - view_count, helpful_yes, helpful_no
  - `search_vector tsvector` (generated column)
  - created_at, updated_at, published_at, archived_at
- `trans_kb_article_version`: article_id, version, title, summary, body_md, published_by, published_at. UNIQUE `(article_id, version)`.
- `trans_kb_article_draft`: revisi yang sedang disunting untuk artikel PUBLISHED (satu per artikel).
- `trans_kb_article_activity` (audit).
- `trans_kb_article_incident`: article_id, incident_id, relation (`USED_FOR`/`CREATED_FROM`), created_by. UNIQUE `(article_id, incident_id, relation)`.
- `trans_kb_feedback`: article_id, version, user_id, helpful bool, comment, created_at. UNIQUE `(article_id, version, user_id)`.
- `trans_kb_suggestion_event`: user_id, query, article_id, action (`shown`/`clicked`/`deflected`), created_at. Dipakai untuk metrik deflection.
- Index: GIN pada `search_vector`, GIN trigram pada `title`, `(status, visibility)`.

## 10. API Contract (tambahan)

```
GET    /api/kb/articles?q=&type=&category=&application_id=&tag=&status=&page=&limit=
POST   /api/kb/articles
GET    /api/kb/articles/:idOrSlug            # versi terbit (atau draft bila berhak)
PATCH  /api/kb/articles/:id                  # edit draft / revisi
POST   /api/kb/articles/:id/submit
POST   /api/kb/articles/:id/review           # {decision: PUBLISH|REQUEST_CHANGES, reason}
POST   /api/kb/articles/:id/archive          # {reason}
GET    /api/kb/articles/:id/versions
POST   /api/kb/articles/:id/feedback         # {helpful, comment}
GET    /api/kb/suggest?q=                    # untuk form create incident (≤ 3)
POST   /api/kb/suggest/events                # {query, article_id?, action}
POST   /api/kb/articles/:id/incidents        # {incident_id, relation}
GET    /api/incidents/:id/kb-articles
POST   /api/incidents/:id/kb-draft           # "jadikan artikel"
GET    /api/master/kb-categories | POST | DELETE (Manager)
```

## 11. UX / Pages

| Page | Isi |
|---|---|
| Sidebar "Knowledge Base" | Untuk semua role. User hanya melihat artikel publik. |
| `KbHome` | Pencarian besar, kategori, artikel populer & terbaru. |
| `KbArticle` | Judul, tipe, versi, terakhir ditinjau, isi Markdown, "membantu?", incident terkait (internal). |
| `KbEditor` | Form + editor Markdown dengan preview, pilih tipe/visibilitas/kategori/tag, submit review. |
| `KbReviewQueue` | Untuk Analyst/Manager: daftar IN_REVIEW & perlu ditinjau. |
| Incident New | Panel saran artikel di samping form. |
| Incident Detail | Panel "Artikel terkait" + tombol "Jadikan artikel" (RESOLVED/CLOSED). |

Rendering Markdown harus aman: HTML mentah dilarang, link eksternal `rel="noopener noreferrer"`. Lihat Q2 untuk pilihan library.

## 12. Notifications

| Event | Penerima |
|---|---|
| `kb_review_requested` | Analyst + Manager aktif (kecuali penulis) |
| `kb_published` / `kb_changes_requested` | Penulis |
| `kb_review_due` | Owner artikel |
| `kb_negative_feedback` (≥ 3 "tidak membantu" pada versi yang sama) | Owner |

## 13. Success Metrics

| Kategori | Metric |
|---|---|
| Outcome | Deflection rate (sesi saran yang berakhir "tidak jadi buat incident"); MTTR incident yang memakai artikel vs tidak. |
| Leading | # artikel PUBLISHED; % incident RESOLVED ter-link ke artikel; helpful ratio. |
| Guardrail | % artikel lewat review due; artikel PUBLIC yang di-archive karena data sensitif. |

## 14. Milestones

| Fase | Estimasi | Scope |
|---|---|---|
| KB-1 | ± 1–1,5 minggu | Artikel CRUD, lifecycle + review, versi, visibilitas, search FTS, halaman KbHome/Article/Editor. |
| KB-2 | ± 1 minggu | Link incident, "jadikan artikel", saran di create incident + event deflection, feedback. |
| KB-3 | ± 0,5 minggu | Review due + notifikasi, review queue, lampiran gambar, Known Error → KB. |

## 15. Acceptance Test Matrix (ringkas)

- Penulis publish sendiri → 403; reviewer lain → PUBLISHED v1.
- Revisi PUBLISHED: GET publik tetap v1 sampai revisi terbit.
- User/Customer GET artikel INTERNAL → 404; search tidak mengembalikan INTERNAL untuk User.
- Search relevansi: artikel dengan kata di title mengalahkan yang hanya di body.
- Feedback ganda per versi → 409; versi baru boleh feedback lagi.
- "Jadikan artikel" dari incident NEW → 409 (harus RESOLVED/CLOSED).
- Body berisi `<script>` tidak dieksekusi di UI (test render).

## 16. Risiko

- **Kebocoran data di artikel PUBLIC:** checklist review wajib + reviewer ≠ penulis.
- **Artikel basi:** review due + metrik guardrail.
- **Relevansi pencarian Bahasa Indonesia** terbatas tanpa stemmer: diterima untuk fase ini; peningkatan lewat AI Assist (embedding).

## 17. Coding-Agent Handoff (ringkas)

Paket service/repository `kb_*.go`. Search query memakai `websearch_to_tsquery('simple', $1)` + `similarity(title, $1)`. Markdown dirender di frontend.

## 18. Dependensi

- Ekstensi PostgreSQL `pg_trgm`.
- Frontend: library Markdown + sanitizer (**dependensi baru, butuh persetujuan** — Q2).

## 19. Keputusan yang Dibutuhkan

| # | Pertanyaan | Usulan default |
|---|---|---|
| Q1 | User/Customer boleh membaca artikel PUBLIC? | Ya (tujuan deflection). |
| Q2 | Library Markdown untuk frontend? | `react-markdown` (aman by default, tanpa HTML mentah) + `remark-gfm`. Alternatif tanpa dependensi: teks polos + baris baru (kurang nyaman). |
| Q3 | Siapa reviewer/publisher? | System Analyst & Manager/Lead, bukan penulis versi tsb. |
| Q4 | Periode review ulang default? | 180 hari. |
| Q5 | Format konten: Markdown cukup? | Ya. |
| Q6 | Saran artikel juga untuk User/Customer di form create? | Ya, terbatas artikel PUBLIC. |
