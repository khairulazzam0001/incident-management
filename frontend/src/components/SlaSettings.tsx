import { useState } from "react";
import type { FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, api } from "../api/client";
import type { BusinessCalendar, SLAPolicy } from "../api/types";

// Master Data → SLA (PRD_SLA_Escalation.md §11): target per priority, jam
// kerja, dan hari libur. Ubah hanya Manager/Lead; role lain read-only.

const DAYS = ["Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"];
const inputClass =
  "w-full rounded border border-mist bg-paper px-2 py-1 text-sm focus:border-iris focus:outline-none disabled:bg-chalk";

function toHHMM(m: number): string {
  return `${String(Math.floor(m / 60)).padStart(2, "0")}:${String(m % 60).padStart(2, "0")}`;
}

function fromHHMM(v: string): number {
  const [h, m] = v.split(":").map(Number);
  return (h ?? 0) * 60 + (m ?? 0);
}

function PolicyRow({
  policy,
  calendars,
  canEdit,
  onSaved,
  onError,
}: {
  policy: SLAPolicy;
  calendars: BusinessCalendar[];
  canEdit: boolean;
  onSaved: (msg: string) => void;
  onError: (err: unknown) => void;
}) {
  const [v, setV] = useState({
    response_minutes: policy.response_minutes,
    resolution_minutes: policy.resolution_minutes,
    calendar_code: policy.calendar_code,
    warn_percent: policy.warn_percent,
    breach_reminder_minutes: policy.breach_reminder_minutes,
  });
  const save = useMutation({
    mutationFn: () => api.updateSLAPolicy(policy.priority, v),
    onSuccess: () => onSaved(`SLA ${policy.priority} disimpan. Berlaku untuk incident baru.`),
    onError,
  });
  const num = (key: "response_minutes" | "resolution_minutes" | "warn_percent", value: string) =>
    setV((p) => ({ ...p, [key]: Number(value) }));

  return (
    <tr className="border-t border-mist">
      <td className="py-2 font-mono">{policy.priority}</td>
      <td>
        <input type="number" min={1} value={v.response_minutes} disabled={!canEdit}
          onChange={(e) => num("response_minutes", e.target.value)} className={inputClass}
          aria-label={`Response ${policy.priority} (menit)`} />
      </td>
      <td>
        <input type="number" min={1} value={v.resolution_minutes} disabled={!canEdit}
          onChange={(e) => num("resolution_minutes", e.target.value)} className={inputClass}
          aria-label={`Resolution ${policy.priority} (menit)`} />
      </td>
      <td>
        <select value={v.calendar_code} disabled={!canEdit}
          onChange={(e) => setV((p) => ({ ...p, calendar_code: e.target.value }))} className={inputClass}
          aria-label={`Kalender ${policy.priority}`}>
          {calendars.map((c) => (
            <option key={c.code} value={c.code}>
              {c.is_24x7 ? "24x7" : "Jam kerja"}
            </option>
          ))}
        </select>
      </td>
      <td>
        <input type="number" min={1} max={99} value={v.warn_percent} disabled={!canEdit}
          onChange={(e) => num("warn_percent", e.target.value)} className={inputClass}
          aria-label={`Peringatan ${policy.priority} (%)`} />
      </td>
      <td>
        <input type="number" min={1} placeholder="—" value={v.breach_reminder_minutes ?? ""} disabled={!canEdit}
          onChange={(e) =>
            setV((p) => ({ ...p, breach_reminder_minutes: e.target.value === "" ? null : Number(e.target.value) }))
          }
          className={inputClass} aria-label={`Pengingat breach ${policy.priority} (menit)`} />
      </td>
      {canEdit && (
        <td className="pl-2">
          <button onClick={() => save.mutate()} disabled={save.isPending}
            className="rounded-full bg-iris px-3 py-1 text-xs font-semibold text-white hover:brightness-95 disabled:opacity-50">
            {save.isPending ? "…" : "Simpan"}
          </button>
        </td>
      )}
    </tr>
  );
}

