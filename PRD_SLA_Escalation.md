# PRD — SLA & Escalation (Enhancement)

**Produk:** Incident Management System
**Versi dokumen:** 0.2 (keputusan Q1–Q6: default disetujui)
**Tanggal:** 2026-10-03
**Status:** Disetujui — SLA-1, SLA-2, SLA-3 terimplementasi (kecuali SLA-06 recalc priority, SLA-FR-10 export CSV)
**Referensi:** PRD MVP §4 (Non-Goals: *Advanced SLA*), §19 (*struktur data jangan menghambat SLA*), §21 (*SLA and escalation*)
**Bergantung pada:** MVP (selesai). Tidak bergantung pada PRD enhancement lain.

---

## 1. TL;DR

Setiap incident mendapat **target waktu** berdasarkan priority:
- **Response**: kapan incident harus sudah di-assign.
- **Resolution**: kapan incident harus sudah RESOLVED.

Sistem menghitung mundur (24x7 atau jam kerja), memberi **peringatan** saat 75% waktu terpakai, menandai **breach** saat lewat target, dan **mengeskalasi** ke Manager/Lead lewat notifikasi. Dashboard menampilkan **SLA compliance** per priority.

## 2. Problem Statement

Saat ini priority (P1–P4) hanya label: tidak ada janji waktu, tidak ada pengingat, dan tidak ada data apakah tim memenuhi target. Akibatnya:
- Incident P1 bisa "diam" di status NEW tanpa ada yang sadar.
- Manager baru tahu ada keterlambatan setelah user komplain.
- Tidak ada angka MTTA/MTTR per priority untuk evaluasi kinerja tim.

## 3. Goals

- Target response & resolution yang jelas dan bisa dikonfigurasi per priority.
- Peringatan dini **sebelum** breach, bukan sesudah.
- Eskalasi otomatis ke penanggung jawab yang tepat.
- Laporan SLA compliance yang dapat dipercaya (dihitung server-side, terekam historinya).

## 4. Non-Goals (fase ini)

- On-call scheduling / rotasi jaga (PagerDuty-like).
- Eskalasi fungsional otomatis (auto-reassign ke tim lain).
- SLA per customer/kontrak (OLA/UC, multi-tier customer).
- Status baru "Pending Customer" / pause clock (lihat Q3).
- Penalti/kredit SLA finansial.

## 5. Users & Roles

| Role | Kebutuhan |
|---|---|
| Help Desk | Melihat incident yang hampir breach untuk segera di-assign. |
| PIC (Dev/DevOps/Analyst) | Melihat sisa waktu di incident miliknya. |
| Manager/Lead | Menerima eskalasi; mengelola kebijakan SLA, jam kerja, dan hari libur; melihat laporan compliance. |
| User/Customer | Melihat target penyelesaian incident miliknya (read-only, tanpa detail eskalasi). |

## 6. Konsep & Aturan

### 6.1 Metrik SLA

| Metrik | Mulai | Berhenti (met) | Default target (usulan) |
|---|---|---|---|
| **Response** | `created_at` | Assignment pertama (status → ASSIGNED). Lihat Q2. | P1 15 mnt · P2 1 jam · P3 4 jam kerja · P4 1 hari kerja |
| **Resolution** | `created_at` | Status → RESOLVED | P1 4 jam · P2 8 jam · P3 3 hari kerja · P4 5 hari kerja |

Angka default hanya usulan. Final mapping ada di Q1 dan disepakati business/IT ops (PRD MVP §19).

### 6.2 Kalender

- Tiap policy memakai kalender `24x7` atau `BUSINESS_HOURS`.
- Default jam kerja: Senin–Jumat 08:00–17:00 Asia/Jakarta. Hari libur nasional dikelola di Master Data.
- Default: P1/P2 memakai 24x7, P3/P4 memakai jam kerja.
- `target_at` dihitung server-side dengan menambahkan menit kerja pada kalender tersebut.

### 6.3 Siklus hidup SLA instance (`trans_incident_sla`)

```
RUNNING ──(stop event sebelum target)──▶ MET
   │
   ├─(now ≥ warn_at)──▶ tetap RUNNING + warned_at diisi (notifikasi WARNING)
   └─(now ≥ target_at)──▶ BREACHED (notifikasi BREACH) ──(stop event)──▶ tetap BREACHED + stopped_at
```

