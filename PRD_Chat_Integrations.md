# PRD — Slack / Microsoft Teams / WhatsApp Integrations (Enhancement)

**Produk:** Incident Management System
**Versi dokumen:** 0.1 (DRAFT untuk review)
**Tanggal:** 2026-10-03
**Status:** Menunggu review & keputusan user (lihat §19)
**Referensi:** PRD MVP §13 (*WhatsApp sebagai notification engine = pengembangan berikutnya*), §1 (*laporan dari Email/WhatsApp*), §21
**Bergantung pada:** Notifikasi MVP (sudah ada). Lebih bernilai bila SLA ada, karena event breach dikirim ke channel.

---

## 1. TL;DR

Membawa incident ke tempat tim bekerja sehari-hari, dalam tiga tahap:
1. **CHAT-1 — Notifikasi ke channel** Slack/Teams via incoming webhook. Contoh: "🔴 INC-2026-000123 S1 Checkout gagal — assigned ke Developer". Tanpa instalasi bot, paling cepat bernilai.
2. **CHAT-2 — Slack interaktif:** tombol *Acknowledge / Assign ke saya / Buka* di pesan + slash command `/incident` untuk membuat incident dari Slack. Membutuhkan Slack App.
3. **CHAT-3 — WhatsApp** (WhatsApp Business Platform):
   - notifikasi personal ke PIC untuk P1/P2 (template disetujui Meta);
   - **intake**: pesan masuk ke nomor Help Desk menjadi draft incident di antrean triase.

## 2. Problem Statement

- Notifikasi saat ini hanya in-app + email. Untuk P1, email terlalu lambat dibaca.
- Diskusi incident terjadi di grup chat dan tidak terhubung ke incident.
- Laporan via WhatsApp (PRD MVP §1) masih disalin manual oleh Help Desk.

## 3. Goals

- Event penting incident sampai ke channel tim dalam < 30 detik.
- Mengurangi waktu acknowledge P1 lewat aksi langsung dari chat (CHAT-2).
- Laporan WhatsApp tercatat sebagai incident tanpa salin manual (CHAT-3).
- Aman: rahasia integrasi terenkripsi, data sensitif bisa disamarkan.

## 4. Non-Goals (fase ini)

- Sinkronisasi dua arah percakapan penuh (thread chat ↔ komentar incident). Hanya tautan.
- Bot AI di chat (lihat PRD_AI_Assist.md).
- Panggilan telepon/SMS on-call.
- Teams interaktif (bot Teams/Adaptive Card actions). Teams hanya outbound di fase ini (Q1).

## 5. Users & Roles

| Role | Kebutuhan |
|---|---|
| Manager/Lead, DevOps | Mengonfigurasi channel & aturan routing. |
| Semua role internal | Menerima notifikasi di channel; (CHAT-2) aksi cepat dari Slack. |
| Help Desk | (CHAT-3) Mengelola antrean intake WhatsApp → incident. |
| User/Customer | (CHAT-3) Melapor via WhatsApp dan menerima nomor incident. |

## 6. Konsep & Aturan

### 6.1 Channel & routing (CHAT-1)
- `master_chat_channel`:
  - provider: `SLACK_WEBHOOK` / `TEAMS_WEBHOOK`
  - nama, webhook URL (terenkripsi), aktif
- **Aturan routing per channel:**
  - event yang dikirim (subset tipe notifikasi: created, assigned, status_changed, verification_failed, resolved, closed, `sla_*`, `change_*`, `alert_storm`);
  - filter: severity minimum, priority minimum, aplikasi, team.
- Satu event bisa dikirim ke beberapa channel. Satu channel menerima satu pesan per event (dedup).
- **Isi pesan:**
  - nomor, judul, severity/priority, status, aplikasi, PIC, link ke aplikasi;
  - **deskripsi tidak dikirim** secara default (Q3, data sensitif);
  - format: Slack Block Kit / Teams Adaptive Card sederhana.

