import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";

// Konteks alert monitoring di Incident Detail (PRD_Monitoring_Integration.md
// MON-07). Isi label berasal dari sistem luar: ditampilkan sebagai teks saja.

function fmt(iso: string): string {
  return new Date(iso).toLocaleString("id-ID");
}

export function AlertPanel({ incidentId }: { incidentId: string }) {
  const alerts = useQuery({
    queryKey: ["incident-alerts", incidentId],
    queryFn: ({ signal }) => api.getIncidentAlerts(incidentId, signal),
  });

  if (alerts.isPending) {
    return <div className="h-20 animate-pulse rounded-lg bg-lilac" aria-label="Memuat alert" />;
  }
  if (alerts.isError) {
    return (
      <div className="rounded-lg border border-mist bg-paper p-6 text-sm text-red-700">
        Gagal memuat alert.{" "}
        <button onClick={() => alerts.refetch()} className="text-iris hover:underline">
          Coba lagi
        </button>
      </div>
    );
  }
  const items = alerts.data.data;
  if (items.length === 0) return null;

  return (
    <div className="rounded-lg border border-mist bg-paper p-6">
      <h2 className="font-semibold">Alert monitoring</h2>
      <ul className="mt-2 space-y-3 text-sm">
        {items.map((a) => (
          <li key={a.id} className="rounded-lg bg-chalk p-3">
            <div className="flex flex-wrap items-center gap-2">
              <span
                className={`rounded-full border border-mist px-2 py-0.5 text-xs font-semibold ${
                  a.last_status === "firing" ? "bg-red-100 text-red-700" : "bg-green-100 text-green-700"
                }`}
              >
                {a.last_status === "firing" ? "Firing" : "Resolved"}
              </span>
              <span className="font-medium">{a.title}</span>
              <span className="text-xs text-veil">
                {a.integration_name} · {a.fire_count}× · pertama {fmt(a.first_seen)} · terakhir{" "}
                {fmt(a.last_seen)}
              </span>
              {a.source_url.startsWith("http") && (
                <a
                  href={a.source_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="ml-auto rounded-full border border-mist px-3 py-1 text-xs font-semibold text-deep hover:bg-lilac"
                >
                  Buka di monitoring
                </a>
              )}
            </div>
            <div className="mt-2 flex flex-wrap gap-1">
              {Object.entries(a.labels)
                .sort(([x], [y]) => x.localeCompare(y))
                .map(([k, v]) => (
                  <span key={k} className="rounded border border-mist bg-paper px-1.5 py-0.5 font-mono text-xs">
                    {k}={v}
                  </span>
                ))}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
