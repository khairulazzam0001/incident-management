import { useState } from "react";
import type { FormEvent } from "react";
import type { ChangeInput, ChangeRisk, ChangeType } from "../api/types";
import { useMeta, useUsers } from "../hooks/useMeta";
import { canCreateChange } from "../auth/AuthContext";

export const EMPTY_CHANGE: ChangeInput = {
  title: "",
  description: "",
  justification: "",
  type: "NORMAL",
  risk: "",
  application_id: "",
  environment: "",
  implementation_plan: "",
  rollback_plan: "",
  test_plan: "",
  implementer_id: null,
  team_id: null,
};

// Aturan konten sebelum submit (PRD_Change_Management.md §6, CM-01).
// Backend tetap memvalidasi ulang.
export function submitProblem(v: ChangeInput): string | null {
  if (v.description.trim() === "") return "Deskripsi wajib diisi sebelum submit.";
  if (v.implementation_plan.trim() === "") return "Implementation plan wajib diisi sebelum submit.";
  if (v.rollback_plan.trim() === "") return "Rollback plan wajib diisi sebelum submit.";
  if (v.risk === "HIGH" && v.test_plan.trim() === "") return "Test plan wajib untuk risk HIGH.";
  if (v.type === "STANDARD" && v.risk !== "LOW") return "Standard change harus berisiko LOW.";
  return null;
}

function draftProblem(v: ChangeInput): string | null {
  if (v.title.trim().length < 5) return "Title minimal 5 karakter.";
  if (v.type === "") return "Type wajib dipilih.";
  if (v.risk === "") return "Risk wajib dipilih.";
  if (v.application_id === "") return "Application wajib dipilih.";
  if (v.environment === "") return "Environment wajib dipilih.";
  return null;
}

const inputClass =
  "mt-1 w-full rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none";