function HoursEditor({
  calendar,
  canEdit,
  onSaved,
  onError,
}: {
  calendar: BusinessCalendar;
  canEdit: boolean;
  onSaved: (msg: string) => void;
  onError: (err: unknown) => void;
}) {
  const [days, setDays] = useState(() =>
    DAYS.map((_, wd) => {
      const h = calendar.hours.find((x) => x.weekday === wd);
      return { on: h !== undefined, start: toHHMM(h?.start_minute ?? 480), end: toHHMM(h?.end_minute ?? 1020) };
    }),
  );
  const save = useMutation({
    mutationFn: () =>
      api.updateBusinessHours(
        calendar.code,
        days.flatMap((d, wd) =>
          d.on ? [{ weekday: wd, start_minute: fromHHMM(d.start), end_minute: fromHHMM(d.end) }] : [],
        ),
      ),
    onSuccess: () => onSaved("Jam kerja disimpan. Berlaku untuk perhitungan target berikutnya."),
    onError,
  });
  return (
    <div>
      <h3 className="font-semibold">Jam kerja ({calendar.timezone})</h3>
      <ul className="mt-2 space-y-1 text-sm">
        {days.map((d, wd) => (
          <li key={wd} className="flex items-center gap-2">
            <label className="flex w-24 items-center gap-1">
              <input type="checkbox" checked={d.on} disabled={!canEdit}
                onChange={(e) => setDays((all) => all.map((x, i) => (i === wd ? { ...x, on: e.target.checked } : x)))} />
              {DAYS[wd]}
            </label>
            <input type="time" value={d.start} disabled={!canEdit || !d.on} className="rounded border border-mist px-2 py-1 disabled:bg-chalk"
              onChange={(e) => setDays((all) => all.map((x, i) => (i === wd ? { ...x, start: e.target.value } : x)))}
              aria-label={`Mulai ${DAYS[wd]}`} />
            <span>–</span>
            <input type="time" value={d.end} disabled={!canEdit || !d.on} className="rounded border border-mist px-2 py-1 disabled:bg-chalk"
              onChange={(e) => setDays((all) => all.map((x, i) => (i === wd ? { ...x, end: e.target.value } : x)))}
              aria-label={`Selesai ${DAYS[wd]}`} />
          </li>
        ))}
      </ul>
      {canEdit && (
        <button onClick={() => save.mutate()} disabled={save.isPending}
          className="mt-2 rounded-full bg-iris px-4 py-1.5 text-sm font-semibold text-white hover:brightness-95 disabled:opacity-50">
          {save.isPending ? "Menyimpan…" : "Simpan jam kerja"}
        </button>
      )}
    </div>
  );
}