### 6.2 Pengiriman
- Reuse pola `notify.Sender` (async queue) dengan worker terpisah per provider.
- Retry eksponensial: 3x (1s, 10s, 60s). Status per pengiriman di `trans_chat_delivery`.
- 4xx permanen (webhook dicabut) → channel ditandai `ERROR` + notifikasi ke pembuatnya.
- Timeout 10 detik per request. Tidak pernah memblokir aksi incident.

### 6.3 Slack interaktif (CHAT-2)
- Slack App milik perusahaan dengan scope minimal: `chat:write`, `commands`, `users:read.email`.
- Verifikasi setiap request Slack dengan **signing secret** (HMAC + timestamp ≤ 5 menit).
- **Pemetaan user:** email Slack ↔ `master_user.email`. Aksi hanya untuk user yang terpetakan dan berhak sesuai aturan aplikasi (role & ownership sama seperti di UI).
- **Aksi:**
  - `Acknowledge` = assign PIC ke diri sendiri bila koordinator/eligible;
  - `Buka di aplikasi`;
  - `/incident <judul>` membuka modal (severity, priority, aplikasi) → create incident.
- Pesan di-update (bukan diposting ulang) saat status incident berubah. Simpan `channel_id` + `ts` di `trans_chat_message`.

### 6.4 WhatsApp (CHAT-3)
- Melalui **WhatsApp Business Platform (Cloud API Meta)** atau BSP lokal (Q4). Membutuhkan:
  - nomor bisnis terverifikasi;
  - template pesan disetujui Meta untuk pesan keluar di luar jendela 24 jam;
  - biaya per percakapan.
- **Notifikasi personal:**
  - hanya ke user yang **opt-in** dan mengisi nomor (`master_user.phone_e164`, `whatsapp_opt_in_at`);
  - hanya event P1/P2 assigned / sla_breached (configurable);
  - template berisi nomor incident + link, tanpa deskripsi.
- **Intake:**
  - webhook pesan masuk (verifikasi signature `X-Hub-Signature-256`) → `trans_chat_intake` (nomor pengirim, teks, media) → antrean "Intake WhatsApp" untuk Help Desk;
  - Help Desk mengonversi ke incident (pre-fill, source `whatsapp`) atau menolak sebagai spam/duplikat;
  - balasan otomatis ke pelapor: "Laporan diterima, nomor INC-…" (dalam jendela 24 jam, tanpa template);
  - pengirim dicocokkan ke `master_user.phone_e164` bila ada (reporter); selain itu reporter kosong + nomor disimpan di intake.
- Media (foto) dari intake disimpan sebagai attachment incident lewat aturan FR-09.

## 7. User Stories & Acceptance Criteria

- **CHAT-01.** Manager menambah channel Slack dengan webhook URL → URL tersimpan terenkripsi dan tidak pernah dikembalikan API (hanya `…/XXXX`). "Kirim pesan uji" berhasil.
- **CHAT-02.** Incident S1 dibuat → channel dengan filter `min_severity=S2` menerima satu pesan < 30 detik. Channel dengan filter aplikasi lain tidak menerima.
- **CHAT-03.** Webhook mengembalikan 404 → 3 retry → channel `ERROR` + notifikasi in-app ke pembuat.
- **CHAT-04 (Slack).** Klik "Assign ke saya" oleh user Slack yang terpetakan dan berhak → incident ASSIGNED + pesan ter-update. User tak terpetakan → pesan ephemeral "akun tidak terhubung".
- **CHAT-05 (Slack).** Request tanpa signature valid → 401; timestamp > 5 menit → 401 (anti-replay).
- **CHAT-06 (WA).** User opt-in + P1 assigned → template WA terkirim, status delivery tercatat. User tanpa opt-in → tidak dikirim.
- **CHAT-07 (WA intake).** Pesan masuk → item antrean. Help Desk "Jadikan incident" → incident NEW source `whatsapp` + balasan nomor ke pelapor.

