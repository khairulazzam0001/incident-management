import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../api/client";
import type { SLABucket } from "../api/types";
import { formatDuration } from "./SlaBadge";

// Section SLA di Dashboard (PRD_SLA_Escalation.md §13, SLA-09): compliance
// per priority, MTTA/MTTR, dan breach yang masih terbuka. Periode 30 hari.

function pct(v: number | null): string {
  return v === null ? "—" : `${(v * 100).toFixed(1)}%`;
}

function overall(buckets: SLABucket[], metric: string): number | null {
  let met = 0;
  let breached = 0;
  for (const b of buckets) {
    if (b.metric !== metric) continue;
    met += b.met;
    breached += b.breached;
  }
  return met + breached === 0 ? null : met / (met + breached);
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-mist bg-paper p-6">
      <p className="text-xs uppercase text-veil">{label}</p>
      <p className="mt-1 text-2xl font-bold">{value}</p>
    </div>
  );
}

export function SlaDashboardPanel() {
  const board = useQuery({
    queryKey: ["sla-dashboard"],
    queryFn: ({ signal }) => api.getSLADashboard(signal),
    refetchInterval: 60_000,
  });

  if (board.isPending) {
    return <div className="h-24 animate-pulse rounded bg-lilac" aria-label="Memuat SLA" />;
  }
  if (board.isError) {
    return (
      <div className="rounded border border-red-200 bg-red-50 p-4 text-sm">
        <p className="text-red-700">Gagal memuat ringkasan SLA.</p>
        <button
          onClick={() => board.refetch()}
          className="mt-2 rounded-full border border-mist px-3 py-1 text-deep hover:bg-lilac"
        >
          Coba lagi
        </button>
      </div>
    );
  }

  const d = board.data;
  const priorities = [...new Set(d.buckets.map((b) => b.priority))].sort();
  const cell = (priority: string, metric: string) =>
    d.buckets.find((b) => b.priority === priority && b.metric === metric);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="text-xl font-semibold">SLA</h2>
        <span className="text-xs text-veil">
          {d.from} – {d.to}
          {d.worker_last_tick_at &&
            ` · dicek ${new Date(d.worker_last_tick_at).toLocaleTimeString("id-ID")}`}
        </span>
      </div>
      <div className="grid grid-cols-2 gap-3 md:grid-cols-3">
        <Stat label="Response tepat waktu" value={pct(overall(d.buckets, "RESPONSE"))} />
        <Stat label="Resolution tepat waktu" value={pct(overall(d.buckets, "RESOLUTION"))} />
        <Stat label="Breach aktif" value={String(d.active_breaches.length)} />
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        <div className="overflow-x-auto rounded-lg border border-mist bg-paper p-6">
          <h3 className="font-semibold">Per priority</h3>
          <table className="mt-2 w-full text-sm">
            <thead className="text-left text-xs uppercase text-veil">
              <tr>
                <th className="py-1">Priority</th>
                <th>Response</th>
                <th>MTTA</th>
                <th>Resolution</th>
                <th>MTTR</th>
              </tr>
            </thead>
            <tbody>
              {priorities.map((p) => {
                const r = cell(p, "RESPONSE");
                const s = cell(p, "RESOLUTION");
                return (
                  <tr key={p} className="border-t border-mist">
                    <td className="py-1 font-mono">{p}</td>
                    <td>{pct(r?.compliance ?? null)}</td>
                    <td>{r?.avg_minutes != null ? formatDuration(r.avg_minutes * 60_000) : "—"}</td>
                    <td>{pct(s?.compliance ?? null)}</td>
                    <td>{s?.avg_minutes != null ? formatDuration(s.avg_minutes * 60_000) : "—"}</td>
                  </tr>
                );
              })}
              {priorities.length === 0 && (
                <tr>
                  <td colSpan={5} className="py-2 text-veil">
                    Belum ada data SLA pada periode ini.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        <div className="rounded-lg border border-mist bg-paper p-6">
          <h3 className="font-semibold">Breach aktif</h3>
          <ul className="mt-2 space-y-2 text-sm">
            {d.active_breaches.map((b) => (
              <li key={`${b.incident_id}-${b.metric}`} className="flex flex-wrap items-center gap-2">
                <Link to={`/incidents/${b.incident_id}`} className="font-mono text-xs text-iris hover:underline">
                  {b.incident_no}
                </Link>
                <span className="font-mono text-xs">{b.priority}</span>
                <span className="min-w-0 flex-1 truncate">{b.title}</span>
                <span className="text-xs text-red-700">
                  {b.metric === "RESPONSE" ? "response" : "resolution"} sejak{" "}
                  {new Date(b.target_at).toLocaleString("id-ID")}
                </span>
              </li>
            ))}
            {d.active_breaches.length === 0 && <li className="text-veil">Tidak ada breach aktif.</li>}
          </ul>
        </div>
      </div>
    </div>
  );
}
