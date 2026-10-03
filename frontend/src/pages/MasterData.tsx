import { useState } from "react";
import type { FormEvent } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ApiError, api } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { useMeta } from "../hooks/useMeta";
import { SlaSettings } from "../components/SlaSettings";

export function MasterData() {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const meta = useMeta();
  const [appCode, setAppCode] = useState("");
  const [appName, setAppName] = useState("");
  const [teamCode, setTeamCode] = useState("");
  const [teamName, setTeamName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const isManager = user?.role === "ManagerLead";
  const isCoordinator =
    user?.role === "HelpDesk" ||
    user?.role === "SystemAnalyst" ||
    user?.role === "ManagerLead";

  function fail(err: unknown) {
    setSuccess(null);
    setError(err instanceof ApiError ? err.message : "Aksi gagal. Coba lagi.");
  }

  const createApp = useMutation({
    mutationFn: () => api.createApplication({ code: appCode.trim(), name: appName.trim() }),
    onSuccess: () => {
      setAppCode("");
      setAppName("");
      setError(null);
      setSuccess("Application ditambahkan.");
      queryClient.invalidateQueries({ queryKey: ["meta"] });
    },
    onError: fail,
  });
  const createTeam = useMutation({
    mutationFn: () => api.createTeam({ code: teamCode.trim(), name: teamName.trim() }),
    onSuccess: () => {
      setTeamCode("");
      setTeamName("");
      setError(null);
      setSuccess("Team ditambahkan.");
      queryClient.invalidateQueries({ queryKey: ["meta"] });
    },
    onError: fail,
  });
  const deleteApp = useMutation({
    mutationFn: (id: string) => api.deleteApplication(id),
    onSuccess: () => {
      setError(null);
      setSuccess("Application dihapus.");
      queryClient.invalidateQueries({ queryKey: ["meta"] });
    },
    onError: fail,
  });
  const deleteTeam = useMutation({
    mutationFn: (id: string) => api.deleteTeam(id),
    onSuccess: () => {
      setError(null);
      setSuccess("Team dihapus.");
      queryClient.invalidateQueries({ queryKey: ["meta"] });
    },
    onError: fail,
  });

  if (!isCoordinator) {
    return (
      <section className="w-full p-6">
        <h1 className="text-2xl font-semibold">Master Data</h1>
        <p className="mt-4 text-sm text-red-700">
          Hanya koordinator yang boleh mengakses halaman ini.
        </p>
      </section>
    );
  }

  function submitApp(e: FormEvent) {
    e.preventDefault();
    if (!appCode.trim() || !appName.trim()) {
      setError("Code dan name aplikasi wajib diisi.");
      return;
    }
    createApp.mutate();
  }

  function submitTeam(e: FormEvent) {
    e.preventDefault();
    if (!teamCode.trim() || !teamName.trim()) {
      setError("Code dan name team wajib diisi.");
      return;
    }
    createTeam.mutate();
  }

  const inputClass =
    "rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none";

  return (
    <section className="w-full space-y-6 p-6">
      <h1 className="text-2xl font-semibold">Master Data</h1>
      {error !== null && (
        <p className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
      )}
      {success !== null && (
        <p className="rounded border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-700">{success}</p>
      )}

      <div className="rounded-lg border border-mist bg-paper p-6">
        <h2 className="font-semibold">Applications</h2>
        <ul className="mt-2 divide-y text-sm">
          {(meta.data?.applications ?? []).map((a) => (
            <li key={a.id} className="flex items-center justify-between py-1">
              <span>
                {a.name} <span className="font-mono text-xs text-veil">{a.code}</span>
              </span>
              {isManager && (
                <button
                  onClick={() => deleteApp.mutate(a.id)}
                  disabled={deleteApp.isPending}
                  className="rounded border px-2 py-1 text-xs text-red-600 hover:bg-red-50 disabled:opacity-50"
                >
                  Hapus
                </button>
              )}
            </li>
          ))}
        </ul>
        <form onSubmit={submitApp} className="mt-3 flex gap-2">
          <input
            value={appCode}
            onChange={(e) => setAppCode(e.target.value)}
            placeholder="code"
            className={`w-28 ${inputClass}`}
          />
          <input
            value={appName}
            onChange={(e) => setAppName(e.target.value)}
            placeholder="Nama aplikasi"
            className={`flex-1 ${inputClass}`}
          />
          <button
            type="submit"
            disabled={createApp.isPending}
            className="rounded-full bg-iris px-4 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50"
          >
            Tambah
          </button>
        </form>
      </div>

      <div className="rounded-lg border border-mist bg-paper p-6">
        <h2 className="font-semibold">Teams</h2>
        <ul className="mt-2 divide-y text-sm">
          {(meta.data?.teams ?? []).map((t) => (
            <li key={t.id} className="flex items-center justify-between py-1">
              <span>
                {t.name} <span className="font-mono text-xs text-veil">{t.code}</span>
              </span>
              {isManager && (
                <button
                  onClick={() => deleteTeam.mutate(t.id)}
                  disabled={deleteTeam.isPending}
                  className="rounded border px-2 py-1 text-xs text-red-600 hover:bg-red-50 disabled:opacity-50"
                >
                  Hapus
                </button>
              )}
            </li>
          ))}
        </ul>
        <form onSubmit={submitTeam} className="mt-3 flex gap-2">
          <input
            value={teamCode}
            onChange={(e) => setTeamCode(e.target.value)}
            placeholder="code"
            className={`w-28 ${inputClass}`}
          />
          <input
            value={teamName}
            onChange={(e) => setTeamName(e.target.value)}
            placeholder="Nama team"
            className={`flex-1 ${inputClass}`}
          />
          <button
            type="submit"
            disabled={createTeam.isPending}
            className="rounded-full bg-iris px-4 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50"
          >
            Tambah
          </button>
        </form>
        {!isManager && (
          <p className="mt-2 text-xs text-veil">
            Hapus hanya untuk ManagerLead; item terpakai tidak bisa dihapus.
          </p>
        )}
      </div>

      <SlaSettings canEdit={isManager} />
    </section>
  );
}
