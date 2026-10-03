import { useState } from "react";
import type { FormEvent } from "react";
import { useMutation } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../api/client";
import type { Change, ChangeOutcome, User } from "../api/types";

// Aksi lifecycle CM-2 (PRD_Change_Management.md §7, §9): schedule, start,
// complete, close. Aturan peran untuk UX saja; backend yang menegakkan.

function toLocalInput(iso: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function formatTime(iso: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString("id-ID");
}

const inputClass =
  "mt-1 w-full rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none";
const primaryButton =
  "rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50";

export function ChangeLifecyclePanel({
  change,
  user,
  onDone,
  onError,
}: {
  change: Change;
  user: User | null;
  onDone: (message: string) => void;
  onError: (err: unknown) => void;
}) {
  const [start, setStart] = useState(toLocalInput(change.planned_start));
  const [end, setEnd] = useState(toLocalInput(change.planned_end));
  const [conflicts, setConflicts] = useState<Change[]>([]);
  const [outcome, setOutcome] = useState<ChangeOutcome>("SUCCESS");
  const [outcomeNotes, setOutcomeNotes] = useState("");
  const [reviewNotes, setReviewNotes] = useState("");
  const [localError, setLocalError] = useState<string | null>(null);

  const schedule = useMutation({
    mutationFn: () =>
      api.scheduleChange(change.id, new Date(start).toISOString(), new Date(end).toISOString()),
    onSuccess: (res) => {
      setConflicts(res.conflicts);
      onDone(
        res.conflicts.length > 0
          ? `Change dijadwalkan, tetapi bentrok dengan ${res.conflicts.length} change lain.`
          : "Change dijadwalkan.",
      );
    },
    onError,
  });
  const startMutation = useMutation({
    mutationFn: () => api.startChange(change.id),
    onSuccess: () => onDone("Implementasi dimulai."),
    onError,
  });
  const complete = useMutation({
    mutationFn: () => api.completeChange(change.id, outcome, outcomeNotes.trim()),
    onSuccess: () => {
      setOutcomeNotes("");
      onDone("Implementasi selesai — menunggu review.");
    },
    onError,
  });
  const close = useMutation({
    mutationFn: () => api.closeChange(change.id, reviewNotes.trim()),
    onSuccess: () => {
      setReviewNotes("");
      onDone("Change CLOSED.");
    },
    onError,
  });

  if (user === null) return null;
  const isManager = user.role === "ManagerLead";
  const isImplementer =
    isManager ||
    (change.implementer_id !== null
      ? change.implementer_id === user.id
      : change.requester_id === user.id);
  const canSchedule =
    isManager || change.requester_id === user.id || change.implementer_id === user.id;
  const pirRequired = change.type === "EMERGENCY" || change.outcome !== "SUCCESS";
  const pending =
    schedule.isPending || startMutation.isPending || complete.isPending || close.isPending;

  const showSchedule =
    canSchedule && (change.status === "APPROVED" || change.status === "SCHEDULED");
  const showStart =
    isImplementer &&
    (change.status === "SCHEDULED" ||
      (change.status === "APPROVED" && change.type === "EMERGENCY"));
  const showComplete = isImplementer && change.status === "IMPLEMENTING";
  const showClose =
    change.status === "REVIEWING" && (isManager || user.role === "SystemAnalyst");

  if (!showSchedule && !showStart && !showComplete && !showClose) return null;

  function submitSchedule(e: FormEvent) {
    e.preventDefault();
    if (start === "" || end === "") {
      setLocalError("Isi waktu mulai dan selesai.");
      return;
    }
    if (new Date(end) <= new Date(start)) {
      setLocalError("Waktu selesai harus setelah waktu mulai.");
      return;
    }
    setLocalError(null);
    schedule.mutate();
  }

  function submitComplete(e: FormEvent) {
    e.preventDefault();
    if (outcome !== "SUCCESS" && outcomeNotes.trim() === "") {
      setLocalError("Outcome notes wajib untuk FAILED / ROLLED_BACK.");
      return;
    }
    setLocalError(null);
    complete.mutate();
  }

  function submitClose(e: FormEvent) {
    e.preventDefault();
    if (pirRequired && reviewNotes.trim() === "") {
      setLocalError("Post-implementation review wajib untuk change ini.");
      return;
    }
    setLocalError(null);
    close.mutate();
  }

  return (
    <div className="space-y-4 rounded-lg border border-mist bg-paper p-6">
      <h2 className="font-semibold">Implementasi</h2>
      {localError !== null && (
        <p className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{localError}</p>
      )}

      {showSchedule && (
        <form onSubmit={submitSchedule} className="space-y-2">
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <label className="block text-sm">
              Mulai
              <input
                type="datetime-local"
                value={start}
                onChange={(e) => setStart(e.target.value)}
                className={inputClass}
              />
            </label>
            <label className="block text-sm">
              Selesai
              <input
                type="datetime-local"
                value={end}
                onChange={(e) => setEnd(e.target.value)}
                className={inputClass}
              />
            </label>
          </div>
          <button type="submit" disabled={pending} className={primaryButton}>
            {schedule.isPending
              ? "…"
              : change.status === "SCHEDULED"
                ? "Ubah Jadwal"
                : "Jadwalkan"}
          </button>
          {conflicts.length > 0 && (
            <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
              <p className="font-semibold">Bentrok jadwal di aplikasi & environment yang sama:</p>
              <ul className="mt-1 space-y-1">
                {conflicts.map((c) => (
                  <li key={c.id}>
                    <Link to={`/changes/${c.id}`} className="font-mono text-xs hover:underline">
                      {c.change_no}
                    </Link>{" "}
                    {c.title} · {formatTime(c.planned_start)} – {formatTime(c.planned_end)}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </form>
      )}

      {showStart && (
        <div>
          {change.status === "APPROVED" && (
            <p className="mb-2 text-sm text-veil">
              Emergency change boleh langsung diimplementasikan tanpa jadwal.
            </p>
          )}
          <button onClick={() => startMutation.mutate()} disabled={pending} className={primaryButton}>
            {startMutation.isPending ? "…" : "Mulai Implementasi"}
          </button>
        </div>
      )}

      {showComplete && (
        <form onSubmit={submitComplete} className="space-y-2">
          <label className="block text-sm">
            Outcome
            <select
              value={outcome}
              onChange={(e) => setOutcome(e.target.value as ChangeOutcome)}
              className={inputClass}
            >
              <option value="SUCCESS">SUCCESS</option>
              <option value="FAILED">FAILED</option>
              <option value="ROLLED_BACK">ROLLED_BACK</option>
            </select>
          </label>
          <textarea
            value={outcomeNotes}
            onChange={(e) => setOutcomeNotes(e.target.value)}
            placeholder={outcome === "SUCCESS" ? "Catatan (opsional)" : "Apa yang terjadi? (wajib)"}
            rows={2}
            className={inputClass}
          />
          <button type="submit" disabled={pending} className={primaryButton}>
            {complete.isPending ? "…" : "Selesaikan Implementasi"}
          </button>
        </form>
      )}

      {showClose && (
        <form onSubmit={submitClose} className="space-y-2">
          <label className="block text-sm">
            Post-implementation review{pirRequired ? " (wajib)" : " (opsional)"}
            <textarea
              value={reviewNotes}
              onChange={(e) => setReviewNotes(e.target.value)}
              placeholder="Root cause, pelajaran, tindak lanjut"
              rows={3}
              className={inputClass}
            />
          </label>
          <button type="submit" disabled={pending} className={primaryButton}>
            {close.isPending ? "…" : "Close Change"}
          </button>
        </form>
      )}
    </div>
  );
}
