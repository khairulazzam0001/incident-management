import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../api/client";
import type { Change } from "../api/types";
import { canViewChanges, useAuth } from "../auth/AuthContext";
import { useMeta } from "../hooks/useMeta";
import { ChangeStatusBadge } from "../components/ChangeStatusBadge";
import { RiskBadge } from "../components/RiskBadge";

// Kalender change per minggu (PRD_Change_Management.md CM-FR-11): daftar
// change terjadwal dikelompokkan per hari, tanpa library kalender.

function startOfWeek(d: Date): Date {
  const out = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  const offset = (out.getDay() + 6) % 7; // Senin = 0
  out.setDate(out.getDate() - offset);
  return out;
}

function addDays(d: Date, n: number): Date {
  const out = new Date(d);
  out.setDate(out.getDate() + n);
  return out;
}

function ymd(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

function hm(iso: string | null): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleTimeString("id-ID", { hour: "2-digit", minute: "2-digit" });
}

const navButton =
  "rounded-full border border-mist px-3 py-1 text-sm font-semibold text-deep hover:bg-lilac";

export function ChangeCalendar() {
  const { user } = useAuth();
  const meta = useMeta();
  const [weekStart, setWeekStart] = useState(() => startOfWeek(new Date()));
  const allowed = user !== null && canViewChanges(user.role);
  const weekEnd = addDays(weekStart, 6);

  const changes = useQuery({
    queryKey: ["changes-calendar", ymd(weekStart)],
    queryFn: ({ signal }) =>
      api.listChanges({
        // Filter server berbasis tanggal UTC; perluas sehari agar zona waktu lokal aman.
        scheduled_from: ymd(addDays(weekStart, -1)),
        scheduled_to: ymd(addDays(weekEnd, 1)),
        sort: "planned_start",
        limit: 100,
        signal,
      }),
    enabled: allowed,
  });
  const appName = useMemo(
    () => new Map((meta.data?.applications ?? []).map((a) => [a.id, a.name] as const)),
    [meta.data],
  );
  const days = useMemo(() => {
    const buckets = new Map<string, Change[]>();
    for (let i = 0; i < 7; i++) buckets.set(ymd(addDays(weekStart, i)), []);
    for (const c of changes.data?.data ?? []) {
      if (!c.planned_start) continue;
      buckets.get(ymd(new Date(c.planned_start)))?.push(c);
    }
    return [...buckets.entries()];
  }, [changes.data, weekStart]);

  if (!allowed) {
    return (
      <section className="w-full p-6">
        <p className="rounded-lg border border-mist bg-paper p-6 text-sm text-veil">
          Anda tidak berhak mengakses Change Management.
        </p>
      </section>
    );
  }

  const rangeLabel = `${weekStart.toLocaleDateString("id-ID", { day: "numeric", month: "short" })} – ${weekEnd.toLocaleDateString("id-ID", { day: "numeric", month: "short", year: "numeric" })}`;
  const total = days.reduce((n, [, items]) => n + items.length, 0);

  return (
    <section className="w-full p-6">
      <Link to="/changes" className="text-sm text-iris hover:underline">
        ← Kembali ke daftar change
      </Link>
      <div className="mt-2 flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold">Kalender Change</h1>
        <div className="flex flex-wrap items-center gap-2">
          <button className={navButton} onClick={() => setWeekStart((w) => addDays(w, -7))}>
            ← Minggu lalu
          </button>
          <button className={navButton} onClick={() => setWeekStart(startOfWeek(new Date()))}>
            Minggu ini
          </button>
          <button className={navButton} onClick={() => setWeekStart((w) => addDays(w, 7))}>
            Minggu depan →
          </button>
        </div>
      </div>
      <p className="mt-1 text-sm text-veil">
        {rangeLabel} · {total} change terjadwal
      </p>

      {changes.isPending && (
        <div className="mt-4 space-y-2" aria-label="Memuat">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-16 animate-pulse rounded bg-lilac" />
          ))}
        </div>
      )}
      {changes.isError && (
        <div className="mt-4 rounded border border-red-200 bg-red-50 p-4 text-sm">
          <p className="text-red-700">Gagal memuat jadwal change.</p>
          <button onClick={() => changes.refetch()} className={`mt-2 ${navButton}`}>
            Coba lagi
          </button>
        </div>
      )}
      {changes.data && (
        <ol className="mt-4 space-y-3">
          {days.map(([day, items]) => {
            const date = new Date(`${day}T00:00:00`);
            const isToday = day === ymd(new Date());
            return (
              <li key={day} className="rounded-lg border border-mist bg-paper p-4">
                <h2 className={`text-sm font-semibold ${isToday ? "text-iris" : "text-ink"}`}>
                  {date.toLocaleDateString("id-ID", { weekday: "long", day: "numeric", month: "long" })}
                  {isToday && " · hari ini"}
                </h2>
                {items.length === 0 ? (
                  <p className="mt-1 text-sm text-veil">Tidak ada change.</p>
                ) : (
                  <ul className="mt-2 divide-y divide-mist">
                    {items.map((c) => (
                      <li key={c.id}>
                        <Link
                          to={`/changes/${c.id}`}
                          className="flex flex-wrap items-center gap-2 py-2 text-sm hover:bg-chalk"
                        >
                          <span className="w-28 shrink-0 font-mono text-xs text-veil">
                            {hm(c.planned_start)}–{hm(c.planned_end)}
                          </span>
                          <span className="font-mono text-xs text-veil">{c.change_no}</span>
                          <ChangeStatusBadge status={c.status} />
                          <RiskBadge risk={c.risk} />
                          <span className="min-w-0 flex-1 truncate font-medium">{c.title}</span>
                          <span className="text-xs text-veil">
                            {appName.get(c.application_id) ?? "—"} · {c.environment}
                          </span>
                        </Link>
                      </li>
                    ))}
                  </ul>
                )}
              </li>
            );
          })}
        </ol>
      )}
    </section>
  );
}