- **Satu incident punya dua instance per siklus:** RESPONSE dan RESOLUTION.
- **Priority berubah saat RUNNING:**
  - Target dihitung ulang dari `created_at` dengan policy baru.
  - Kalau target baru sudah lewat, langsung BREACHED.
  - Perubahan tercatat di timeline incident (`sla_recalculated`).
- **Reopen (FR-12):**
  - Instance lama tetap apa adanya (MET/BREACHED) sebagai histori.
  - Dibuat instance RESOLUTION baru, `cycle = n+1`, dihitung dari waktu reopen. Lihat Q4.
- **Incident CLOSED tanpa pernah RESOLVED** (secara aturan tidak mungkin): instance yang masih berjalan menjadi `CANCELLED`.
- Snapshot policy (menit target + kalender) disimpan di instance, supaya perubahan policy tidak mengubah histori.
- Incident yang dibuat **sebelum** SLA aktif tidak mendapat instance (tanpa backfill), agar data lama tidak memicu banjir breach. UI menampilkan "SLA tidak berlaku".
- 1 hari kerja = 9 jam (08:00–17:00). Target P3/P4 disimpan dalam menit kerja: P3 = 240 / 1620, P4 = 540 / 2700.

### 6.4 Eskalasi

| Level | Pemicu | Penerima | Kanal |
|---|---|---|---|
| L1 — Warning | 75% waktu target terpakai (configurable per policy) | PIC (atau koordinator bila belum ada PIC) | In-app + email |
| L2 — Breach | `now ≥ target_at` | PIC + semua ManagerLead aktif | In-app + email |
| L3 — Breach berlanjut (opsional) | Breach + X jam (default P1: 1 jam, P2: 4 jam) | ManagerLead (pengingat ulang, maks 1x) | In-app + email |

- Setiap eskalasi tercatat di `trans_incident_escalation` dan di timeline incident (`sla_warning`, `sla_breached`, `sla_escalated`).
- Idempotent: satu level hanya dikirim sekali per instance.

### 6.5 Engine (worker)

- Goroutine di proses API, berjalan tiap 60 detik (configurable `SLA_TICK_SECONDS`).
- Query: instance `RUNNING` dengan `next_check_at <= now()` diambil dengan `FOR UPDATE SKIP LOCKED`. Aman bila API dijalankan lebih dari satu replica.
- Stop event (assign, resolve) diproses **sinkron** di transaksi yang sama dengan perubahan status, sehingga MET tidak bergantung pada tick.
- Worker tidak pernah mengubah status incident. Ia hanya menandai SLA dan mengirim notifikasi.

## 7. User Stories & Acceptance Criteria

**SLA-01 — Target otomatis saat create.** Given policy P1 = 15/240 menit 24x7, when incident P1 dibuat, then dua instance RUNNING dengan `target_at` = created + 15 mnt dan created + 4 jam.

**SLA-02 — Response met.** When incident di-assign sebelum target, then instance RESPONSE menjadi MET dengan `stopped_at`, dan durasi tercatat.

**SLA-03 — Warning.** When 75% waktu terpakai dan belum met, then PIC/koordinator menerima notifikasi `sla_warning` sekali.

**SLA-04 — Breach.** When target terlewati, then status menjadi BREACHED, PIC + ManagerLead menerima `sla_breached`, dan list incident menampilkan badge merah.

**SLA-05 — Jam kerja.**
- Incident P3 dibuat Jumat 16:00 dengan target 4 jam kerja → `target_at` Senin 11:00.
- Hari libur dilewati.

**SLA-06 — Ubah priority.** *(Ditunda: aplikasi belum punya endpoint ubah priority; recalc dibuat bersamaan dengan fitur tersebut.)* P3 → P1 saat RUNNING → target dihitung ulang. Bila sudah lewat, langsung BREACHED + notifikasi.

**SLA-07 — Reopen.** Instance RESOLUTION lama tetap MET; instance baru `cycle 2` RUNNING dari waktu reopen.

**SLA-08 — Kelola policy.**
- ManagerLead mengubah target P2.
- Instance yang sedang berjalan **tidak** berubah, karena memakai snapshot. Lihat Q5.
- Incident baru memakai target baru.

