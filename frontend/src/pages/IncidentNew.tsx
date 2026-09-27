import { useState } from "react";
import type { FormEvent } from "react";
import { useMutation } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { ApiError, api } from "../api/client";
import type { Priority, Severity } from "../api/types";
import { useMeta } from "../hooks/useMeta";

export function IncidentNew() {
  const navigate = useNavigate();
  const meta = useMeta();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [severity, setSeverity] = useState<Severity | "">("");
  const [priority, setPriority] = useState<Priority>("P3");
  const [source, setSource] = useState("");
  const [applicationId, setApplicationId] = useState("");
  const [environment, setEnvironment] = useState("");
  const [error, setError] = useState<string | null>(null);

  const create = useMutation({
    mutationFn: () =>
      api.createIncident({
        title: title.trim(),
        description,
        source,
        severity,
        priority,
        application_id: applicationId || null,
        environment: environment || null,
      }),
    onSuccess: (incident) => navigate(`/incidents/${incident.id}`),
    onError: (err: unknown) => {
      setError(err instanceof ApiError ? err.message : "Gagal membuat incident.");
    },
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    if (title.trim().length < 5) {
      setError("Title minimal 5 karakter.");
      return;
    }
    if (severity === "") {
      setError("Severity wajib dipilih.");
      return;
    }
    if (source === "") {
      setError("Source wajib dipilih.");
      return;
    }
    if (environment === "") {
      setError("Environment wajib dipilih.");
      return;
    }
    if (applicationId === "") {
      setError("Application wajib dipilih.");
      return;
    }
    create.mutate();
  }

  const inputClass =
    "mt-1 w-full rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none";

  return (
    <section className="w-full p-6">
      <h1 className="text-2xl font-semibold">Buat Incident</h1>
      <form onSubmit={onSubmit} className="mt-6 space-y-4">
        <label className="block">
          <span className="text-sm font-medium">Title *</span>
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className={inputClass}
            placeholder="Cth: Checkout gagal 500"
          />
        </label>
        <label className="block">
          <span className="text-sm font-medium">Deskripsi</span>
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className={inputClass}
            rows={4}
          />
        </label>
        <div className="grid grid-cols-2 gap-4">
          <label className="block">
            <span className="text-sm font-medium">Severity *</span>
            <select
              value={severity}
              onChange={(e) => setSeverity(e.target.value as Severity | "")}
              className={inputClass}
            >
              <option value="">— pilih —</option>
              {(meta.data?.severities ?? []).map((s) => (
                <option key={s.code} value={s.code}>
                  {s.code} — {s.name}
                </option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="text-sm font-medium">Priority</span>
            <select
              value={priority}
              onChange={(e) => setPriority(e.target.value as Priority)}
              className={inputClass}
            >
              {(meta.data?.priorities ?? []).map((p) => (
                <option key={p.code} value={p.code}>
                  {p.code} — {p.name}
                </option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="text-sm font-medium">Source *</span>
            <select
              value={source}
              onChange={(e) => setSource(e.target.value)}
              className={inputClass}
            >
              <option value="">Silahkan pilih</option>
              {(meta.data?.sources ?? []).map((s) => (
                <option key={s.code} value={s.code}>
                  {s.name}
                </option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="text-sm font-medium">Environment *</span>
            <select
              value={environment}
              onChange={(e) => setEnvironment(e.target.value)}
              className={inputClass}
            >
              <option value="">Silahkan pilih</option>
              {(meta.data?.environments ?? []).map((s) => (
                <option key={s.code} value={s.code}>
                  {s.name}
                </option>
              ))}
            </select>
          </label>
        </div>
        <label className="block">
          <span className="text-sm font-medium">Application *</span>
          <select
            value={applicationId}
            onChange={(e) => setApplicationId(e.target.value)}
            className={inputClass}
          >
            <option value="">Silahkan pilih</option>
            {(meta.data?.applications ?? []).map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </select>
        </label>
        {error !== null && (
          <p className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
        )}
        <div className="flex gap-2">
          <button
            type="submit"
            disabled={create.isPending}
            className="rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50"
          >
            {create.isPending ? "Menyimpan…" : "Buat Incident"}
          </button>
          <Link
            to="/"
            className="rounded-full border border-mist px-4 py-2 text-sm font-semibold text-deep hover:bg-lilac"
          >
            Batal
          </Link>
        </div>
      </form>
    </section>
  );
}