export function SlaSettings({ canEdit }: { canEdit: boolean }) {
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const year = new Date().getFullYear();
  const [holidayDate, setHolidayDate] = useState("");
  const [holidayName, setHolidayName] = useState("");

  const settings = useQuery({
    queryKey: ["sla-settings"],
    queryFn: ({ signal }) => api.getSLASettings(signal),
  });
  const holidays = useQuery({
    queryKey: ["holidays", year],
    queryFn: ({ signal }) => api.getHolidays(year, signal),
  });

  function saved(msg: string) {
    setError(null);
    setSuccess(msg);
    queryClient.invalidateQueries({ queryKey: ["sla-settings"] });
    queryClient.invalidateQueries({ queryKey: ["holidays"] });
  }
  function fail(err: unknown) {
    setSuccess(null);
    setError(err instanceof ApiError ? err.message : "Aksi gagal. Coba lagi.");
  }

  const addHoliday = useMutation({
    mutationFn: () => api.createHoliday({ date: holidayDate, name: holidayName.trim() }),
    onSuccess: () => {
      setHolidayDate("");
      setHolidayName("");
      saved("Hari libur ditambahkan.");
    },
    onError: fail,
  });
  const removeHoliday = useMutation({
    mutationFn: (id: string) => api.deleteHoliday(id),
    onSuccess: () => saved("Hari libur dihapus."),
    onError: fail,
  });

  function submitHoliday(e: FormEvent) {
    e.preventDefault();
    if (holidayDate === "" || holidayName.trim().length < 3) {
      setError("Isi tanggal dan nama hari libur (min 3 karakter).");
      return;
    }
    addHoliday.mutate();
  }

  if (settings.isPending) {
    return <div className="h-32 animate-pulse rounded-lg bg-lilac" aria-label="Memuat SLA" />;
  }
  if (settings.isError) {
    return (
      <div className="rounded-lg border border-mist bg-paper p-6 text-sm text-red-700">
        Gagal memuat pengaturan SLA.{" "}
        <button onClick={() => settings.refetch()} className="text-iris hover:underline">
          Coba lagi
        </button>
      </div>
    );
  }

  const { policies, calendars } = settings.data;
  const business = calendars.find((c) => !c.is_24x7);

  return (
    <div className="space-y-4 rounded-lg border border-mist bg-paper p-6">
      <div>
        <h2 className="font-semibold">SLA</h2>
        <p className="mt-1 text-xs text-veil">
          Target dalam menit (jam kerja: 1 hari kerja = 540 menit). Perubahan hanya berlaku untuk
          incident baru.{!canEdit && " Hanya Manager/Lead yang boleh mengubah."}
        </p>
      </div>
      {error !== null && (
        <p className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
      )}
      {success !== null && (
        <p className="rounded border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-700">{success}</p>
      )}
      <div className="overflow-x-auto">
        <table className="w-full min-w-[640px] text-sm">
          <thead className="text-left text-xs uppercase text-veil">
            <tr>
              <th className="py-1">Priority</th>
              <th>Response (mnt)</th>
              <th>Resolution (mnt)</th>
              <th>Kalender</th>
              <th>Peringatan (%)</th>
              <th>Pengingat breach (mnt)</th>
              {canEdit && <th />}
            </tr>
          </thead>
          <tbody>
            {policies.map((p) => (
              <PolicyRow key={`${p.priority}-${p.updated_at}`} policy={p} calendars={calendars}
                canEdit={canEdit} onSaved={saved} onError={fail} />
            ))}
          </tbody>
        </table>
      </div>
      <div className="grid gap-6 md:grid-cols-2">
        {business && (
          <HoursEditor key={JSON.stringify(business.hours)} calendar={business} canEdit={canEdit}
            onSaved={saved} onError={fail} />
        )}
        <div>
          <h3 className="font-semibold">Hari libur {year}</h3>
          <ul className="mt-2 space-y-1 text-sm">
            {(holidays.data?.data ?? []).map((h) => (
              <li key={h.id} className="flex items-center justify-between gap-2 rounded bg-chalk px-3 py-1.5">
                <span>
                  <span className="font-mono text-xs">{h.date}</span> · {h.name}
                </span>
                {canEdit && (
                  <button onClick={() => removeHoliday.mutate(h.id)} disabled={removeHoliday.isPending}
                    className="text-xs font-semibold text-veil hover:text-red-700 disabled:opacity-50"
                    aria-label={`Hapus ${h.name}`}>
                    Hapus
                  </button>
                )}
              </li>
            ))}
            {holidays.data && holidays.data.data.length === 0 && (
              <li className="text-veil">Belum ada hari libur.</li>
            )}
          </ul>
          {canEdit && (
            <form onSubmit={submitHoliday} className="mt-2 flex flex-wrap gap-2">
              <input type="date" value={holidayDate} onChange={(e) => setHolidayDate(e.target.value)}
                className="rounded border border-mist px-2 py-1 text-sm" aria-label="Tanggal libur" />
              <input value={holidayName} onChange={(e) => setHolidayName(e.target.value)} placeholder="Nama hari libur"
                className="min-w-40 flex-1 rounded border border-mist px-2 py-1 text-sm" />
              <button type="submit" disabled={addHoliday.isPending}
                className="rounded-full border border-mist px-3 py-1 text-sm font-semibold text-deep hover:bg-lilac disabled:opacity-50">
                Tambah
              </button>
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
