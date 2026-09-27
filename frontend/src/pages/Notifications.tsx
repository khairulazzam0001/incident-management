import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { ApiError, api } from "../api/client";

const TYPE_LABELS: Record<string, string> = {
  created: "Incident dibuat",
  assigned: "Anda ditugaskan",
  status_changed: "Status berubah",
  comment: "Komentar baru",
  verification_failed: "Verification FAIL",
  resolved: "Incident resolved",
  closed: "Incident closed",
};

export function Notifications() {
  const queryClient = useQueryClient();
  const [showUnreadOnly, setShowUnreadOnly] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const list = useQuery({
    queryKey: ["notifications", showUnreadOnly],
    queryFn: ({ signal }) => api.getNotifications(showUnreadOnly, signal),
  });

  const markRead = useMutation({
    mutationFn: (id: string) => api.markNotificationRead(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["notifications"] });
    },
    onError: (err: unknown) => {
      setError(err instanceof ApiError ? err.message : "Gagal menandai dibaca.");
    },
  });

  return (
    <section className="w-full p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Notifikasi</h1>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={showUnreadOnly}
            onChange={(e) => setShowUnreadOnly(e.target.checked)}
          />
          Belum dibaca saja
        </label>
      </div>
      {error !== null && (
        <p className="mt-3 rounded bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
      )}
      <div className="mt-4">
        {list.isPending && (
          <div className="space-y-2" aria-label="Memuat">
            {[0, 1].map((i) => (
              <div key={i} className="h-14 animate-pulse rounded bg-lilac" />
            ))}
          </div>
        )}
        {list.isError && (
          <div className="rounded border border-red-200 bg-red-50 p-4 text-sm">
            <p className="text-red-700">Gagal memuat notifikasi.</p>
            <button
              onClick={() => list.refetch()}
              className="mt-2 rounded-full border border-mist px-3 py-1 text-deep hover:bg-lilac"
            >
              Coba lagi
            </button>
          </div>
        )}
        {list.data && list.data.data.length === 0 && (
          <p className="rounded border border-dashed p-6 text-center text-sm text-veil">
            Tidak ada notifikasi.
          </p>
        )}
        {list.data && list.data.data.length > 0 && (
          <ul className="space-y-2">
            {list.data.data.map((n) => (
              <li
                key={n.id}
                className={`flex items-center justify-between gap-3 rounded-lg border bg-paper p-3 text-sm ${
                  n.read_at === null ? "border-iris" : "border-mist"
                }`}
              >
                <div>
                  <p className="font-medium">
                    {TYPE_LABELS[n.type] ?? n.type}
                    {n.read_at === null && (
                      <span className="ml-2 rounded-full bg-lilac px-1.5 text-xs font-semibold text-iris">
                        baru
                      </span>
                    )}
                  </p>
                  <Link
                    to={`/incidents/${n.incident_id}`}
                    className="text-iris hover:underline"
                  >
                    Lihat incident →
                  </Link>
                </div>
                {n.read_at === null && (
                  <button
                    onClick={() => markRead.mutate(n.id)}
                    disabled={markRead.isPending}
                    className="shrink-0 rounded-full border border-mist px-2 py-1 text-xs font-semibold text-deep hover:bg-lilac disabled:opacity-50"
                  >
                    Tandai dibaca
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
