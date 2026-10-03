import { useMemo, useState } from "react";
import type { FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../api/client";
import { canCreateChange, canViewChanges, useAuth } from "../auth/AuthContext";
import { useMeta } from "../hooks/useMeta";
import { ChangeStatusBadge } from "../components/ChangeStatusBadge";
import { ChangeTypeBadge } from "../components/ChangeTypeBadge";
import { RiskBadge } from "../components/RiskBadge";

const PAGE_SIZE = 20;

const selectClass =
  "rounded border border-mist bg-paper px-2 py-1 text-sm focus:border-iris focus:outline-none";

export function ChangeList() {
  const { user } = useAuth();
  const [status, setStatus] = useState("");
  const [type, setType] = useState("");
  const [risk, setRisk] = useState("");
  const [application, setApplication] = useState("");
  const [environment, setEnvironment] = useState("");
  const [mineOnly, setMineOnly] = useState(false);
  const [searchInput, setSearchInput] = useState("");
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);

  const meta = useMeta();
  const allowed = user !== null && canViewChanges(user.role);
  const changes = useQuery({
    queryKey: ["changes", status, type, risk, application, environment, mineOnly, q, page],
    queryFn: ({ signal }) =>
      api.listChanges({
        status: status || undefined,
        type: type || undefined,
        risk: risk || undefined,
        application_id: application || undefined,
        environment: environment || undefined,
        requester: mineOnly ? "me" : undefined,
        q: q || undefined,
        page,
        limit: PAGE_SIZE,
        signal,
      }),
    enabled: allowed,
  });
  const appName = useMemo(
    () => new Map((meta.data?.applications ?? []).map((a) => [a.id, a.name] as const)),
    [meta.data],
  );

  if (!allowed) {
    return (
      <section className="w-full p-6">
        <p className="rounded-lg border border-mist bg-paper p-6 text-sm text-veil">
          Anda tidak berhak mengakses Change Management.
        </p>
      </section>
    );
  }

  const canCreate = canCreateChange(user?.role ?? "");

  function filter(setter: (v: string) => void) {
    return (v: string) => {
      setter(v);
      setPage(1);
    };
  }

  function applySearch(e: FormEvent) {
    e.preventDefault();
    setQ(searchInput.trim());
    setPage(1);
  }

  const total = changes.data?.meta.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const createButton = canCreate && (
    <Link
      to="/changes/new"
      className="rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95"
    >
      Buat Change Request
    </Link>
  );

  const selects: {
    label: string;
    value: string;
    set: (v: string) => void;
    all: string;
    options: { value: string; label: string }[];
  }[] = [
    {
      label: "Filter status",
      value: status,
      set: filter(setStatus),
      all: "Semua status",
      options: (meta.data?.change_statuses ?? []).map((s) => ({ value: s.code, label: s.code })),
    },
    {
      label: "Filter type",
      value: type,
      set: filter(setType),
      all: "Semua type",
      options: (meta.data?.change_types ?? []).map((s) => ({ value: s.code, label: s.name })),
    },
    {
      label: "Filter risk",
      value: risk,
      set: filter(setRisk),
      all: "Semua risk",
      options: (meta.data?.change_risks ?? []).map((s) => ({ value: s.code, label: s.name })),
    },
    {
      label: "Filter aplikasi",
      value: application,
      set: filter(setApplication),
      all: "Semua aplikasi",
      options: (meta.data?.applications ?? []).map((a) => ({ value: a.id, label: a.name })),
    },
    {
      label: "Filter environment",
      value: environment,
      set: filter(setEnvironment),
      all: "Semua environment",
      options: (meta.data?.environments ?? []).map((e) => ({ value: e.code, label: e.name })),
    },
  ];

  return (
    <section className="w-full p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Change Requests</h1>
        {createButton}
      </div>

      <form onSubmit={applySearch} className="mt-4 flex flex-wrap gap-2">
        {selects.map((s) => (
          <select
            key={s.label}
            value={s.value}
            onChange={(e) => s.set(e.target.value)}
            className={selectClass}
            aria-label={s.label}
          >
            <option value="">{s.all}</option>
            {s.options.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        ))}
        <label className="flex items-center gap-1 text-sm">
          <input
            type="checkbox"
            checked={mineOnly}
            onChange={(e) => {
              setMineOnly(e.target.checked);
              setPage(1);
            }}
          />
          Pengajuan saya
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
        {changes.isPending && (
          <div className="space-y-2" aria-label="Memuat">
            {[0, 1, 2].map((i) => (
              <div key={i} className="h-16 animate-pulse rounded bg-lilac" />
            ))}
          </div>
        )}
        {changes.isError && (
          <div className="rounded border border-red-200 bg-red-50 p-4 text-sm">
            <p className="text-red-700">Gagal memuat daftar change.</p>
            <button
              onClick={() => changes.refetch()}
              className="mt-2 rounded-full border border-mist px-3 py-1 text-deep hover:bg-lilac"
            >
              Coba lagi
            </button>
          </div>
        )}
        {changes.data && changes.data.data.length === 0 && (
          <div className="rounded-lg border border-mist bg-paper p-8 text-center">
            <p className="text-veil">Belum ada change request yang cocok.</p>
            {createButton && <div className="mt-4">{createButton}</div>}
          </div>
        )}
        {changes.data && changes.data.data.length > 0 && (
          <>
            <ul className="divide-y divide-mist rounded-lg border border-mist bg-paper">
              {changes.data.data.map((c) => (
                <li key={c.id}>
                  <Link to={`/changes/${c.id}`} className="block p-4 hover:bg-chalk">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-mono text-xs text-veil">{c.change_no}</span>
                      <ChangeStatusBadge status={c.status} />
                      <ChangeTypeBadge type={c.type} />
                      <RiskBadge risk={c.risk} />
                    </div>
                    <p className="mt-1 font-medium">{c.title}</p>
                    <p className="mt-1 text-xs text-veil">
                      {appName.get(c.application_id) ?? "—"} · {c.environment}
                    </p>
                  </Link>
                </li>
              ))}
            </ul>
            <div className="mt-3 flex items-center justify-between text-sm">
              <span className="text-veil">
                Hal {page} dari {totalPages} · {total} change
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
