# PRD — Monitoring / Observability Integration (Enhancement)

**Produk:** Incident Management System
**Versi dokumen:** 0.1 (DRAFT untuk review)
**Tanggal:** 2026-10-03
**Status:** Menunggu review & keputusan user (lihat §19)
**Referensi:** PRD MVP §1 (*incident dari sumber otomatis*), §11 (`master_incident_source`: System/Monitoring), §19 (*integration layer setelah core stabil*), §21
**Bergantung pada:** MVP. Lebih bernilai bila SLA (PRD_SLA_Escalation.md) sudah ada, karena alert P1 langsung masuk hitungan SLA.

---

## 1. TL;DR

Alert dari sistem monitoring (Prometheus Alertmanager, Grafana, Uptime Kuma, atau JSON generik) dikirim ke **webhook** aplikasi ini dan otomatis menjadi **incident**:
- Severity/priority/aplikasi/environment dipetakan dari label alert.
- Alert yang sama tidak membuat incident ganda (**deduplikasi** per fingerprint).
- Saat alert resolved, incident mendapat catatan otomatis. Penutupan tetap lewat alur verifikasi manusia.

## 2. Problem Statement

- Alert monitoring saat ini masuk ke email/chat dan harus disalin manual menjadi incident. Ada jeda waktu, dan kadang terlupa.
- Satu gangguan bisa memicu puluhan alert; tanpa deduplikasi, tim kebanjiran.
- Tidak ada data waktu dari alert pertama sampai incident ditangani (MTTD/MTTA).

## 3. Goals

- Incident tercipta < 1 menit setelah alert critical dikirim.
- Deduplikasi & storm protection agar tidak banjir incident.
- Konfigurasi integrasi aman (token per integrasi, rahasia tidak pernah tampil ulang).
- Jejak audit lengkap: event alert mentah tersimpan & terhubung ke incident.

## 4. Non-Goals (fase ini)

- Menjalankan/menyimpan metrik atau log sendiri (bukan pengganti Prometheus/Grafana).
- Auto-remediation (PRD MVP §4).
- Polling/pull ke sistem monitoring. Hanya push via webhook.
- Auto-resolve/auto-close incident (Q3).
- Korelasi lintas alert berbasis AI (lihat PRD_AI_Assist.md).

## 5. Users & Roles

| Role | Kebutuhan |
|---|---|
| DevOps | Mengonfigurasi integrasi & mapping, memantau event masuk. |
| Manager/Lead | Membuat/menonaktifkan integrasi (pemilik akses), melihat metrik. |
| Help Desk / PIC | Menangani incident hasil alert seperti incident biasa, melihat konteks alert. |

Kelola integrasi: ManagerLead dan DevOps. Read-only event log: role internal.

## 6. Konsep & Aturan

### 6.1 Integrasi
- Satu record per sumber: tipe `ALERTMANAGER`, `GRAFANA`, `GENERIC_JSON`.
- URL webhook: `POST /api/integrations/alerts/{integration_id}`.
- Autentikasi: header `Authorization: Bearer <token>`:
  - token acak 32 byte, ditampilkan **sekali** saat dibuat/rotasi;
  - disimpan sebagai hash SHA-256;
  - **bukan** JWT user.
- Opsional: allowlist IP/CIDR per integrasi.
- Batas body 1 MiB, rate limit per integrasi (default 60 req/menit).

### 6.2 Normalisasi
Setiap payload dinormalisasi menjadi daftar alert:
`{fingerprint, status: firing|resolved, title, description, labels{}, starts_at, ends_at, source_url}`.
- Alertmanager: `alerts[]`, `fingerprint`, `labels`, `annotations.summary/description`, `generatorURL`.
- Grafana: format unified alerting (mirip Alertmanager).
- Generic JSON: mapping JSONPath sederhana dikonfigurasi per integrasi (field title, fingerprint, status, severity).

### 6.3 Mapping ke incident (konfigurasi per integrasi)

| Field incident | Sumber default | Contoh |
|---|---|---|
| title | `annotations.summary` atau `alertname` | "HighErrorRate checkout" |
| description | `annotations.description` + daftar label | |
| severity | label `severity`: critical→S1, high/error→S2, warning→S3, info→S4 | configurable |
| priority | dari severity (S1→P1 …) atau label `priority` | configurable |
| application | label `service`/`app` dicocokkan ke `master_application.code` | fallback: aplikasi default integrasi |
| environment | label `env`/`environment` → `master_environment.code` | fallback: default integrasi |
| source | `monitoring` | tetap |
| reporter | `NULL`; activity actor = sistem, payload menyebut integrasi | |

Alert dengan severity di bawah ambang (default: `info`) hanya dicatat sebagai event, tanpa membuat incident (configurable).

