import { useState } from "react";
import type { FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { API_BASE_URL, ApiError, api } from "../api/client";
import type { DryRunAlert, Integration, IntegrationInput, IntegrationType } from "../api/types";
import { canManageIntegrations, useAuth } from "../auth/AuthContext";
import { useMeta } from "../hooks/useMeta";

// Integrasi monitoring (PRD_Monitoring_Integration.md §11): webhook
// Alertmanager/Grafana/JSON → incident. Manager/Lead & DevOps.

const TYPE_LABEL: Record<IntegrationType, string> = {
  ALERTMANAGER: "Prometheus Alertmanager",
  GRAFANA: "Grafana Alerting",
  GENERIC_JSON: "JSON generik",
};

const SAMPLE_PAYLOAD = JSON.stringify(
  {
    alerts: [
      {
        status: "firing",
        fingerprint: "contoh-1",
        labels: { alertname: "HighErrorRate", severity: "critical", service: "checkout", env: "production" },
        annotations: { summary: "Error rate checkout > 5%", description: "5xx naik sejak deploy" },
        generatorURL: "http://prometheus/graph",
      },
    ],
  },
  null,
  2,
);

const inputClass =
  "mt-1 w-full rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none";
const primaryButton =
  "rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50";
const secondaryButton =
  "rounded-full border border-mist px-3 py-1 text-xs font-semibold text-deep hover:bg-lilac disabled:opacity-50";

function errorText(err: unknown, fallback: string): string {
  return err instanceof ApiError ? err.message : fallback;
}

function TokenReveal({ name, path, token, onClose }: { name: string; path: string; token: string; onClose: () => void }) {
  const url = `${API_BASE_URL}${path}`;
  const snippet = `receivers:
  - name: incident-management
    webhook_configs:
      - url: ${url}
        send_resolved: true
        http_config:
          authorization:
            type: Bearer
            credentials: ${token}`;
  return (
    <div className="rounded-lg border border-amber-200 bg-amber-50 p-6 text-sm text-ink">
      <h2 className="font-semibold">Token untuk {name}</h2>
      <p className="mt-1 text-amber-800">
        Salin sekarang: token hanya ditampilkan sekali. Bila hilang, lakukan rotasi token.
      </p>
      <dl className="mt-3 space-y-2">
        <div>
          <dt className="text-xs uppercase text-veil">Webhook URL</dt>
          <dd className="break-all font-mono text-xs">{url}</dd>
        </div>
        <div>
          <dt className="text-xs uppercase text-veil">Header</dt>
          <dd className="break-all font-mono text-xs">Authorization: Bearer {token}</dd>
        </div>
      </dl>
      <p className="mt-3 text-xs text-veil">Contoh konfigurasi Alertmanager:</p>
      <pre className="mt-1 overflow-x-auto rounded bg-paper p-3 font-mono text-xs">{snippet}</pre>
      <div className="mt-3 flex flex-wrap gap-2">
        <button onClick={() => void navigator.clipboard?.writeText(token)} className={secondaryButton}>
          Salin token
        </button>
        <button onClick={() => void navigator.clipboard?.writeText(snippet)} className={secondaryButton}>
          Salin konfigurasi
        </button>
        <button onClick={onClose} className={secondaryButton}>
          Sudah disalin, tutup
        </button>
      </div>
    </div>
  );
}

function SettingsForm({
  initial,
  creating,
  pending,
  onSubmit,
  onCancel,
}: {
  initial: IntegrationInput;
  creating: boolean;
  pending: boolean;
  onSubmit: (v: IntegrationInput) => void;
  onCancel?: () => void;
}) {
  const meta = useMeta();
  const [v, setV] = useState<IntegrationInput>(initial);
  const [allowlist, setAllowlist] = useState(initial.ip_allowlist.join(", "));
  function submit(e: FormEvent) {
    e.preventDefault();
    onSubmit({
      ...v,
      name: v.name.trim(),
      ip_allowlist: allowlist.split(",").map((x) => x.trim()).filter((x) => x !== ""),
    });
  }
  return (
    <form onSubmit={submit} className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      {creating && (
        <label className="block text-sm">
          Tipe
          <select value={v.type} onChange={(e) => setV({ ...v, type: e.target.value as IntegrationType })} className={inputClass}>
            {Object.entries(TYPE_LABEL).map(([k, label]) => (
              <option key={k} value={k}>
                {label}
              </option>
            ))}
          </select>
        </label>
      )}
      <label className="block text-sm">
        Nama
        <input value={v.name} onChange={(e) => setV({ ...v, name: e.target.value })} className={inputClass}
          placeholder="Cth: Prometheus Production" />
      </label>
      <label className="block text-sm">
        Application default
        <select value={v.default_application_id ?? ""}
          onChange={(e) => setV({ ...v, default_application_id: e.target.value || null })} className={inputClass}>
          <option value="">— (dari label service/app)</option>
          {(meta.data?.applications ?? []).map((a) => (
            <option key={a.id} value={a.id}>
              {a.name} ({a.code})
            </option>
          ))}
        </select>
      </label>
      <label className="block text-sm">
        Environment default
        <select value={v.default_environment ?? ""}
          onChange={(e) => setV({ ...v, default_environment: e.target.value || null })} className={inputClass}>
          <option value="">— (dari label env)</option>
          {(meta.data?.environments ?? []).map((env) => (
            <option key={env.code} value={env.code}>
              {env.name}
            </option>
          ))}
        </select>
      </label>
      <label className="block text-sm">
        Buat incident mulai severity
        <select value={v.min_severity} onChange={(e) => setV({ ...v, min_severity: e.target.value })} className={inputClass}>
          {(meta.data?.severities ?? []).map((s) => (
            <option key={s.code} value={s.code}>
              {s.code} — {s.name} ke atas
            </option>
          ))}
        </select>
      </label>
      <label className="block text-sm">
        Rate limit (request/menit)
        <input type="number" min={1} max={6000} value={v.rate_limit_per_min}
          onChange={(e) => setV({ ...v, rate_limit_per_min: Number(e.target.value) })} className={inputClass} />
      </label>
      <label className="block text-sm sm:col-span-2">
        IP allowlist (opsional, pisahkan dengan koma)
        <input value={allowlist} onChange={(e) => setAllowlist(e.target.value)} className={inputClass}
          placeholder="Cth: 10.0.0.0/8, 192.168.1.20" />
      </label>
      <div className="flex gap-2 sm:col-span-2">
        <button type="submit" disabled={pending} className={primaryButton}>
          {pending ? "Menyimpan…" : creating ? "Buat integrasi" : "Simpan"}
        </button>
        {onCancel && (
          <button type="button" onClick={onCancel} className="rounded-full px-4 py-2 text-sm font-semibold text-veil hover:bg-lilac">
            Batal
          </button>
        )}
      </div>
    </form>
  );
}

function toInput(it: Integration): IntegrationInput {
  return {
    name: it.name,
    is_active: it.is_active,
    default_application_id: it.default_application_id,
    default_environment: it.default_environment,
    min_severity: it.min_severity,
    mapping: it.mapping,
    ip_allowlist: it.ip_allowlist,
    rate_limit_per_min: it.rate_limit_per_min,
  };
}

const RESULT_CLASS: Record<string, string> = {
  created: "bg-green-100 text-green-700",
  updated: "bg-lilac text-iris",
  grouped: "bg-amber-100 text-amber-800",
  ignored: "bg-chalk text-veil",
  duplicate: "bg-chalk text-veil",
  error: "bg-red-100 text-red-700",
};

function EventLog({ id }: { id: string }) {
  const [page, setPage] = useState(1);
  const events = useQuery({
    queryKey: ["alert-events", id, page],
    queryFn: ({ signal }) => api.getAlertEvents(id, page, signal),
  });
  if (events.isPending) return <div className="h-16 animate-pulse rounded bg-lilac" />;
  if (events.isError) {
    return (
      <p className="text-sm text-red-700">
        Gagal memuat event.{" "}
        <button onClick={() => events.refetch()} className="text-iris hover:underline">
          Coba lagi
        </button>
      </p>
    );
  }
  const totalPages = Math.max(1, Math.ceil(events.data.meta.total / 20));
  return (
    <div>
      <ul className="divide-y divide-mist text-sm">
        {events.data.data.map((e) => (
          <li key={e.id} className="flex flex-wrap items-center gap-2 py-2">
            <span className="w-36 shrink-0 text-xs text-veil">{new Date(e.received_at).toLocaleString("id-ID")}</span>
            <span className={`rounded-full px-2 py-0.5 text-xs font-semibold ${RESULT_CLASS[e.result] ?? ""}`}>{e.result}</span>
            <span className="text-xs">{e.status}</span>
            <span className="font-mono text-xs text-veil">{e.fingerprint || "—"}</span>
            {e.incident_id && (
              <Link to={`/incidents/${e.incident_id}`} className="font-mono text-xs text-iris hover:underline">
                {e.incident_no}
              </Link>
            )}
            {(e.warning || e.error) && <span className="text-xs text-amber-800">{e.error || e.warning}</span>}
          </li>
        ))}
        {events.data.data.length === 0 && <li className="py-2 text-veil">Belum ada event.</li>}
      </ul>
      {totalPages > 1 && (
        <div className="mt-2 flex items-center gap-2 text-xs">
          <button disabled={page <= 1} onClick={() => setPage((p) => p - 1)} className={secondaryButton}>
            ← Sebelumnya
          </button>
          <span className="text-veil">
            Hal {page} dari {totalPages}
          </span>
          <button disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)} className={secondaryButton}>
            Berikutnya →
          </button>
        </div>
      )}
    </div>
  );
}

