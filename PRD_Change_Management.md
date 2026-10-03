# PRD — Change Management (Enhancement)

**Produk:** Incident Management System
**Versi dokumen:** 0.2 (keputusan Q1–Q7 disepakati)
**Tanggal:** 2026-10-01
**Status:** Disetujui — CM-1, CM-2, CM-3, dan CM-FR-13 terimplementasi
**Referensi:** `PRD_Incident_Management_MVP(1).pdf` §4 (Non-Goals) dan §21 (Recommended Future Enhancements)

---

## 1. TL;DR

Change Management menambahkan **Change Request (CR)**, yaitu catatan terkontrol untuk setiap perubahan pada aplikasi/infrastruktur (deploy, konfigurasi, migrasi DB, patch). Satu CR berisi rencana, risiko, rencana rollback, persetujuan, jadwal, hasil implementasi, dan review pasca-implementasi.

Core workflow: **Draft → Submit → Approve → Schedule → Implement → Review → Close**.

Fitur ini terhubung langsung dengan incident:
- **Incident → Change**: fix incident yang butuh deploy ke production dijalankan lewat CR (relasi `FIX_FOR`).
- **Change → Incident**: incident yang muncul akibat sebuah change dapat ditautkan (relasi `CAUSED_BY`). Dari sini terukur *change failure rate*.

## 2. Problem Statement

Saat ini perbaikan incident hanya dicatat sebagai `trans_incident_fix.reference` (teks bebas berisi link PR/deploy). Akibatnya:

- Tidak ada persetujuan formal sebelum perubahan masuk ke production.
- Risiko dan rollback plan tidak terdokumentasi, sehingga rollback saat gagal sering improvisasi.
- Jadwal perubahan tidak terlihat, sehingga dua tim bisa men-deploy aplikasi/environment yang sama di jam yang sama.
- Sulit menjawab pertanyaan "incident ini muncul gara-gara deploy apa?" dan "berapa persen change yang gagal?".
- Audit (siapa mengubah apa, kapan, disetujui siapa) tersebar di chat/email.

## 3. Goals

**Business goals**
- Satu sumber data untuk semua perubahan terencana dan darurat.
- Setiap perubahan ke production punya approval, risk assessment, dan rollback plan.
- Menurunkan incident yang disebabkan oleh change (*change-induced incident*).
- Menyediakan data change success/failure rate untuk reporting.

**User goals**
- Developer/DevOps dapat mengajukan CR dengan cepat, termasuk langsung dari incident yang sedang di-fix.
- Manager/Lead dapat menyetujui atau menolak CR dengan informasi risiko yang cukup.
- Implementer tahu kapan jadwalnya dan mencatat hasilnya (sukses/gagal/rollback).
- System Analyst/Help Desk dapat melihat change apa yang baru terjadi pada aplikasi yang sedang bermasalah.

## 4. Non-Goals (fase ini)

- CAB meeting scheduling / agenda rapat.
- Approval multi-level / multi-approver (diputuskan: cukup satu approval, Q1).
- Integrasi otomatis CI/CD (webhook deploy → update CR). Bisa menyusul.
- CMDB / configuration item. Relasi cukup ke `master_application` + `master_environment`.
- Release Management (bundling banyak change jadi satu release).
- Change template / catalog standard change yang bisa dikelola user (seed tetap via migrasi).
- Change freeze window / blackout period (Later).

## 5. Users & Roles

Role yang dipakai tetap role yang sudah ada (`model/user.go`). Tidak ada role baru.

| Role | Tanggung jawab di Change Management |
|---|---|
| User / Customer | Tidak punya akses ke modul change. |
| Help Desk / Support | Melihat change (read-only) untuk konteks triase incident. |
| System Analyst | Membuat CR, menjadi implementer, melakukan review/close. |
| Developer | Membuat CR, menjadi implementer. |
| QA | Melihat change, memberi komentar hasil test. |
| DevOps | Membuat CR, menjadi implementer (umumnya deploy/infra). |
| Manager / Lead | **Approver** (CAB tunggal), close/review, cancel. |