**SLA-09 — Laporan.** Dashboard menampilkan % met response & resolution per priority untuk periode terpilih.

## 8. Functional Requirements

| ID | Requirement | Priority |
|---|---|---|
| SLA-FR-01 | Master SLA policy per priority (response & resolution menit, kalender, warn %). | Must |
| SLA-FR-02 | Instance SLA per incident + perhitungan `target_at` server-side. | Must |
| SLA-FR-03 | Stop event sinkron (assign → response met, resolved → resolution met). | Must |
| SLA-FR-04 | Worker warning/breach + notifikasi + audit eskalasi. | Must |
| SLA-FR-05 | Kalender jam kerja + hari libur. | Must |
| SLA-FR-06 | Badge/countdown SLA di list & detail + filter `sla=at_risk/breached`. | Must |
| SLA-FR-07 | Recalc saat priority berubah; instance baru saat reopen. | Must |
| SLA-FR-08 | Dashboard compliance + MTTA/MTTR per priority. | Should |
| SLA-FR-09 | Eskalasi L3 (pengingat breach berlanjut). | Could |
| SLA-FR-10 | Export CSV laporan SLA. | Could |

## 9. Data Model (migrasi baru `000N_sla.sql`, nomor ditetapkan saat eksekusi)

**Master**
- `master_business_calendar` (code PK: `24x7`, `BUSINESS_HOURS`; name; timezone)
- `master_business_hours` (calendar_code, weekday 0–6, start_time, end_time)
- `master_holiday` (id, calendar_code, date, name), UNIQUE `(calendar_code, date)`
- `master_sla_policy`:
  - priority_code PK (FK master_incident_priority)
  - response_minutes, resolution_minutes
  - calendar_code
  - warn_percent (default 75)
  - breach_reminder_minutes (nullable)
  - updated_by, updated_at
  - Seed sesuai §6.1.

**Transaksi**
- `trans_incident_sla`:
  - id, incident_id, metric (`RESPONSE`/`RESOLUTION`), cycle
  - status (`RUNNING`/`MET`/`BREACHED`/`CANCELLED`)
  - started_at, target_at, warn_at, next_check_at
  - warned_at, breached_at, stopped_at
  - policy snapshot: priority_code, target_minutes, calendar_code
  - Index `(status, next_check_at)`, `(incident_id)`.
  - UNIQUE `(incident_id, metric, cycle)` untuk mencegah instance ganda.
- `trans_incident_escalation`: id, incident_id, sla_id, level (`WARNING`/`BREACH`/`REMINDER`), recipients (UUID[]), created_at. UNIQUE `(sla_id, level)` untuk idempotensi.
- `trans_notification`: tipe baru `sla_warning`, `sla_breached`, `sla_reminder`. Tidak perlu perubahan skema.

## 10. API Contract (tambahan)

```
GET    /api/incidents/:id/sla                 # instance SLA semua siklus
GET    /api/incidents?sla=at_risk|breached    # filter baru (backward compatible)
GET    /api/master/sla-policies               # semua role internal (read)
PUT    /api/master/sla-policies/:priority     # ManagerLead
GET    /api/master/holidays?year=             # semua role internal
POST   /api/master/holidays                   # ManagerLead
DELETE /api/master/holidays/:id               # ManagerLead
GET    /api/master/business-hours             # read
PUT    /api/master/business-hours/:calendar   # ManagerLead
GET    /api/dashboard/sla?from=&to=           # compliance, MTTA, MTTR per priority
```

Response `GET /api/incidents` menambah field ringkas `sla` (status + target_at terdekat) per item. Ini **perubahan contract** yang backward compatible, dan frontend `api/types.ts` ikut diupdate.

## 11. UX / Pages

| Lokasi | Perubahan |
|---|---|
| Incident List | Kolom/badge SLA: hijau (on track), kuning (≥ warn %), merah (breached), abu (met). Countdown "sisa 42 mnt". Filter "Hampir breach" & "Breach". |
| Incident Detail | Panel SLA: Response & Resolution (target, sisa/terlambat, status), riwayat siklus & eskalasi. |
| Dashboard | Kartu compliance response/resolution, tabel per priority, daftar breach aktif. |
| Master Data | Tab "SLA": edit target per priority, kalender jam kerja, daftar hari libur (ManagerLead). |
| User/Customer | Hanya melihat "Target penyelesaian: …" di incident miliknya. |