function DryRun({ id }: { id: string }) {
  const [payload, setPayload] = useState(SAMPLE_PAYLOAD);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<DryRunAlert[] | null>(null);
  const run = useMutation({
    mutationFn: (body: unknown) => api.testIntegration(id, body),
    onSuccess: (res) => {
      setError(null);
      setResult(res.data);
    },
    onError: (err: unknown) => setError(errorText(err, "Uji gagal.")),
  });
  function submit(e: FormEvent) {
    e.preventDefault();
    try {
      run.mutate(JSON.parse(payload) as unknown);
    } catch {
      setError("Payload bukan JSON valid.");
    }
  }
  return (
    <form onSubmit={submit} className="space-y-2">
      <p className="text-xs text-veil">Uji payload tanpa membuat incident (dry-run).</p>
      <textarea value={payload} onChange={(e) => setPayload(e.target.value)} rows={8}
        className={`${inputClass} font-mono text-xs`} aria-label="Payload uji" />
      <button type="submit" disabled={run.isPending} className={secondaryButton}>
        {run.isPending ? "…" : "Jalankan uji"}
      </button>
      {error !== null && <p className="text-sm text-red-700">{error}</p>}
      {result !== null && (
        <ul className="space-y-1 text-sm">
          {result.map((r, i) => (
            <li key={i} className="rounded bg-chalk p-2">
              <span className="font-semibold">{r.action === "create" ? "Buat incident" : r.action === "ignore" ? "Diabaikan" : "Update (resolved)"}</span>
              {" · "}
              {r.mapped.severity}/{r.mapped.priority} · {r.mapped.title} · app {r.mapped.application_code || "—"} ·{" "}
              env {r.environment ?? "—"}
              {r.warning && <span className="block text-xs text-amber-800">{r.warning}</span>}
            </li>
          ))}
        </ul>
      )}
    </form>
  );
}