### Permission matrix

| Aksi | User | HelpDesk | Analyst | Developer | QA | DevOps | Manager |
|---|---|---|---|---|---|---|---|
| Lihat list/detail change | – | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Create / edit DRAFT | – | – | ✓ | ✓ | – | ✓ | ✓ |
| Submit | – | – | requester | requester | – | requester | requester |
| Approve / Reject / Request changes | – | – | – | – | – | – | ✓ (bukan requester) |
| Schedule | – | – | requester/implementer | requester/implementer | – | requester/implementer | ✓ |
| Start / complete implementation | – | – | implementer | implementer | – | implementer | ✓ |
| Review & close | – | – | ✓ | – | – | – | ✓ |
| Cancel | – | – | requester | requester | – | requester | ✓ |
| Komentar | – | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Link/unlink incident | – | ✓ | ✓ | ✓ | – | ✓ | ✓ |

**Segregation of duties:** requester **tidak boleh** meng-approve CR miliknya sendiri. Aturan ini juga berlaku untuk Manager yang membuat CR, sehingga dibutuhkan Manager lain. **Pengecualian (Q2):** CR berisiko `LOW`/`MEDIUM` yang requester-nya ManagerLead boleh di-approve oleh System Analyst.

## 6. Change Types & Risk

### 6.1 Change Type (`master_change_type`)

| Code | Nama | Approval | Catatan |
|---|---|---|---|
| `STANDARD` | Standard | **Pre-approved**: submit langsung menjadi APPROVED (actor = system, tercatat di timeline). | Perubahan rutin berisiko rendah (restart service, rotate log, scale replica). Risk wajib `LOW`. |
| `NORMAL` | Normal | Wajib approval Manager/Lead. | Default untuk sebagian besar perubahan. |
| `EMERGENCY` | Emergency | Wajib approval Manager/Lead **sebelum implementasi** (Q4; tidak ada approval retroaktif), tetapi **jadwal opsional** (boleh langsung implement setelah approve). **PIR (post-implementation review) wajib** sebelum close. | Untuk fix incident S1/P1 yang tidak bisa menunggu. |

### 6.2 Risk Level (`master_change_risk`)

| Code | Nama | Panduan |
|---|---|---|
| `LOW` | Low | Dampak lokal, rollback < 15 menit, sudah pernah dilakukan. |
| `MEDIUM` | Medium | Menyentuh fitur bisnis, rollback teruji, ada downtime kecil. |
| `HIGH` | High | Menyentuh data/skema, banyak service terdampak, rollback sulit, atau di jam sibuk. |

Risk dipilih manual oleh requester. MVP tidak memakai kalkulator otomatis.
- Approver dapat menolak atau meminta revisi bila risk tidak sesuai.
- Risk `HIGH` mewajibkan `test_plan` diisi.

## 7. Change Lifecycle

```
          ┌──────── request changes (reason wajib) ────────┐
          ▼                                                 │
DRAFT ──submit──▶ SUBMITTED ──approve──▶ APPROVED ──schedule──▶ SCHEDULED ──start──▶ IMPLEMENTING ──complete──▶ REVIEWING ──close──▶ CLOSED
                     │                                                                (outcome:
                     └──reject (reason)──▶ REJECTED  (terminal)                       SUCCESS / FAILED / ROLLED_BACK)

DRAFT / SUBMITTED / APPROVED / SCHEDULED ──cancel (reason)──▶ CANCELLED (terminal)
```