## 8. Functional Requirements

| ID | Requirement | Priority |
|---|---|---|
| CHAT-FR-01 | Master channel Slack/Teams webhook + enkripsi rahasia + uji kirim. | Must (CHAT-1) |
| CHAT-FR-02 | Aturan routing event & filter severity/priority/app/team. | Must (CHAT-1) |
| CHAT-FR-03 | Pengiriman async + retry + log delivery + status channel. | Must (CHAT-1) |
| CHAT-FR-04 | Format pesan Slack Block Kit & Teams Adaptive Card, tanpa deskripsi default. | Must (CHAT-1) |
| CHAT-FR-05 | Slack App: signing secret, pemetaan user via email, tombol aksi, update pesan. | Should (CHAT-2) |
| CHAT-FR-06 | Slash command `/incident` + modal create. | Should (CHAT-2) |
| CHAT-FR-07 | WA notifikasi personal opt-in via template. | Could (CHAT-3) |
| CHAT-FR-08 | WA intake → antrean Help Desk → incident + balasan nomor. | Could (CHAT-3) |

## 9. Data Model (migrasi baru)

- `master_chat_channel`:
  - id, provider, name, webhook_url_enc, webhook_last4, status (`ACTIVE`/`ERROR`/`DISABLED`)
  - event_types TEXT[], min_severity, min_priority, application_ids UUID[], team_ids UUID[]
  - include_description bool (default false)
  - created_by, created_at
- `trans_chat_delivery`: id, channel_id, notification_type, incident_id/change_id, attempt, status, http_status, error, created_at.
- `trans_chat_message` (CHAT-2): provider, channel_ref, message_ref (Slack `ts`), incident_id.
- `master_user`: tambah `slack_user_id` (cache), `phone_e164`, `whatsapp_opt_in_at` (CHAT-3).
- `trans_chat_intake` (CHAT-3): id, provider, sender_phone, sender_user_id, text, media JSONB, status (`NEW`/`CONVERTED`/`REJECTED`), incident_id, handled_by, created_at.
- Rahasia dienkripsi AES-256-GCM dengan kunci dari env `INTEGRATION_ENC_KEY`. Validasi saat startup (fail fast) bila fitur aktif.

## 10. API Contract (tambahan)

```
GET    /api/master/chat-channels                 # Manager/DevOps
POST   /api/master/chat-channels                 # {provider, name, webhook_url, rules...}
PATCH  /api/master/chat-channels/:id
POST   /api/master/chat-channels/:id/test
GET    /api/master/chat-channels/:id/deliveries
# CHAT-2 (di luar JWT, diverifikasi signing secret):
POST   /api/integrations/slack/interactions
POST   /api/integrations/slack/commands
# CHAT-3 (di luar JWT, diverifikasi signature Meta):
GET    /api/integrations/whatsapp/webhook        # verifikasi hub.challenge
POST   /api/integrations/whatsapp/webhook
GET    /api/intake?status=                       # Help Desk
POST   /api/intake/:id/convert                   # → incident
POST   /api/intake/:id/reject                    # {reason}
PATCH  /api/me/whatsapp                          # {phone_e164, opt_in}
```

## 11. UX / Pages

| Page | Isi |
|---|---|
| Master Data → "Chat & Notifikasi" | Daftar channel, status, pesan terakhir; form tambah (provider, URL, event, filter); tombol uji; log delivery. |
| Profil saya (CHAT-3) | Nomor WhatsApp + opt-in eksplisit (checkbox persetujuan). |
| Intake WhatsApp (CHAT-3) | Antrean Help Desk: pesan, pengirim, media; aksi Jadikan incident / Tolak. |

## 12. Keamanan & Privasi