### 6.4 Deduplikasi & lifecycle
- **Kunci dedup:** `(integration_id, fingerprint)`.
- **Firing pertama:** buat incident NEW + link `trans_incident_alert` (first_seen, last_seen, fire_count = 1). Notifikasi `created` berjalan seperti biasa.
- **Firing lagi** saat incident masih terbuka (bukan RESOLVED/CLOSED): `fire_count++`, `last_seen` diperbarui, activity `alert_refired`. Activity dibatasi maks 1 per 15 menit per fingerprint agar timeline tidak penuh.
- **Firing lagi** saat incident sudah RESOLVED/CLOSED:
  - dalam 24 jam: komentar otomatis + notifikasi PIC "alert muncul lagi". Reopen tetap keputusan koordinator (Q4);
  - di luar 24 jam: incident baru.
- **Resolved:** activity `alert_resolved` + komentar otomatis "Alert resolved pada …". Status incident **tidak** diubah (Q3).
- **Storm protection:** bila > N (default 10) fingerprint baru dari satu integrasi dalam 5 menit, alert berikutnya dikelompokkan ke satu "incident induk" per aplikasi, lalu ManagerLead diberi notifikasi `alert_storm`.

### 6.5 Keandalan
- Webhook merespons cepat:
  1. simpan event mentah (`trans_alert_event`) dalam transaksi;
  2. proses normalisasi/pembuatan incident di transaksi yang sama (volume kecil). Pemrosesan async bila volume naik (PRD MVP §15).
- Idempotensi: event dengan hash payload sama dalam 60 detik diabaikan (retry Alertmanager).
- Respons `202 Accepted` + ringkasan `{created: n, updated: n, ignored: n}`.

## 7. User Stories & Acceptance Criteria

- **MON-01 Buat integrasi.** Manager/DevOps membuat integrasi Alertmanager → URL + token tampil sekali. GET berikutnya hanya menampilkan 4 karakter terakhir.
- **MON-02 Alert critical.** POST payload Alertmanager firing `severity=critical, service=checkout, env=production` → incident NEW S1/P1, aplikasi Checkout, environment production, source monitoring, dalam satu request.
- **MON-03 Dedup.** Payload yang sama dikirim ulang 5x → tetap 1 incident, `fire_count` = 6, activity tidak membanjiri.
- **MON-04 Resolved.** Payload resolved → komentar + activity; status tetap.
- **MON-05 Token salah.** Token salah → 401; integrasi nonaktif → 403; body > 1 MiB → 413; rate limit terlewati → 429.
- **MON-06 Mapping gagal.** Label `service` tidak dikenal → pakai aplikasi default. Bila tidak ada default, incident tetap dibuat tanpa aplikasi dan event diberi flag `mapping_warning`.
- **MON-07 Konteks di incident.** Incident Detail menampilkan panel Alert: labels, fire count, first/last seen, tombol "Buka di monitoring" (`source_url`).
- **MON-08 Rotasi token.** Rotasi → token lama langsung ditolak.

## 8. Functional Requirements

| ID | Requirement | Priority |
|---|---|---|
| MON-FR-01 | Master integrasi + token hash + aktif/nonaktif + rotasi. | Must |
| MON-FR-02 | Webhook endpoint + autentikasi + limit ukuran + rate limit. | Must |
| MON-FR-03 | Normalisasi Alertmanager & Grafana. | Must |
| MON-FR-04 | Mapping label → severity/priority/app/env + default. | Must |
| MON-FR-05 | Dedup per fingerprint + link incident ↔ alert. | Must |
| MON-FR-06 | Event log mentah + halaman log per integrasi. | Must |
| MON-FR-07 | Panel Alert di Incident Detail. | Should |
| MON-FR-08 | Generic JSON mapping (JSONPath sederhana). | Should |
| MON-FR-09 | Storm protection + notifikasi `alert_storm`. | Should |
| MON-FR-10 | "Kirim test payload" dari UI. | Should |
| MON-FR-11 | Allowlist IP/CIDR. | Could |

## 9. Data Model (migrasi baru)

- `master_integration`:
  - id, type, name, token_hash, token_last4, is_active
  - default_application_id, default_environment_code, min_severity
  - mapping JSONB, ip_allowlist CIDR[]
  - created_by, created_at, rotated_at
- `trans_alert_event`:
  - id, integration_id, received_at, payload_hash, payload JSONB (dipangkas maks 64 KiB)
  - fingerprint, status, result (`created`/`updated`/`ignored`/`error`), error, incident_id
  - Index `(integration_id, received_at DESC)`, `(payload_hash, received_at)`.
  - Retensi 90 hari (job pembersih harian, Q6).
- `trans_incident_alert`:
  - incident_id, integration_id, fingerprint, labels JSONB, source_url
  - first_seen, last_seen, fire_count, last_status
  - UNIQUE `(integration_id, fingerprint, incident_id)`, index `(integration_id, fingerprint)`.
- Tipe activity incident baru: `alert_created`, `alert_refired`, `alert_resolved`. Tipe notifikasi baru: `alert_storm`.

## 10. API Contract (tambahan)