| Status | Definisi | Transisi yang diizinkan |
|---|---|---|
| `DRAFT` | CR sedang disusun, masih bisa diedit. | → SUBMITTED, → CANCELLED |
| `SUBMITTED` | Menunggu keputusan approver. Field terkunci. | → APPROVED, → REJECTED, → DRAFT (request changes), → CANCELLED |
| `APPROVED` | Disetujui, belum dijadwalkan. | → SCHEDULED, → CANCELLED; khusus `EMERGENCY`: → IMPLEMENTING |
| `SCHEDULED` | Window `planned_start`–`planned_end` sudah ditetapkan. | → IMPLEMENTING, → SCHEDULED (reschedule), → CANCELLED |
| `IMPLEMENTING` | Sedang dikerjakan; `actual_start` tercatat. | → REVIEWING (dengan outcome) |
| `REVIEWING` | Implementasi selesai; `actual_end` + outcome tercatat. Menunggu review/PIR. | → CLOSED |
| `CLOSED` | Selesai. | Terminal |
| `REJECTED` | Ditolak approver. | Terminal (buat CR baru bila ingin mengajukan ulang) |
| `CANCELLED` | Dibatalkan sebelum implementasi. | Terminal |

**Aturan khusus**
- `STANDARD`: submit langsung menghasilkan `SUBMITTED → APPROVED` dalam satu transaksi. Timeline mencatat dua activity, yang kedua dengan actor `system` dan payload `{auto_approved: true}`.
- **Request changes**: `SUBMITTED → DRAFT`, reason wajib, `revision` bertambah 1. Approval berikutnya tercatat untuk revision baru.
- **Outcome** wajib saat complete: `SUCCESS`, `FAILED`, atau `ROLLED_BACK`. Untuk `FAILED`/`ROLLED_BACK`, `outcome_notes` wajib.
- **PIR** (`review_notes`) wajib saat close bila type `EMERGENCY` atau outcome ≠ `SUCCESS`. Selain itu opsional.
- Transisi invalid ditolak server-side dengan **409** dan status tidak berubah. Aksi tanpa hak ditolak dengan **403**.
- Double submit ditangani dengan guard `WHERE status_code = <from>` pada UPDATE (pola sama dengan incident). Request kedua mendapat 409, bukan duplikasi activity.
- Field CR (selain jadwal) hanya bisa diedit saat `DRAFT`.

## 8. Integrasi dengan Incident

### 8.1 Link Incident ↔ Change (`trans_change_incident`)

| Relation | Arti | Dibuat dari |
|---|---|---|
| `FIX_FOR` | Change ini mengimplementasikan fix untuk incident. | Tombol "Buat Change Request" di Incident Detail (pre-fill title, application, environment, link), atau manual dari Change Detail. |
| `CAUSED_BY` | Incident ini disebabkan oleh change. | Dari Incident Detail ("Tautkan ke change penyebab"), dibatasi pada change berstatus `IMPLEMENTING`/`REVIEWING`/`CLOSED`. |

- Satu incident bisa punya banyak change dan sebaliknya. Pasangan `(change_id, incident_id, relation)` unik.
- Link/unlink tercatat di **dua** timeline: activity change dan activity incident (tipe baru `change_link`).
- Incident Detail menampilkan panel **Linked Changes** (no, title, status, relation).
- Change Detail menampilkan panel **Linked Incidents**.

### 8.2 Gate fix production (blocking, Q3)

Incident di environment `production` **tidak boleh** berpindah `FIXING → VERIFYING` bila belum punya minimal satu change `FIX_FOR` dengan status `IMPLEMENTING`, `REVIEWING`, atau `CLOSED` (outcome bukan `FAILED`/`ROLLED_BACK`).

- Server menolak dengan **409** `CHANGE_REQUIRED`, status incident tidak berubah.
- UI Incident Detail menampilkan alasan + tombol "Buat Change Request" sebelum user mencoba transisi.
- Incident non-production tidak terkena gate.

### 8.3 Konteks triase

Incident Detail menampilkan "Recent changes" untuk aplikasi + environment yang sama dalam 72 jam terakhir (status `IMPLEMENTING`/`REVIEWING`/`CLOSED`). Tujuannya agar Help Desk/Analyst cepat curiga pada change terbaru.

## 9. Scheduling