- Webhook URL Slack/Teams adalah rahasia: dienkripsi, tidak pernah dikembalikan/di-log.
- Deskripsi incident tidak dikirim default (bisa diaktifkan per channel internal, Q3).
- Slack/WA endpoint memverifikasi signature dan menolak replay.
- Opt-in WhatsApp eksplisit dan bisa dicabut kapan saja. Nomor telepon adalah data pribadi; akses dibatasi.
- Aksi dari Slack tunduk pada otorisasi yang sama dengan UI. Tidak ada jalan pintas permission.

## 13. Success Metrics

| Kategori | Metric |
|---|---|
| Outcome | MTTA P1/P2 sebelum vs sesudah; % laporan WA yang menjadi incident < 15 menit. |
| Leading | # channel aktif; % aksi assign/ack dilakukan dari Slack. |
| Guardrail | Delivery failure rate; channel status ERROR; opt-out WA; biaya WA per bulan. |

## 14. Milestones

| Fase | Estimasi | Scope |
|---|---|---|
| CHAT-1 | ± 1 minggu | Slack + Teams incoming webhook, routing, retry, log, UI master. |
| CHAT-2 | ± 1,5 minggu | Slack App: signing, pemetaan user, tombol aksi, update pesan, `/incident`. |
| CHAT-3 | ± 2 minggu + waktu approval Meta/BSP | WA template notifikasi + intake + antrean + opt-in. |

## 15. Acceptance Test Matrix (ringkas)

- Routing: kombinasi filter severity/app/team, dengan test table-driven.
- Webhook gagal 5xx → retry 3x; 4xx → channel ERROR tanpa retry berlebih.
- API tidak pernah mengembalikan `webhook_url` utuh (test JSON response).
- Slack signature salah/kedaluwarsa → 401; user tak terpetakan → aksi ditolak; user tak berhak → ditolak dengan pesan.
- WA signature salah → 401; opt-in false → tidak terkirim; intake convert → incident `source=whatsapp`.

## 16. Risiko

- **Kebocoran data ke platform pihak ketiga:** default tanpa deskripsi; channel internal saja.
- **Spam di intake WA:** antrean manual + tolak; rate limit per nomor.
- **Biaya & ketergantungan approval Meta:** CHAT-3 dipisah dan dijadikan Could.
- **Pemetaan user via email tidak cocok:** halaman pemetaan manual sebagai cadangan.

## 17. Coding-Agent Handoff (ringkas)

- Paket `internal/chat` dengan interface `Provider{Send(ctx, msg)}` (Slack, Teams, WhatsApp).
- Hook di `fanout`/`changeFanout` untuk routing channel.
- Enkripsi di paket kecil `internal/secretbox` (crypto/aes + cipher.GCM, stdlib).

## 18. Dependensi

- Tidak ada SDK pihak ketiga wajib. Slack, Teams webhook, dan WhatsApp Cloud API memakai HTTP+JSON stdlib.
- Butuh akun/aplikasi: Slack App (CHAT-2), WhatsApp Business Account/BSP (CHAT-3).

## 19. Keputusan yang Dibutuhkan

| # | Pertanyaan | Usulan default |
|---|---|---|
| Q1 | Platform chat yang dipakai perusahaan: Slack, Teams, atau keduanya? Perlu interaktif di Teams juga? | CHAT-1 keduanya (murah); interaktif hanya Slack dulu. |
| Q2 | Event apa yang dikirim ke channel secara default? | created (S1/S2), assigned, verification_failed, resolved, `sla_breached`, `change_failed`. |
| Q3 | Boleh mengirim deskripsi incident ke chat? | Tidak, default off; bisa diaktifkan per channel. |
| Q4 | WhatsApp: Meta Cloud API langsung atau BSP lokal (mis. Qontak/Wati)? Ada nomor bisnis & anggaran? | Tentukan sebelum CHAT-3; CHAT-3 ditunda sampai siap. |
| Q5 | Siapa yang boleh mengelola channel? | ManagerLead + DevOps. |
| Q6 | Kerjakan CHAT-3 sekarang atau setelah CHAT-1/2 terbukti? | Setelah CHAT-1/2. |
