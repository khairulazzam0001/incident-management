import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import type { NamedCount } from "../api/types";
import { canViewChanges, useAuth } from "../auth/AuthContext";
import { ChangeSummaryPanel } from "../components/ChangeSummaryPanel";

function fmtHours(v: number | null): string {
  if (v === null || !Number.isFinite(v)) return "—";
  return `${v.toFixed(1)} jam`;
}

function fmtRate(v: number | null): string {
  if (v === null || !Number.isFinite(v)) return "—";
  return `${(v * 100).toFixed(1)}%`;
}

function Breakdown({ title, entries }: { title: string; entries: [string, number][] }) {
  const max = Math.max(1, ...entries.map(([, n]) => n));
  return (
    <div className="rounded-lg border border-mist bg-paper p-6">
      <h2 className="font-semibold">{title}</h2>
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

function NamedList({ title, items }: { title: string; items: NamedCount[] }) {
  return (
    <div className="rounded-lg border border-mist bg-paper p-6">
      <h2 className="font-semibold">{title}</h2>
      <ul className="mt-2 space-y-1 text-sm">
        {items.slice(0, 8).map((it) => (
          <li key={it.id ?? it.name} className="flex justify-between gap-2">
            <span className="truncate">{it.name}</span>
            <span className="font-medium">{it.count}</span>
          </li>
        ))}
        {items.length === 0 && <li className="text-veil">Tidak ada data.</li>}
      </ul>
    </div>
  );
}

function Card({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-mist bg-paper p-6">
      <p className="text-xs uppercase text-veil">{label}</p>
      <p className="mt-1 text-2xl font-bold">{value}</p>
    </div>
  );
}

export function Dashboard() {
  const { user } = useAuth();
  const dashboard = useQuery({
    queryKey: ["dashboard"],
    queryFn: ({ signal }) => api.getDashboard(signal),
  });

  if (dashboard.isPending) {
    return (
      <section className="mx-auto max-w-5xl p-6" aria-label="Memuat">
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="h-20 animate-pulse rounded bg-lilac" />
          ))}
        </div>
      </section>
    );
  }

  if (dashboard.isError || !dashboard.data) {
    return (
      <section className="mx-auto max-w-5xl p-6">
        <div className="rounded border border-red-200 bg-red-50 p-4 text-sm">
          <p className="text-red-700">Gagal memuat dashboard.</p>
          <button
            onClick={() => dashboard.refetch()}
            className="mt-2 rounded border px-3 py-1 hover:bg-white"
          >
            Coba lagi
          </button>
        </div>
      </section>
    );
  }

  const d = dashboard.data;

  return (
    <section className="w-full space-y-4 p-6">
      <h1 className="text-2xl font-semibold">Dashboard</h1>
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <Card label="Open incident" value={String(d.open_total)} />
        <Card label="Open milik saya" value={String(d.my_open)} />
        <Card label="Rata-rata assign" value={fmtHours(d.avg_hours_create_to_assign)} />
        <Card label="Rata-rata resolve" value={fmtHours(d.avg_hours_create_to_resolve)} />
        <Card label="Rata-rata close" value={fmtHours(d.avg_hours_create_to_close)} />
        <Card label="Reopen rate" value={fmtRate(d.reopen_rate)} />
        <Card label="Verification gagal" value={fmtRate(d.verification_failure_rate)} />
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        <Breakdown title="Open per status" entries={Object.entries(d.by_status)} />
        <Breakdown title="Open per severity" entries={Object.entries(d.by_severity)} />
        <Breakdown title="Open per priority" entries={Object.entries(d.by_priority)} />
        <NamedList title="Open per aplikasi" items={d.by_application} />
        <NamedList title="Open per team" items={d.by_team} />
      </div>
      {user !== null && canViewChanges(user.role) && <ChangeSummaryPanel />}
    </section>
  );
}