- `planned_start` dan `planned_end` wajib saat schedule, dengan `planned_end > planned_start` dan `planned_start` tidak di masa lalu (toleransi 5 menit).
- **Conflict detection (warning, tidak blokir):** saat schedule, server mengembalikan `conflicts[]` berisi CR lain berstatus `SCHEDULED`/`IMPLEMENTING` dengan **application + environment sama** dan window yang overlap. UI menampilkannya sebelum konfirmasi.
- Reschedule (`SCHEDULED → SCHEDULED`) tercatat di timeline dengan jadwal lama dan baru.
- `actual_start` diisi saat start, `actual_end` saat complete. Keduanya diisi server, bukan dari client.

## 10. User Stories & Acceptance Criteria

**CM-01 — Create Change Request**
Sebagai Developer/DevOps/Analyst, saya ingin membuat CR agar perubahan terdokumentasi sebelum dikerjakan.
- Given role berhak, when field wajib valid, then CR dibuat dengan status `DRAFT` dan nomor unik `CHG-2026-000001`.
- Wajib saat simpan draft: title (min 5), type, risk, application, environment.
- Wajib sebelum submit: description, implementation_plan, rollback_plan; `test_plan` bila risk `HIGH`. Draft boleh disimpan belum lengkap.
- Bila dibuat dari incident, link `FIX_FOR` otomatis tersimpan.

**CM-02 — Submit**
- Requester submit `DRAFT`, status menjadi `SUBMITTED`, dan semua Manager/Lead aktif (kecuali requester) menerima notifikasi.
- Type `STANDARD` langsung `APPROVED` (auto-approve tercatat).

**CM-03 — Approve / Reject / Request changes**
- Manager/Lead (bukan requester), atau System Analyst untuk CR LOW/MEDIUM milik ManagerLead, memilih APPROVE, REJECT, atau REQUEST_CHANGES.
- REJECT dan REQUEST_CHANGES wajib reason.
- Keputusan tersimpan di `trans_change_approval` (approver, decision, reason, revision, timestamp), lalu requester menerima notifikasi.
- Requester yang mencoba approve CR sendiri mendapat 403 `SELF_APPROVAL_FORBIDDEN`.

**CM-04 — Schedule**
- CR `APPROVED` dijadwalkan dengan window valid, lalu status menjadi `SCHEDULED`. Response menyertakan `conflicts[]`.
- Implementer dan requester menerima notifikasi.

**CM-05 — Implement**
- Implementer klik Start: status `IMPLEMENTING` dan `actual_start` tercatat.
- Complete dengan outcome: status `REVIEWING` dan `actual_end` tercatat.
- Outcome `FAILED`/`ROLLED_BACK` wajib notes. Requester dan Manager menerima notifikasi "change failed".

**CM-06 — Review & Close**
- Analyst/Manager close CR `REVIEWING`. `closed_at` dan `closed_by` tercatat.
- PIR wajib untuk `EMERGENCY` atau outcome ≠ `SUCCESS`. Bila PIR kosong pada kondisi wajib, ditolak 400.

**CM-07 — Cancel**
- Requester atau Manager dapat cancel CR berstatus `DRAFT`/`SUBMITTED`/`APPROVED`/`SCHEDULED` dengan reason. Setelah `IMPLEMENTING`, cancel ditolak 409.

**CM-08 — Link incident**
- User berhak menautkan incident ke change dengan relation `FIX_FOR` atau `CAUSED_BY`. Kedua timeline mencatat aksi ini.
- Duplikat link ditolak 409.

**CM-09 — Timeline & komentar**
- Setiap status change, approval, reschedule, link, dan komentar tercatat dengan actor + timestamp, lalu ditampilkan berurutan tanpa query N+1.

## 11. Functional Requirements

