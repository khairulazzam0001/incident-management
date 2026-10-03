import { useState } from "react";
import type { FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../api/client";
import { useMeta } from "../hooks/useMeta";
import { PriorityBadge } from "../components/PriorityBadge";
import { SeverityBadge } from "../components/SeverityBadge";
import { SlaBadge } from "../components/SlaBadge";
import { StatusBadge } from "../components/StatusBadge";

const PAGE_SIZE = 20;

function selectClass() {
  return "rounded border border-mist bg-paper px-2 py-1 text-sm focus:border-iris focus:outline-none";
}

export function IncidentList() {
  const [status, setStatus] = useState("");
  const [severity, setSeverity] = useState("");
  const [priority, setPriority] = useState("");
  const [application, setApplication] = useState("");
  const [team, setTeam] = useState("");
  const [mineOnly, setMineOnly] = useState(false);
  const [slaFilter, setSlaFilter] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);

  const meta = useMeta();
  const incidents = useQuery({
    queryKey: ["incidents", status, severity, priority, application, team, mineOnly, slaFilter, q, page],
    queryFn: ({ signal }) =>
      api.listIncidents({
        status: status || undefined,
        severity: severity || undefined,
        priority: priority || undefined,
        application: application || undefined,
        team: team || undefined,
        assignee: mineOnly ? "me" : undefined,
        sla: slaFilter || undefined,
        q: q || undefined,
        page,
        limit: PAGE_SIZE,
        signal,
      }),
  });

  function resetPage() {
    setPage(1);
  }

  function applySearch(e: FormEvent) {
    e.preventDefault();
    setQ(searchInput.trim());
    resetPage();
  }

  const total = incidents.data?.meta.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <section className="w-full p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Incidents</h1>
        <Link
          to="/incidents/new"
          className="rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95"
        >
          Buat Incident
        </Link>
      </div>

      <form onSubmit={applySearch} className="mt-4 flex flex-wrap gap-2">
        <select
          value={status}
          onChange={(e) => {
            setStatus(e.target.value);
            resetPage();
          }}
          className={selectClass()}
          aria-label="Filter status"
        >
          <option value="">Semua status</option>
          {(meta.data?.statuses ?? []).map((s) => (
            <option key={s.code} value={s.code}>
              {s.code}
            </option>
          ))}
        </select>
        <select
          value={severity}
          onChange={(e) => {
            setSeverity(e.target.value);
            resetPage();
          }}
          className={selectClass()}
          aria-label="Filter severity"
        >
          <option value="">Semua severity</option>
          {(meta.data?.severities ?? []).map((s) => (
            <option key={s.code} value={s.code}>
              {s.code}
            </option>
          ))}
        </select>
        <select
          value={priority}
          onChange={(e) => {
            setPriority(e.target.value);
            resetPage();
          }}
          className={selectClass()}
          aria-label="Filter priority"
        >
          <option value="">Semua priority</option>
          {(meta.data?.priorities ?? []).map((p) => (
            <option key={p.code} value={p.code}>
              {p.code}
            </option>
          ))}
        </select>
        <select
          value={application}
          onChange={(e) => {
            setApplication(e.target.value);
            resetPage();
          }}
          className={selectClass()}
          aria-label="Filter aplikasi"
        >
          <option value="">Semua aplikasi</option>
          {(meta.data?.applications ?? []).map((a) => (
            <option key={a.id} value={a.id}>
              {a.name}
            </option>
          ))}
        </select>
        <select
          value={team}
          onChange={(e) => {
            setTeam(e.target.value);
            resetPage();
          }}
          className={selectClass()}
          aria-label="Filter team"
        >
          <option value="">Semua team</option>
          {(meta.data?.teams ?? []).map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </select>
        <select
          value={slaFilter}
          onChange={(e) => {
            setSlaFilter(e.target.value);
            resetPage();
          }}
          className={selectClass()}
          aria-label="Filter SLA"
        >
          <option value="">Semua SLA</option>
          <option value="at_risk">Hampir breach</option>
          <option value="breached">Breach</option>
        </select>
        <label className="flex items-center gap-1 text-sm">
          <input
            type="checkbox"
            checked={mineOnly}
            onChange={(e) => {
              setMineOnly(e.target.checked);
              resetPage();
            }}
          />
          Milik saya
        </label>
        <input
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder="Cari judul / nomor…"
          className="min-w-52 flex-1 rounded-full border border-mist bg-paper px-4 py-1 text-sm focus:border-iris focus:outline-none"
        />
        <button
          type="submit"
          className="rounded-full border border-mist px-4 py-1 text-sm font-semibold text-deep hover:bg-lilac"
        >
          Cari
        </button>
      </form>

      <div className="mt-4">
        {incidents.isPending && (
          <div className="space-y-2" aria-label="Memuat">
            {[0, 1, 2].map((i) => (
              <div key={i} className="h-16 animate-pulse rounded bg-lilac" />
            ))}
          </div>
        )}
        {incidents.isError && (
          <div className="rounded border border-red-200 bg-red-50 p-4 text-sm">
            <p className="text-red-700">Gagal memuat daftar incident.</p>
            <button
              onClick={() => incidents.refetch()}
              className="mt-2 rounded-full border border-mist px-3 py-1 text-deep hover:bg-lilac"
            >
              Coba lagi
            </button>
          </div>
        )}
        {incidents.data && incidents.data.data.length === 0 && (
          <div className="rounded-lg border border-mist bg-paper p-8 text-center">
            <p className="text-veil">Belum ada incident yang cocok.</p>
            <Link
              to="/incidents/new"
              className="mt-4 inline-block rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95"
            >
              Buat Incident
            </Link>
          </div>
        )}
        {incidents.data && incidents.data.data.length > 0 && (
          <>
            <ul className="divide-y divide-mist rounded-lg border border-mist bg-paper">
              {incidents.data.data.map((in_) => (
                <li key={in_.id}>
                  <Link to={`/incidents/${in_.id}`} className="block p-4 hover:bg-chalk">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-mono text-xs text-veil">
                        {in_.incident_no}
                      </span>
                      <StatusBadge status={in_.status} />
                      <SeverityBadge severity={in_.severity} />
                      <PriorityBadge priority={in_.priority} />
                      <SlaBadge sla={in_.sla ?? null} />
                    </div>
                    <p className="mt-1 font-medium">{in_.title}</p>
                  </Link>
                </li>
              ))}
            </ul>
            <div className="mt-3 flex items-center justify-between text-sm">
              <span className="text-veil">
                Hal {page} dari {totalPages} · {total} incident
              </span>
              <div className="flex gap-2">
                <button
                  disabled={page <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  className="rounded-full border border-mist px-3 py-1 text-deep hover:bg-lilac disabled:opacity-40"
                >
                  ← Sebelumnya
                </button>
                <button
                  disabled={page >= totalPages}
                  onClick={() => setPage((p) => p + 1)}
                  className="rounded-full border border-mist px-3 py-1 text-deep hover:bg-lilac disabled:opacity-40"
                >
                  Berikutnya →
                </button>
              </div>
            </div>
          </>
        )}
      </div>
    </section>
  );
}
