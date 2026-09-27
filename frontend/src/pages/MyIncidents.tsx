import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../api/client";
import { PriorityBadge } from "../components/PriorityBadge";
import { SeverityBadge } from "../components/SeverityBadge";
import { StatusBadge } from "../components/StatusBadge";

const PAGE_SIZE = 20;

export function MyIncidents() {
  const [page, setPage] = useState(1);
  const incidents = useQuery({
    queryKey: ["incidents", "mine", page],
    queryFn: ({ signal }) =>
      api.listIncidents({ assignee: "me", page, limit: PAGE_SIZE, signal }),
  });

  const total = incidents.data?.meta.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <section className="w-full p-6">
      <h1 className="text-2xl font-semibold">My Incidents</h1>
      <p className="mt-1 text-sm text-veil">
        Incident yang ditugaskan kepada saya sebagai PIC.
      </p>
      <div className="mt-4">
        {incidents.isPending && (
          <div className="space-y-2" aria-label="Memuat">
            {[0, 1].map((i) => (
              <div key={i} className="h-16 animate-pulse rounded bg-lilac" />
            ))}
          </div>
        )}
        {incidents.isError && (
          <div className="rounded border border-red-200 bg-red-50 p-4 text-sm">
            <p className="text-red-700">Gagal memuat.</p>
            <button
              onClick={() => incidents.refetch()}
              className="mt-2 rounded border px-3 py-1 hover:bg-white"
            >
              Coba lagi
            </button>
          </div>
        )}
        {incidents.data && incidents.data.data.length === 0 && (
          <div className="rounded-lg border border-mist bg-paper p-8 text-center text-sm text-veil">
            Tidak ada incident yang ditugaskan kepada Anda.
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