| ID | Requirement | Priority |
|---|---|---|
| CM-FR-01 | Create/edit CR (DRAFT) dengan field wajib & nomor unik `CHG-YYYY-NNNNNN`. | Must |
| CM-FR-02 | List/search/filter CR (status, type, risk, application, environment, requester, implementer, tanggal jadwal) + pagination. | Must |
| CM-FR-03 | Controlled status transition server-side (§7). | Must |
| CM-FR-04 | Approval single-approver Manager/Lead + segregation of duties + riwayat per revision. | Must |
| CM-FR-05 | Scheduling + conflict warning. | Must |
| CM-FR-06 | Implementation outcome (SUCCESS/FAILED/ROLLED_BACK) + PIR. | Must |
| CM-FR-07 | Activity timeline + komentar change. | Must |
| CM-FR-08 | Link incident ↔ change (FIX_FOR / CAUSED_BY) + panel di kedua detail page. | Must |
| CM-FR-09 | Notifikasi in-app + email untuk event change (§13). | Should |
| CM-FR-10 | Dashboard change (§14). | Should |
| CM-FR-11 | Kalender/jadwal change (list per tanggal). | Should |
| CM-FR-12 | "Recent changes" di Incident Detail (§8.3). | Should |
| CM-FR-13 | Attachment pada CR (reuse aturan attachment incident). | Could |
| CM-FR-14 | Gate blocking FIXING→VERIFYING untuk incident production (§8.2). | Must (CM-2) |

## 12. Data Model (Goose migration baru `00004_change_management.sql`)

Mengikuti konvensi `master_` / `trans_`, code TEXT sebagai PK master, UUID untuk transaksi, `timestamptz`.

### Master

| Table | Isi seed |
|---|---|
| `master_change_type` | STANDARD, NORMAL, EMERGENCY |
| `master_change_risk` | LOW, MEDIUM, HIGH (`sort_order`) |
| `master_change_status` | DRAFT, SUBMITTED, APPROVED, SCHEDULED, IMPLEMENTING, REVIEWING, CLOSED, REJECTED, CANCELLED (`is_terminal`) |

### Transaksi

**`trans_change`**

| Field | Tipe | Catatan |
|---|---|---|
| id | UUID PK | |
| change_no | TEXT UNIQUE | `CHG-2026-000001` via sequence `change_no_seq` |
| title | TEXT | CHECK len ≥ 5 |
| description | TEXT | |
| justification | TEXT | alasan bisnis/teknis |
| type_code | FK master_change_type | |
| risk_code | FK master_change_risk | |
| status_code | FK master_change_status | default DRAFT |
| application_id | FK master_application | NOT NULL |
| environment_code | FK master_environment | NOT NULL |
| implementation_plan | TEXT | |
| rollback_plan | TEXT | |
| test_plan | TEXT | wajib bila HIGH (validasi service) |
| requester_id | FK master_user | |
| implementer_id | FK master_user | nullable |
| team_id | FK master_team | nullable |
| revision | SMALLINT | default 1 |
| planned_start / planned_end | TIMESTAMPTZ | nullable |
| actual_start / actual_end | TIMESTAMPTZ | nullable |
| outcome | TEXT | CHECK IN (SUCCESS, FAILED, ROLLED_BACK), nullable |
| outcome_notes | TEXT | |
| review_notes | TEXT | PIR |
| created_at / updated_at | TIMESTAMPTZ | |
| closed_at / closed_by | | |

Index: `status_code`, `type_code`, `risk_code`, `(application_id, environment_code, planned_start)` untuk conflict detection, `requester_id`, `implementer_id`, `created_at DESC`.

**`trans_change_approval`**: id, change_id, approver_id, decision (`APPROVED`/`REJECTED`/`CHANGES_REQUESTED`/`AUTO_APPROVED`), reason, revision, created_at. Index `(change_id, created_at)`.

**`trans_change_activity`**: struktur sama dengan `trans_incident_activity` (type, actor_id, from_status, to_status, payload JSONB). Index `(change_id, created_at)`.

**`trans_change_comment`**: id, change_id, author_id, body, created_at.

**`trans_change_incident`**: id, change_id, incident_id, relation (`FIX_FOR`/`CAUSED_BY`), created_by, created_at. UNIQUE `(change_id, incident_id, relation)`. Index `incident_id`.