Visual mengikuti DESIGN.md. Warna status SLA memakai warna semantik pastel, sama seperti badge severity.

## 12. Notifications

| Event | Penerima |
|---|---|
| `sla_warning` | PIC, atau koordinator (HelpDesk/Analyst) bila belum ada PIC |
| `sla_breached` | PIC + ManagerLead aktif |
| `sla_reminder` (L3) | ManagerLead aktif |

Memakai mekanisme `trans_notification` + email async yang sudah ada.

## 13. Success Metrics

| Kategori | Metric |
|---|---|
| Outcome | % response met, % resolution met per priority; MTTA; MTTR. |
| Leading | % incident yang di-assign sebelum warning. |
| Guardrail | Jumlah breach tanpa tindak lanjut > 24 jam; notifikasi SLA gagal kirim; drift worker (lag tick > 2 menit). |

## 14. Milestones

| Fase | Estimasi | Scope |
|---|---|---|
| SLA-1 | ± 1 minggu | Master policy (24x7 dulu), instance + stop event sinkron, badge di list/detail, API read. |
| SLA-2 | ± 1 minggu | Kalender jam kerja + libur, worker warning/breach/eskalasi, notifikasi, recalc priority, reopen. |
| SLA-3 | ± 0,5 minggu | Dashboard compliance, filter, Master Data UI, (opsional) export CSV & L3. |

## 15. Acceptance Test Matrix

- Kalkulator jam kerja (unit test murni):
  - lintas akhir pekan
  - lintas hari libur
  - mulai di luar jam kerja
  - target tepat di batas jam
- Create P1 → 2 instance RUNNING dengan target benar.
- Assign sebelum target → RESPONSE MET; assign sesudah → tetap BREACHED + stopped_at.
- Worker:
  - warning dikirim sekali (dua tick berturut tidak menggandakan);
  - breach mengirim notifikasi ke PIC + Manager;
  - dua worker paralel tidak menggandakan eskalasi (SKIP LOCKED + UNIQUE).
- Ubah priority → recalc; reopen → cycle 2.
- PUT policy oleh non-Manager → 403; nilai ≤ 0 → 400.
- Filter `sla=breached` hanya mengembalikan yang breach.

## 16. Risiko

- **Jam server/zona waktu:** semua perhitungan memakai UTC + timezone kalender eksplisit. Hindari `time.Local`.
- **Worker mati:** stop event tetap sinkron sehingga MET akurat; hanya notifikasi yang terlambat. Health endpoint menampilkan waktu tick terakhir.
- **Kelelahan notifikasi:** idempotensi per level, L3 maksimal 1x.

## 17. Coding-Agent Handoff (ringkas)

- Paket baru `internal/sla` untuk kalkulator jam kerja (pure function, unit test).
- Service `sla.go` + worker di `cmd/api/main.go`, dengan stop/start lewat context.
- Hook pada `CreateIncident`, `AssignIncident`, `UpdateStatus`/`AddVerification` (RESOLVED), `Reopen`, dan perubahan priority.

## 18. Dependensi

Tidak ada dependensi Go/npm baru.

## 19. Keputusan (default disetujui user, 2026-10-03)

| # | Pertanyaan | Usulan default |
|---|---|---|
| Q1 | Angka target per priority & kalender (§6.1)? | Tabel §6.1; P1/P2 24x7, P3/P4 jam kerja. |
| Q2 | Definisi "response" = assignment pertama, atau komentar/aksi pertama oleh PIC? | Assignment pertama (sudah ada datanya, objektif). |
| Q3 | Perlu status "Pending Customer" yang menghentikan jam SLA? | Tidak untuk fase ini (menambah status = mengubah lifecycle PRD MVP). |
| Q4 | Reopen: SLA resolution baru dari waktu reopen, atau melanjutkan sisa waktu lama? | Instance baru dari waktu reopen; reopen rate dipantau terpisah. |
| Q5 | Perubahan policy berlaku ke incident yang sedang berjalan? | Tidak (snapshot). Hanya incident baru. |
| Q6 | Siapa yang menerima eskalasi selain ManagerLead? Perlu "team lead" per team (`master_team.lead_user_id`)? | Fase ini: ManagerLead saja. Team lead opsional di fase berikut. |
