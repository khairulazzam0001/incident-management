import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { ApiError, api } from "../api/client";
import type { ChangeInput } from "../api/types";
import { canCreateChange, useAuth } from "../auth/AuthContext";
import { ChangeForm, EMPTY_CHANGE } from "../components/ChangeForm";

export function ChangeNew() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const [params] = useSearchParams();
  const incidentId = params.get("incident") ?? "";
  const [error, setError] = useState<string | null>(null);

  // Pre-fill dari incident (relasi FIX_FOR, PRD_Change_Management.md §8.1).
  const incidentQuery = useQuery({
    queryKey: ["incident", incidentId],
    queryFn: ({ signal }) => api.getIncident(incidentId, signal),
    enabled: incidentId !== "",
  });

  const create = useMutation({
    mutationFn: async ({ input, submitAfter }: { input: ChangeInput; submitAfter: boolean }) => {
      const created = await api.createChange({
        ...input,
        incident_id: incidentQuery.data?.id ?? null,
      });
      if (!submitAfter) return { change: created, notice: "Change request tersimpan sebagai DRAFT." };
      try {
        const submitted = await api.submitChange(created.id);
        return {
          change: submitted,
          notice:
            submitted.status === "APPROVED"
              ? "Standard change tersimpan & otomatis APPROVED."
              : "Change request disubmit, menunggu approval.",
        };
      } catch (err) {
        const reason = err instanceof ApiError ? err.message : "Submit gagal.";
        return { change: created, notice: `Tersimpan sebagai DRAFT, tetapi submit gagal: ${reason}` };
      }
    },
    onSuccess: ({ change, notice }) => navigate(`/changes/${change.id}`, { state: { notice } }),
    onError: (err: unknown) => {
      setError(err instanceof ApiError ? err.message : "Gagal membuat change request.");
    },
  });

  if (user !== null && !canCreateChange(user.role)) {
    return (
      <section className="w-full p-6">
        <p className="rounded-lg border border-mist bg-paper p-6 text-sm text-veil">
          Role Anda tidak boleh membuat change request.
        </p>
      </section>
    );
  }

  if (incidentId !== "" && incidentQuery.isPending) {
    return (
      <section className="w-full p-6" aria-label="Memuat">
        <div className="h-8 w-1/3 animate-pulse rounded bg-lilac" />
        <div className="mt-6 h-64 animate-pulse rounded bg-lilac" />
      </section>
    );
  }

  const incident = incidentQuery.data;
  const initial: ChangeInput = incident
    ? {
        ...EMPTY_CHANGE,
        title: `Fix ${incident.incident_no}: ${incident.title}`,
        justification: `Perbaikan untuk ${incident.incident_no} (${incident.severity}/${incident.priority}).`,
        type: incident.severity === "S1" || incident.priority === "P1" ? "EMERGENCY" : "NORMAL",
        application_id: incident.application_id ?? "",
        environment: incident.environment ?? "",
      }
    : EMPTY_CHANGE;

  return (
    <section className="w-full p-6">
      <Link to="/changes" className="text-sm text-iris hover:underline">
        ← Kembali ke daftar change
      </Link>
      <h1 className="mt-2 text-2xl font-semibold">Buat Change Request</h1>
      {incident && (
        <p className="mt-2 rounded-lg border border-mist bg-lilac px-3 py-2 text-sm text-deep">
          Ditautkan sebagai <span className="font-semibold">FIX_FOR</span>{" "}
          <Link to={`/incidents/${incident.id}`} className="font-mono text-iris hover:underline">
            {incident.incident_no}
          </Link>
        </p>
      )}
      {incidentQuery.isError && (
        <p className="mt-2 rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          Incident sumber tidak bisa dimuat; change dibuat tanpa link.
        </p>
      )}
      <ChangeForm
        key={incident?.id ?? "blank"}
        initial={initial}
        pending={create.isPending}
        error={error}
        submitLabel="Simpan Draft"
        onSave={(input, submitAfter) => {
          setError(null);
          create.mutate({ input, submitAfter });
        }}
        onCancel={() => navigate(incident ? `/incidents/${incident.id}` : "/changes")}
      />
    </section>
  );
}