**Perubahan tabel existing (via migrasi baru, tidak mengedit migrasi lama):**
- `trans_notification`: `incident_id` menjadi nullable dan ditambah `change_id UUID NULL REFERENCES trans_change`, dengan `CHECK (incident_id IS NOT NULL OR change_id IS NOT NULL)`. Response notifikasi ditambah field `change_id` (nullable). **Perubahan ini menyentuh contract `GET /api/notifications`** dan frontend `api/types.ts` ikut diupdate.
- `trans_incident_activity`: tipe activity baru `change_link`. Kolom tidak berubah, cukup payload `{change_id, change_no, relation, action}`.

## 13. Notifications

| Event | Penerima |
|---|---|
| CR submitted | Semua ManagerLead aktif kecuali requester |
| CR approved / rejected / changes requested | Requester (+ implementer bila ada) |
| CR scheduled / rescheduled | Requester, implementer |
| Implementation started | Requester, ManagerLead |
| Change failed / rolled back | Requester, implementer, ManagerLead, PIC incident yang ter-link `FIX_FOR` |
| CR closed / cancelled | Requester, implementer |
| Incident ditautkan `CAUSED_BY` | Requester & implementer change tersebut |

Channel dan mekanisme reuse yang sudah ada: in-app di `trans_notification` dan email async via `notify.Sender`. Actor tidak menerima notifikasi atas aksinya sendiri.

## 14. Dashboard & Reporting

Endpoint terpisah `GET /api/changes/summary` agar contract `/api/dashboard` tidak berubah.

- Total change aktif (non-terminal) by status.
- Change by type & risk.
- Upcoming changes (7 hari ke depan).
- **Change success rate** = CLOSED dengan outcome SUCCESS / total CLOSED.
- **Change failure rate** = (outcome FAILED/ROLLED_BACK **atau** punya link `CAUSED_BY`) / total CLOSED.
- **Emergency change ratio** = EMERGENCY / total.
- Average lead time Submit → Approve dan Approve → Implement.
- % CR dengan PIR terisi (untuk yang wajib PIR).

Ditampilkan sebagai section "Change" di halaman Dashboard. Seperti PRD MVP §14, jumlah change closed tidak boleh jadi satu-satunya indikator. Failure rate dan emergency ratio harus selalu ditampilkan berdampingan.

## 15. API Contract (tambahan)

Semua endpoint di bawah grup `RequireAuth`. Format error standar `{code, message, details, request_id}`.

```
GET    /api/changes?status=&type=&risk=&application_id=&environment=&requester=&implementer=&scheduled_from=&scheduled_to=&q=&page=&limit=
POST   /api/changes                         # create DRAFT (opsional incident_id → link FIX_FOR)
GET    /api/changes/:id
PATCH  /api/changes/:id                     # edit field, hanya DRAFT
POST   /api/changes/:id/submit
POST   /api/changes/:id/approval            # {decision: APPROVE|REJECT|REQUEST_CHANGES, reason}
GET    /api/changes/:id/approvals
POST   /api/changes/:id/schedule            # {planned_start, planned_end} → {change, conflicts[]}
POST   /api/changes/:id/start
POST   /api/changes/:id/complete            # {outcome, outcome_notes}
POST   /api/changes/:id/close               # {review_notes}
POST   /api/changes/:id/cancel              # {reason}
GET    /api/changes/:id/timeline
POST   /api/changes/:id/comments
GET    /api/changes/:id/comments
POST   /api/changes/:id/incidents           # {incident_id, relation}
DELETE /api/changes/:id/incidents/:incidentId?relation=
GET    /api/incidents/:id/changes           # linked changes untuk Incident Detail
GET    /api/incidents/:id/recent-changes    # §8.3 (Should)
GET    /api/changes/summary                 # §14 (Should)
GET    /api/meta                            # + change_types, change_risks, change_statuses (field tambahan, backward compatible)
```

