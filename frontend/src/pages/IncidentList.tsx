import { useState } from "react";
import type { FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../api/client";
import { useMeta } from "../hooks/useMeta";
import { PriorityBadge } from "../components/PriorityBadge";
import { SeverityBadge } from "../components/SeverityBadge";
import { StatusBadge } from "../components/StatusBadge";

const PAGE_SIZE = 20;

function selectClass() {
  return "rounded border px-2 py-1 text-sm";
}

export function IncidentList() {
  const [status, setStatus] = useState("");
  const [severity, setSeverity] = useState("");
  const [priority, setPriority] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);

  const meta = useMeta();
  const incidents = useQuery({
    queryKey: ["incidents", status, severity, priority, q, page],
    queryFn: ({ signal }) =>
      api.listIncidents({
        status: status || undefined,
        severity: severity || undefined,
        priority: priority || undefined,
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
    <section className="mx-auto max-w-5xl p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Incidents</h1>
        <Link
          to="/incidents/new"
          className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
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
        <input
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder="Cari judul / nomor…"
          className="min-w-52 flex-1 rounded border px-3 py-1 text-sm"
        />
        <button
          type="submit"
          className="rounded border px-3 py-1 text-sm hover:bg-slate-100"
        >
          Cari
        </button>
      </form>

      <div className="mt-4">
        {incidents.isPending && (
          <div className="space-y-2" aria-label="Memuat">
            {[0, 1, 2].map((i) => (
              <div key={i} className="h-16 animate-pulse rounded bg-slate-200" />
            ))}
          </div>
        )}
        {incidents.isError && (
          <div className="rounded border border-red-200 bg-red-50 p-4 text-sm">
            <p className="text-red-700">Gagal memuat daftar incident.</p>
            <button
              onClick={() => incidents.refetch()}
              className="mt-2 rounded border px-3 py-1 hover:bg-white"
            >
              Coba lagi
            </button>
          </div>
        )}
        {incidents.data && incidents.data.data.length === 0 && (
          <div className="rounded border border-dashed p-8 text-center">
            <p className="text-slate-600">Belum ada incident yang cocok.</p>
            <Link
              to="/incidents/new"
              className="mt-4 inline-block rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
            >
              Buat Incident
            </Link>
          </div>
        )}
        {incidents.data && incidents.data.data.length > 0 && (
          <>
            <ul className="divide-y rounded border bg-white">
              {incidents.data.data.map((in_) => (
                <li key={in_.id}>
                  <Link to={`/incidents/${in_.id}`} className="block p-4 hover:bg-slate-50">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-mono text-xs text-slate-500">
                        {in_.incident_no}
                      </span>
                      <StatusBadge status={in_.status} />
                      <SeverityBadge severity={in_.severity} />
                      <PriorityBadge priority={in_.priority} />
                    </div>
                    <p className="mt-1 font-medium">{in_.title}</p>
                  </Link>
                </li>
              ))}
            </ul>
            <div className="mt-3 flex items-center justify-between text-sm">
              <span className="text-slate-500">
                Hal {page} dari {totalPages} · {total} incident
              </span>
              <div className="flex gap-2">
                <button
                  disabled={page <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  className="rounded border px-3 py-1 disabled:opacity-40"
                >
                  ← Sebelumnya
                </button>
                <button
                  disabled={page >= totalPages}
                  onClick={() => setPage((p) => p + 1)}
                  className="rounded border px-3 py-1 disabled:opacity-40"
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
