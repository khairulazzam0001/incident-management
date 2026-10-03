import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import type { IncidentSLA } from "../api/types";
import { useNow } from "../hooks/useNow";
import { TONE_CLASS, formatDuration, slaTone } from "./SlaBadge";

// Panel SLA di Incident Detail (PRD_SLA_Escalation.md §11). Mode compact
// untuk User/Customer: hanya target penyelesaian.

function fmt(iso: string | null): string {
  return iso ? new Date(iso).toLocaleString("id-ID") : "—";
}

const METRIC_LABEL: Record<string, string> = { RESPONSE: "Response", RESOLUTION: "Resolution" };
const STATUS_LABEL: Record<string, string> = {
  RUNNING: "Berjalan",
  MET: "Terpenuhi",
  BREACHED: "Breach",
  CANCELLED: "Dibatalkan",
};

function Row({ s, now }: { s: IncidentSLA; now: number }) {
  const tone = slaTone(s, now);
  const target = new Date(s.target_at).getTime();
  const end = s.stopped_at ? new Date(s.stopped_at).getTime() : now;
  const diff = target - end;
  const timing =
    s.status === "CANCELLED"
      ? "—"
      : diff >= 0
        ? `${s.stopped_at ? "selesai" : "sisa"} ${formatDuration(diff)}${s.stopped_at ? " lebih cepat" : ""}`
        : `terlambat ${formatDuration(diff)}`;
  return (
    <li className="flex flex-wrap items-center gap-2 rounded-lg bg-chalk p-3 text-sm">
      <span className="w-24 font-medium">
        {METRIC_LABEL[s.metric] ?? s.metric}
        {s.cycle > 1 && <span className="text-xs text-veil"> · siklus {s.cycle}</span>}
      </span>
      <span className={`rounded-full border border-mist px-2 py-0.5 text-xs font-semibold ${TONE_CLASS[tone]}`}>
        {STATUS_LABEL[s.status] ?? s.status}
      </span>
      <span className="text-veil">target {fmt(s.target_at)}</span>
      <span className="text-veil">· {timing}</span>
      <span className="ml-auto text-xs text-veil">
        {s.priority} · {s.calendar_code === "24x7" ? "24x7" : "jam kerja"} ·{" "}
        {formatDuration(s.target_minutes * 60_000)}
      </span>
    </li>
  );
}

export function SlaPanel({
  incidentId,
  hasSla,
  compact,
}: {
  incidentId: string;
  hasSla: boolean;
  compact: boolean;
}) {
  const now = useNow();
  const query = useQuery({
    queryKey: ["incident-sla", incidentId],
    queryFn: ({ signal }) => api.getIncidentSLA(incidentId, signal),
    enabled: hasSla,
  });

  if (!hasSla) {
    return (
      <div className="rounded-lg border border-mist bg-paper p-6 text-sm">
        <h2 className="font-semibold">SLA</h2>
        <p className="mt-1 text-veil">SLA tidak berlaku (incident dibuat sebelum SLA aktif).</p>
      </div>
    );
  }
  if (query.isPending) {
    return <div className="h-24 animate-pulse rounded-lg bg-lilac" aria-label="Memuat SLA" />;
  }
  if (query.isError) {
    return (
      <div className="rounded-lg border border-mist bg-paper p-6 text-sm">
        <h2 className="font-semibold">SLA</h2>
        <p className="mt-1 text-red-700">
          Gagal memuat SLA.{" "}
          <button onClick={() => query.refetch()} className="text-iris hover:underline">
            Coba lagi
          </button>
        </p>
      </div>
    );
  }

  const items = query.data.data;
  if (compact) {
    const resolution = [...items].reverse().find((s) => s.metric === "RESOLUTION");
    return (
      <div className="rounded-lg border border-mist bg-paper p-6 text-sm">
        <h2 className="font-semibold">Target penyelesaian</h2>
        <p className="mt-1 text-ink">{fmt(resolution?.target_at ?? null)}</p>
      </div>
    );
  }
  const latestCycle = Math.max(...items.map((s) => s.cycle));
  const current = items.filter((s) => s.cycle === latestCycle || s.metric === "RESPONSE");
  const history = items.filter((s) => !current.includes(s));
  return (
    <div className="rounded-lg border border-mist bg-paper p-6">
      <h2 className="font-semibold">SLA</h2>
      <ul className="mt-2 space-y-2">
        {current.map((s) => (
          <Row key={s.id} s={s} now={now} />
        ))}
      </ul>
      {history.length > 0 && (
        <details className="mt-3 text-sm">
          <summary className="cursor-pointer text-veil">Riwayat siklus sebelumnya</summary>
          <ul className="mt-2 space-y-2">
            {history.map((s) => (
              <Row key={s.id} s={s} now={now} />
            ))}
          </ul>
        </details>
      )}
    </div>
  );
}