Contoh body create:
```json
{
  "title": "Deploy hotfix checkout timeout",
  "description": "Naikkan timeout gateway & perbaiki retry.",
  "justification": "Fix INC-2026-000123 (S1).",
  "type": "EMERGENCY",
  "risk": "MEDIUM",
  "application_id": "…",
  "environment": "production",
  "implementation_plan": "1) merge PR #45 2) deploy via pipeline 3) smoke test",
  "rollback_plan": "Redeploy tag v1.8.2",
  "test_plan": "",
  "implementer_id": "…",
  "team_id": "…",
  "incident_id": "…"
}
```

Status code: 201 create, 200 ok, 400 validasi, 401, 403 (role/SoD), 404, 409 (transisi invalid, duplikat link, cancel setelah implementing).

## 16. UX / Pages

Mengikuti `frontend/UI-reference/DESIGN.md`: canvas chalk, kartu putih `rounded-lg` border mist, aksen iris `#4255ff`, tombol primer pill. Badge status/type/risk memakai warna semantik pastel.

| Page / Komponen | Fungsi |
|---|---|
| Sidebar: item **Changes** | Disembunyikan untuk role User. |
| `ChangeList` (`/changes`) | Search, filter, pagination. Kolom: no, title, type, risk, status, application/env, jadwal, requester. |
| `ChangeNew` (`/changes/new`, `?incident=<id>`) | Form CR. Pre-fill dari incident bila ada. Simpan sebagai Draft atau langsung Submit. |
| `ChangeDetail` (`/changes/:id`) | Header + action bar sesuai status & role. Tab: Detail/Plan, Approval, Linked Incidents, Timeline, Komentar. |
| `ChangeCalendar` (`/changes/calendar`) | Daftar change terjadwal dikelompokkan per hari (MVP tanpa library kalender). |
| `IncidentDetail` (update) | Panel Linked Changes, tombol "Buat Change Request", "Tautkan change penyebab", panel Recent Changes, warning gate §8.2. |
| `Dashboard` (update) | Section Change (§14). |
| Komponen | `ChangeStatusBadge`, `ChangeTypeBadge`, `RiskBadge`, `ChangeForm`, `ApprovalList`, `ScheduleDialog` (dengan tampilan conflicts), `CompleteDialog`. |

Required states sama dengan PRD MVP §9: loading/skeleton, empty + CTA "Buat Change Request" (bila berhak), success toast, error tanpa kehilangan input, retry tanpa double-submit (tombol disabled saat mutation pending).

## 17. Success Metrics

| Kategori | Metric |
|---|---|
| Leading | % deploy production yang punya CR; % CR dengan rollback plan; % incident S1/S2 yang fix-nya ter-link ke CR. |
| Outcome | Change success rate; change failure rate; lead time Submit→Approve. |
| Guardrail | Emergency change ratio (jangan sampai semua jadi emergency untuk melewati approval); % PIR terisi; CR stuck di SUBMITTED > 2 hari kerja. |

## 18. Milestones & Sequencing

| Fase | Estimasi | Scope |
|---|---|---|
| CM-1 | ± 1–2 minggu | Migrasi + master seed; create/edit/list/detail; submit; approve/reject/request changes; cancel; timeline + komentar; ChangeList/New/Detail. |
| CM-2 | ± 1 minggu | Schedule + conflict warning; start/complete/close + PIR; link incident (kedua arah) + panel di IncidentDetail; gate production §8.2; notifikasi. |
| CM-3 | ± 1 minggu | Summary dashboard; ChangeCalendar; Recent changes di incident; (opsional) attachment CR. |

Estimasi hanya untuk sequencing, bukan komitmen delivery.

## 19. Acceptance Test Matrix (untuk `go test`)