function IntegrationCard({
  it,
  onToken,
  onDone,
  onError,
}: {
  it: Integration;
  onToken: (name: string, path: string, token: string) => void;
  onDone: (msg: string) => void;
  onError: (msg: string) => void;
}) {
  const queryClient = useQueryClient();
  const [panel, setPanel] = useState<"none" | "edit" | "test" | "events">("none");
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["integrations"] });
  const update = useMutation({
    mutationFn: (v: IntegrationInput) => api.updateIntegration(it.id, v),
    onSuccess: () => {
      setPanel("none");
      refresh();
      onDone(`Integrasi ${it.name} disimpan.`);
    },
    onError: (err: unknown) => onError(errorText(err, "Gagal menyimpan integrasi.")),
  });
  const rotate = useMutation({
    mutationFn: () => api.rotateIntegrationToken(it.id),
    onSuccess: (res) => {
      refresh();
      onToken(it.name, `/api/integrations/alerts/${it.id}`, res.token);
    },
    onError: (err: unknown) => onError(errorText(err, "Gagal rotasi token.")),
  });
  const toggle = (v: string) => setPanel((p) => (p === v ? "none" : (v as typeof panel)));

  return (
    <li className="rounded-lg border border-mist bg-paper p-4">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-semibold">{it.name}</span>
        <span className="rounded-full border border-mist bg-chalk px-2 py-0.5 text-xs">{TYPE_LABEL[it.type]}</span>
        <span className={`rounded-full px-2 py-0.5 text-xs font-semibold ${it.is_active ? "bg-green-100 text-green-700" : "bg-chalk text-veil"}`}>
          {it.is_active ? "Aktif" : "Nonaktif"}
        </span>
        <span className="text-xs text-veil">
          token …{it.token_last4} · {it.events_24h} event/24 jam · terakhir{" "}
          {it.last_event_at ? new Date(it.last_event_at).toLocaleString("id-ID") : "—"}
        </span>
      </div>
      <p className="mt-1 break-all font-mono text-xs text-veil">
        POST {API_BASE_URL}/api/integrations/alerts/{it.id}
      </p>
      <div className="mt-3 flex flex-wrap gap-2">
        <button onClick={() => toggle("edit")} className={secondaryButton}>Pengaturan</button>
        <button onClick={() => toggle("test")} className={secondaryButton}>Uji payload</button>
        <button onClick={() => toggle("events")} className={secondaryButton}>Event log</button>
        <button onClick={() => update.mutate({ ...toInput(it), is_active: !it.is_active })} disabled={update.isPending} className={secondaryButton}>
          {it.is_active ? "Nonaktifkan" : "Aktifkan"}
        </button>
        <button onClick={() => rotate.mutate()} disabled={rotate.isPending} className={secondaryButton}>
          Rotasi token
        </button>
      </div>
      {panel !== "none" && (
        <div className="mt-4 border-t border-mist pt-4">
          {panel === "edit" && (
            <SettingsForm initial={toInput(it)} creating={false} pending={update.isPending}
              onSubmit={(v) => update.mutate({ ...v, is_active: it.is_active })} onCancel={() => setPanel("none")} />
          )}
          {panel === "test" && <DryRun id={it.id} />}
          {panel === "events" && <EventLog id={it.id} />}
        </div>
      )}
    </li>
  );
}