```
POST   /api/integrations/alerts/:integrationId     # webhook; auth Bearer token integrasi (bukan JWT)
GET    /api/master/integrations                    # Manager/DevOps
POST   /api/master/integrations                    # → {integration, webhook_url, token} (token sekali)
PATCH  /api/master/integrations/:id                # nama, mapping, default, aktif
POST   /api/master/integrations/:id/rotate-token   # → {token}
POST   /api/master/integrations/:id/test           # kirim payload contoh (dry-run, tidak membuat incident)
GET    /api/master/integrations/:id/events?status=&page=
GET    /api/incidents/:id/alerts
```

Route webhook berada **di luar** grup `RequireAuth` JWT, dengan middleware autentikasi token integrasi sendiri. CORS tidak diperlukan.

## 11. UX / Pages

| Page | Isi |
|---|---|
| Master Data → tab "Integrasi" | Daftar integrasi (status, event terakhir, jumlah 24 jam), tombol buat. |
| Dialog buat/rotasi | URL webhook + token (copy) + peringatan "hanya tampil sekali" + contoh konfigurasi Alertmanager `receivers.webhook_configs`. |
| Detail integrasi | Mapping (tabel label → nilai), default app/env, min severity, event log, "Kirim test". |
| Incident Detail | Panel Alert + badge "dari monitoring" di header. |
| Incident List | Ikon sumber monitoring + filter `source=monitoring` (sudah ada filter source di master). |

## 12. Keamanan

- Token tidak pernah di-log. Payload event disimpan, tetapi header `Authorization` tidak.
- Perbandingan token konstan-waktu (`subtle.ConstantTimeCompare` atas hash).
- Payload dari luar adalah **data**: tidak pernah dieksekusi/dirender sebagai HTML (React escape bawaan; Markdown tidak dipakai di panel Alert).
- Webhook tidak dapat membaca data apa pun. Respons hanya ringkasan hitungan.

## 13. Success Metrics

| Kategori | Metric |
|---|---|
| Outcome | MTTD (alert pertama → incident dibuat) < 1 menit p95; MTTA incident monitoring. |
| Leading | % incident S1/S2 berasal dari monitoring; dedup ratio (event/incident). |
| Guardrail | Event `error`/`mapping_warning`; jumlah storm; incident monitoring yang ditutup "bukan masalah" (noise). |

## 14. Milestones

| Fase | Estimasi | Scope |
|---|---|---|
| MON-1 | ± 1 minggu | Master integrasi + token, webhook Alertmanager, mapping label default, dedup, event log, test di Docker dengan Alertmanager lokal. |
| MON-2 | ± 1 minggu | Grafana + Generic JSON, UI integrasi (mapping, test payload), panel Alert di incident, storm protection. |
| MON-3 | ± 0,5 minggu | Allowlist IP, retensi event, metrik di dashboard. |

## 15. Acceptance Test Matrix (ringkas)

- Token benar/salah/integrasi nonaktif → 202/401/403; body 2 MiB → 413; burst > limit → 429.
- Firing → 1 incident; firing ulang ×5 → tetap 1, `fire_count` 6; resolved → komentar, status tetap.
- Firing setelah incident CLOSED > 24 jam → incident baru; < 24 jam → komentar + notif PIC.
- Mapping: `severity=critical` → S1/P1; label app tak dikenal → default; tanpa default → event flag warning.
- Retry identik dalam 60 detik → `ignored`.
- Storm: 15 fingerprint baru dalam 5 menit → ≤ 10 incident + 1 induk + notifikasi storm.

## 16. Risiko

- **Banjir incident dari alert berisik:** min severity, dedup, storm protection, metrik noise.
- **Token bocor:** rotasi satu klik, allowlist IP, event log untuk forensik.
- **Ketergantungan format vendor:** normalisasi berlapis; generic JSON sebagai jalan keluar.

## 17. Coding-Agent Handoff (ringkas)

- Paket `internal/integration` (normalizer per tipe + mapper, unit test murni dengan fixture payload).
- Middleware `IntegrationAuth`.
- Service membuat incident lewat jalur yang sama dengan `CreateIncident`, agar nomor, activity, notifikasi, dan SLA konsisten.

## 18. Dependensi

Tidak ada dependensi baru. Rate limit memakai implementasi token bucket sederhana in-memory. Bila multi-replica, rate limit per instance (Q5).

## 19. Keputusan yang Dibutuhkan

| # | Pertanyaan | Usulan default |
|---|---|---|
| Q1 | Sistem monitoring yang dipakai perusahaan? | Prioritaskan Alertmanager + Grafana; sebutkan bila ada yang lain (Zabbix, Datadog, Uptime Kuma, New Relic). |
| Q2 | Mapping severity label → S1–S4 & priority? | critical→S1/P1, high→S2/P2, warning→S3/P3, info→hanya event. |
| Q3 | Alert resolved boleh mengubah status incident otomatis? | Tidak; hanya komentar. Verifikasi tetap manusia (PRD MVP US-05). |
| Q4 | Alert muncul lagi setelah incident CLOSED: reopen otomatis? | Tidak; < 24 jam komentar + notif, > 24 jam incident baru. |
| Q5 | API dijalankan multi-replica? | Asumsi 1 replica; bila multi, rate limit dipindah ke DB/Redis (dependensi baru). |
| Q6 | Retensi event mentah? | 90 hari. |