- Create valid → DRAFT + `change_no` unik. Field wajib kosong → 400. Risk HIGH tanpa test_plan → 400.
- Role User/QA create → 403.
- Edit selain DRAFT → 409.
- Submit NORMAL → SUBMITTED. Submit STANDARD → APPROVED + approval `AUTO_APPROVED`. STANDARD dengan risk ≠ LOW → 400.
- Approve oleh non-Manager → 403. Approve oleh requester → 403 `SELF_APPROVAL_FORBIDDEN`.
- Analyst approve CR LOW/MEDIUM milik Manager → 200; CR HIGH milik Manager atau CR milik non-Manager → 403.
- Incident production `FIXING → VERIFYING` tanpa change `FIX_FOR` aktif → 409 `CHANGE_REQUIRED`; dengan change IMPLEMENTING → 200.
- Reject tanpa reason → 400. Request changes → DRAFT + revision+1.
- Schedule dengan end ≤ start → 400. Schedule overlap → 200 + `conflicts` tidak kosong.
- EMERGENCY APPROVED → start langsung boleh. NORMAL APPROVED → start → 409.
- Complete FAILED tanpa notes → 400.
- Close EMERGENCY tanpa PIR → 400.
- Cancel IMPLEMENTING → 409.
- Double submit/approve/start → request kedua 409, activity tidak ganda.
- Link duplikat → 409. Link `CAUSED_BY` ke change DRAFT → 409.
- Timeline change & incident sama-sama mencatat link.
- Handler test (httptest): happy path + 400/403/404/409 untuk endpoint utama.

## 20. Keputusan (Q1–Q7)

| # | Topik | Keputusan |
|---|---|---|
| Q1 | Model approval | **Single approver** Manager/Lead untuk semua risk. |
| Q2 | Hanya satu Manager | System Analyst boleh approve CR **LOW/MEDIUM** yang requester-nya ManagerLead. |
| Q3 | Gate incident production | **Blocking** 409 `CHANGE_REQUIRED` (§8.2). |
| Q4 | Approval emergency | **Wajib approve dulu**, tanpa approval retroaktif. |
| Q5 | Penomoran | `CHG-YYYY-NNNNNN`, sequence global (sama dengan incident). |
| Q6 | Visibilitas | Semua role kecuali User melihat semua change. |
| Q7 | Notifikasi | **Perluas `trans_notification`**: `incident_id` nullable + `change_id`. |

**Risiko**
- Proses dianggap birokratis sehingga user melewati CR. Mitigasi: type STANDARD auto-approve, create dari incident dengan pre-fill, form ringkas.
- Emergency dipakai untuk melewati approval. Mitigasi: PIR wajib + metric emergency ratio.
- Perubahan `trans_notification` menyentuh fitur existing. Mitigasi: migrasi additive + test regresi notifikasi incident.

## 21. Coding-Agent Handoff

**Modul yang disentuh**
- `backend/migrations/00004_change_management.sql` (baru).
- `backend/internal/model/change.go` (baru): status, `AllowedChangeTransitions`, `CanChangeTransition`, struct DTO.
- `backend/internal/repository/changes.go` (baru): query + UPDATE dengan status guard + activity dalam satu tx.
- `backend/internal/service/changes.go` (baru): validasi, permission, SoD, conflict detection, notifikasi.
- `backend/internal/handler/changes.go` (baru) + registrasi route di `handler.go`.
- `service/notifications.go` + `repository/notifications.go`: dukungan `change_id`.
- `repository/masters.go` + `model.Meta`: master change di `/api/meta`.
- Frontend: `api/types.ts`, `api/client.ts`, pages `ChangeList`/`ChangeNew`/`ChangeDetail`/`ChangeCalendar`, komponen badge/form/dialog, update `IncidentDetail`, `Dashboard`, `Sidebar`, `App.tsx`, `Notifications` (link ke change).
- `AGENTS.md` §7 & §8: tambah endpoint dan tabel baru.

**Definition of Done**
- Semua requirement Must CM-1 + CM-2 terimplementasi.
- `go test ./...`, `go vet ./...`, `gofmt -l .` bersih; `npm run build` + `npm run lint` lolos.
- Transisi & permission tervalidasi server-side, ada unit test transisi + handler test.
- Audit trail lengkap (actor + timestamp) untuk status, approval, schedule, dan link.
- Notifikasi incident existing tidak regresi.