export function Integrations() {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const allowed = user !== null && canManageIntegrations(user.role);
  const [creating, setCreating] = useState(false);
  const [reveal, setReveal] = useState<{ name: string; path: string; token: string } | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const list = useQuery({
    queryKey: ["integrations"],
    queryFn: ({ signal }) => api.listIntegrations(signal),
    enabled: allowed,
  });
  const create = useMutation({
    mutationFn: (v: IntegrationInput) => api.createIntegration(v),
    onSuccess: (res) => {
      setCreating(false);
      setError(null);
      setSuccess(null);
      setReveal({ name: res.integration.name, path: res.webhook_path, token: res.token });
      queryClient.invalidateQueries({ queryKey: ["integrations"] });
    },
    onError: (err: unknown) => setError(errorText(err, "Gagal membuat integrasi.")),
  });

  if (!allowed) {
    return (
      <section className="w-full p-6">
        <p className="rounded-lg border border-mist bg-paper p-6 text-sm text-veil">
          Hanya Manager/Lead atau DevOps yang boleh mengelola integrasi.
        </p>
      </section>
    );
  }

  return (
    <section className="w-full space-y-4 p-6">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="text-2xl font-semibold">Integrasi Monitoring</h1>
          <p className="text-sm text-veil">Alert dari monitoring otomatis menjadi incident (dengan deduplikasi).</p>
        </div>
        {!creating && (
          <button onClick={() => setCreating(true)} className={primaryButton}>
            Tambah integrasi
          </button>
        )}
      </div>
      {error !== null && <p className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>}
      {success !== null && <p className="rounded border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-700">{success}</p>}
      {reveal !== null && <TokenReveal {...reveal} onClose={() => setReveal(null)} />}
      {creating && (
        <div className="rounded-lg border border-mist bg-paper p-6">
          <h2 className="mb-3 font-semibold">Integrasi baru</h2>
          <SettingsForm
            creating
            pending={create.isPending}
            initial={{ type: "ALERTMANAGER", name: "", default_application_id: null, default_environment: null,
              min_severity: "S3", ip_allowlist: [], rate_limit_per_min: 60 }}
            onSubmit={(v) => create.mutate(v)}
            onCancel={() => setCreating(false)}
          />
        </div>
      )}
      {list.isPending && <div className="h-24 animate-pulse rounded-lg bg-lilac" aria-label="Memuat" />}
      {list.isError && (
        <div className="rounded border border-red-200 bg-red-50 p-4 text-sm text-red-700">
          Gagal memuat integrasi.{" "}
          <button onClick={() => list.refetch()} className="text-iris hover:underline">
            Coba lagi
          </button>
        </div>
      )}
      {list.data && list.data.data.length === 0 && !creating && (
        <div className="rounded-lg border border-mist bg-paper p-8 text-center text-sm text-veil">
          Belum ada integrasi. Tambahkan Alertmanager atau Grafana agar alert otomatis menjadi incident.
        </div>
      )}
      {list.data && list.data.data.length > 0 && (
        <ul className="space-y-3">
          {list.data.data.map((it) => (
            <IntegrationCard
              key={it.id}
              it={it}
              onToken={(name, path, token) => {
                setSuccess(null);
                setReveal({ name, path, token });
              }}
              onDone={(msg) => {
                setError(null);
                setSuccess(msg);
              }}
              onError={(msg) => {
                setSuccess(null);
                setError(msg);
              }}
            />
          ))}
        </ul>
      )}
    </section>
  );
}
