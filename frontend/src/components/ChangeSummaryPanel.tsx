import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../api/client";
import { ChangeTypeBadge } from "./ChangeTypeBadge";

// Section Change di Dashboard (PRD_Change_Management.md §14). Failure rate dan
// emergency ratio selalu tampil berdampingan dengan success rate.

function fmtRate(v: number | null): string {
  return v === null || !Number.isFinite(v) ? "—" : `${(v * 100).toFixed(1)}%`;
}

function fmtHours(v: number | null): string {
  return v === null || !Number.isFinite(v) ? "—" : `${v.toFixed(1)} jam`;
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-mist bg-paper p-6">
      <p className="text-xs uppercase text-veil">{label}</p>
      <p className="mt-1 text-2xl font-bold">{value}</p>
    </div>
  );
}

function Counts({ title, counts }: { title: string; counts: Record<string, number> }) {
  const entries = Object.entries(counts);
  const max = Math.max(1, ...entries.map(([, n]) => n));
  return (
    <div className="rounded-lg border border-mist bg-paper p-6">
      <h3 className="font-semibold">{title}</h3>
      <ul className="mt-2 space-y-1 text-sm">
        {entries.map(([label, n]) => (
          <li key={label} className="flex items-center gap-2">
            <span className="w-28 shrink-0 font-mono text-xs">{label}</span>
            <span
              className="h-3 rounded-full bg-iris"
              style={{ width: `${Math.max(2, (n / max) * 100)}%` }}
            />
            <span className="text-veil">{n}</span>
          </li>
        ))}
        {entries.length === 0 && <li className="text-veil">Tidak ada data.</li>}
      </ul>
    </div>
  );
}

export function ChangeSummaryPanel() {
  const summary = useQuery({
    queryKey: ["change-summary"],
    queryFn: ({ signal }) => api.getChangeSummary(signal),
  });

  if (summary.isPending) {
    return <div className="h-24 animate-pulse rounded bg-lilac" aria-label="Memuat" />;
  }
  if (summary.isError) {
    return (
      <div className="rounded border border-red-200 bg-red-50 p-4 text-sm">
        <p className="text-red-700">Gagal memuat ringkasan change.</p>
        <button
          onClick={() => summary.refetch()}
          className="mt-2 rounded-full border border-mist px-3 py-1 text-deep hover:bg-lilac"
        >
          Coba lagi
        </button>
      </div>
    );
  }

  const s = summary.data;
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold">Change</h2>
        <Link to="/changes/calendar" className="text-sm text-iris hover:underline">
          Lihat kalender →
        </Link>
      </div>
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <Stat label="Change aktif" value={String(s.active_total)} />
        <Stat label="Success rate" value={fmtRate(s.success_rate)} />
        <Stat label="Failure rate" value={fmtRate(s.failure_rate)} />
        <Stat label="Emergency ratio" value={fmtRate(s.emergency_ratio)} />
        <Stat label="Submit → approve" value={fmtHours(s.avg_hours_submit_to_approve)} />
        <Stat label="Approve → implement" value={fmtHours(s.avg_hours_approve_to_implement)} />
        <Stat label="PIR terisi" value={fmtRate(s.pir_completion_rate)} />
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        <Counts title="Change per status" counts={s.by_status} />
        <Counts title="Aktif per type" counts={s.by_type} />
        <Counts title="Aktif per risk" counts={s.by_risk} />
        <div className="rounded-lg border border-mist bg-paper p-6">
          <h3 className="font-semibold">Terjadwal 7 hari ke depan</h3>
          <ul className="mt-2 space-y-2 text-sm">
            {s.upcoming.map((c) => (
              <li key={c.id} className="flex flex-wrap items-center gap-2">
                <Link to={`/changes/${c.id}`} className="font-mono text-xs text-iris hover:underline">
                  {c.change_no}
                </Link>
                <ChangeTypeBadge type={c.type} />
                <span className="min-w-0 flex-1 truncate">{c.title}</span>
                <span className="text-xs text-veil">
                  {c.planned_start ? new Date(c.planned_start).toLocaleString("id-ID") : "—"}
                </span>
              </li>
            ))}
            {s.upcoming.length === 0 && <li className="text-veil">Tidak ada jadwal.</li>}
          </ul>
        </div>
      </div>
    </div>
  );
}