export function ChangeForm({
  initial,
  pending,
  error,
  submitLabel,
  onSave,
  onCancel,
}: {
  initial: ChangeInput;
  pending: boolean;
  error: string | null;
  submitLabel: string;
  onSave: (input: ChangeInput, submitAfter: boolean) => void;
  onCancel: () => void;
}) {
  const meta = useMeta();
  const users = useUsers();
  const [v, setV] = useState<ChangeInput>(initial);
  const [localError, setLocalError] = useState<string | null>(null);

  function set<K extends keyof ChangeInput>(key: K, value: ChangeInput[K]) {
    setV((prev) => ({ ...prev, [key]: value }));
  }

  function save(e: FormEvent, submitAfter: boolean) {
    e.preventDefault();
    const problem = draftProblem(v) ?? (submitAfter ? submitProblem(v) : null);
    setLocalError(problem);
    if (problem === null) onSave({ ...v, title: v.title.trim() }, submitAfter);
  }

  const implementers = (users.data?.data ?? []).filter((u) => canCreateChange(u.role));
  const shownError = localError ?? error;

  return (
    <form onSubmit={(e) => save(e, false)} className="mt-6 space-y-4">
      <label className="block">
        <span className="text-sm font-medium">Title *</span>
        <input
          value={v.title}
          onChange={(e) => set("title", e.target.value)}
          className={inputClass}
          placeholder="Cth: Deploy hotfix checkout timeout"
        />
      </label>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <label className="block">
          <span className="text-sm font-medium">Type *</span>
          <select
            value={v.type}
            onChange={(e) => set("type", e.target.value as ChangeType | "")}
            className={inputClass}
          >
            {(meta.data?.change_types ?? []).map((t) => (
              <option key={t.code} value={t.code}>
                {t.name}
              </option>
            ))}
          </select>
          <span className="mt-1 block text-xs text-veil">
            {v.type === "STANDARD" && "Pre-approved, khusus perubahan rutin berisiko LOW."}
            {v.type === "NORMAL" && "Wajib approval Manager/Lead."}
            {v.type === "EMERGENCY" &&
              "Wajib approval sebelum implementasi; PIR wajib saat close."}
          </span>
        </label>
        <label className="block">
          <span className="text-sm font-medium">Risk *</span>
          <select
            value={v.risk}
            onChange={(e) => set("risk", e.target.value as ChangeRisk | "")}
            className={inputClass}
          >
            <option value="">Silahkan pilih</option>
            {(meta.data?.change_risks ?? []).map((r) => (
              <option key={r.code} value={r.code}>
                {r.name}
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className="text-sm font-medium">Application *</span>
          <select
            value={v.application_id}
            onChange={(e) => set("application_id", e.target.value)}
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
        <label className="block">
          <span className="text-sm font-medium">Environment *</span>
          <select
            value={v.environment}
            onChange={(e) => set("environment", e.target.value)}
            className={inputClass}
          >
            <option value="">Silahkan pilih</option>
            {(meta.data?.environments ?? []).map((env) => (
              <option key={env.code} value={env.code}>
                {env.name}
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className="text-sm font-medium">Implementer</span>
          <select
            value={v.implementer_id ?? ""}
            onChange={(e) => set("implementer_id", e.target.value || null)}
            className={inputClass}
          >
            <option value="">—</option>
            {implementers.map((u) => (
              <option key={u.id} value={u.id}>
                {u.name} ({u.role})
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className="text-sm font-medium">Team</span>
          <select
            value={v.team_id ?? ""}
            onChange={(e) => set("team_id", e.target.value || null)}
            className={inputClass}
          >
            <option value="">—</option>
            {(meta.data?.teams ?? []).map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </select>
        </label>
      </div>
      <label className="block">
        <span className="text-sm font-medium">Deskripsi</span>
        <textarea
          value={v.description}
          onChange={(e) => set("description", e.target.value)}
          className={inputClass}
          rows={3}
        />
      </label>
      <label className="block">
        <span className="text-sm font-medium">Justifikasi</span>
        <textarea
          value={v.justification}
          onChange={(e) => set("justification", e.target.value)}
          className={inputClass}
          rows={2}
          placeholder="Alasan bisnis/teknis perubahan"
        />
      </label>
      <label className="block">
        <span className="text-sm font-medium">Implementation plan</span>
        <textarea
          value={v.implementation_plan}
          onChange={(e) => set("implementation_plan", e.target.value)}
          className={inputClass}
          rows={3}
          placeholder="Langkah-langkah perubahan"
        />
      </label>
      <label className="block">
        <span className="text-sm font-medium">Rollback plan</span>
        <textarea
          value={v.rollback_plan}
          onChange={(e) => set("rollback_plan", e.target.value)}
          className={inputClass}
          rows={2}
          placeholder="Cara mengembalikan bila gagal"
        />
      </label>
      <label className="block">
        <span className="text-sm font-medium">
          Test plan{v.risk === "HIGH" ? " (wajib untuk HIGH)" : ""}
        </span>
        <textarea
          value={v.test_plan}
          onChange={(e) => set("test_plan", e.target.value)}
          className={inputClass}
          rows={2}
        />
      </label>
      <p className="text-xs text-veil">
        Deskripsi, implementation plan, dan rollback plan boleh dilengkapi nanti, tetapi wajib
        sebelum submit.
      </p>
      {shownError !== null && (
        <p className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {shownError}
        </p>
      )}
      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          disabled={pending}
          onClick={(e) => save(e, true)}
          className="rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50"
        >
          {pending ? "Menyimpan…" : "Simpan & Submit"}
        </button>
        <button
          type="submit"
          disabled={pending}
          className="rounded-full border border-mist px-5 py-2 text-sm font-semibold text-deep hover:bg-lilac disabled:opacity-50"
        >
          {submitLabel}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="rounded-full px-4 py-2 text-sm font-semibold text-veil hover:bg-lilac"
        >
          Batal
        </button>
      </div>
    </form>
  );
}
