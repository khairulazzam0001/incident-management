import { useMemo, useState } from "react";
import type { FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { ALLOWED_TRANSITIONS } from "../api/types";
import type { IncidentStatus } from "../api/types";
import { ApiError, api } from "../api/client";
import { isCoordinator, useAuth } from "../auth/AuthContext";
import { useMeta, useUsers } from "../hooks/useMeta";
import { PriorityBadge } from "../components/PriorityBadge";
import { SeverityBadge } from "../components/SeverityBadge";
import { StatusBadge } from "../components/StatusBadge";

function formatTime(iso: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString("id-ID");
}

function activityLabel(type: string, from: string | null, to: string | null): string {
  switch (type) {
    case "created":
      return "Incident dibuat";
    case "assignment":
      return "Assignment diubah";
    case "status_change":
      return `Status ${from ?? "?"} → ${to ?? "?"}`;
    case "comment":
      return "Komentar ditambahkan";
    case "investigation":
      return "Investigation dicatat";
    case "fix":
      return "Fix dicatat";
    case "verification":
      return "Verification disubmit";
    case "reopen":
      return "Incident dibuka kembali";
    default:
      return type;
  }
}

export function IncidentDetail() {
  const { id } = useParams();
  const incidentId = id ?? "";
  const queryClient = useQueryClient();
  const { user } = useAuth();
  const meta = useMeta();
  const usersQuery = useUsers();

  const [nextStatus, setNextStatus] = useState("");
  const [teamId, setTeamId] = useState("");
  const [picId, setPicId] = useState("");
  const [note, setNote] = useState("");
  const [commentBody, setCommentBody] = useState("");
  const [actionError, setActionError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [invNotes, setInvNotes] = useState("");
  const [invFindings, setInvFindings] = useState("");
  const [fixDesc, setFixDesc] = useState("");
  const [fixRef, setFixRef] = useState("");
  const [verifyResult, setVerifyResult] = useState("PASS");
  const [verifyReason, setVerifyReason] = useState("");
  const [reopenReason, setReopenReason] = useState("");
  const [pendingFile, setPendingFile] = useState<File | null>(null);

  const incidentQuery = useQuery({
    queryKey: ["incident", incidentId],
    queryFn: ({ signal }) => api.getIncident(incidentId, signal),
    enabled: incidentId !== "",
  });
  const commentsQuery = useQuery({
    queryKey: ["comments", incidentId],
    queryFn: ({ signal }) => api.getComments(incidentId, signal),
    enabled: incidentId !== "",
  });
  const timelineQuery = useQuery({
    queryKey: ["timeline", incidentId],
    queryFn: ({ signal }) => api.getTimeline(incidentId, signal),
    enabled: incidentId !== "",
  });
  const investigationsQuery = useQuery({
    queryKey: ["investigations", incidentId],
    queryFn: ({ signal }) => api.getInvestigations(incidentId, signal),
    enabled: incidentId !== "",
  });
  const fixesQuery = useQuery({
    queryKey: ["fixes", incidentId],
    queryFn: ({ signal }) => api.getFixes(incidentId, signal),
    enabled: incidentId !== "",
  });
  const verificationsQuery = useQuery({
    queryKey: ["verifications", incidentId],
    queryFn: ({ signal }) => api.getVerifications(incidentId, signal),
    enabled: incidentId !== "",
  });
  const attachmentsQuery = useQuery({
    queryKey: ["attachments", incidentId],
    queryFn: ({ signal }) => api.getAttachments(incidentId, signal),
    enabled: incidentId !== "",
  });

  const userById = useMemo(
    () => new Map((usersQuery.data?.data ?? []).map((u) => [u.id, u] as const)),
    [usersQuery.data],
  );
  const teamById = useMemo(
    () => new Map((meta.data?.teams ?? []).map((t) => [t.id, t.name] as const)),
    [meta.data],
  );

  function refresh() {
    queryClient.invalidateQueries({ queryKey: ["incident", incidentId] });
    queryClient.invalidateQueries({ queryKey: ["comments", incidentId] });
    queryClient.invalidateQueries({ queryKey: ["timeline", incidentId] });
    queryClient.invalidateQueries({ queryKey: ["investigations", incidentId] });
    queryClient.invalidateQueries({ queryKey: ["fixes", incidentId] });
    queryClient.invalidateQueries({ queryKey: ["verifications", incidentId] });
    queryClient.invalidateQueries({ queryKey: ["attachments", incidentId] });
    queryClient.invalidateQueries({ queryKey: ["incidents"] });
  }

  function fail(err: unknown) {
    setSuccess(null);
    setActionError(err instanceof ApiError ? err.message : "Aksi gagal. Coba lagi.");
  }

  function done(message: string) {
    setActionError(null);
    setSuccess(message);
    refresh();
  }

  const statusMutation = useMutation({
    mutationFn: (status: string) => api.changeStatus(incidentId, status),
    onSuccess: () => {
      setActionError(null);
      setNextStatus("");
      refresh();
    },
    onError: fail,
  });
  const assignMutation = useMutation({
    mutationFn: () =>
      api.assignIncident(incidentId, {
        team_id: teamId || null,
        pic_id: picId || null,
        note,
      }),
    onSuccess: () => {
      setActionError(null);
      refresh();
    },
    onError: fail,
  });
  const commentMutation = useMutation({
    mutationFn: (body: string) => api.addComment(incidentId, body),
    onSuccess: () => {
      setActionError(null);
      setCommentBody("");
      refresh();
    },
    onError: fail,
  });
  const investigationMutation = useMutation({
    mutationFn: () => api.addInvestigation(incidentId, { notes: invNotes, findings: invFindings }),
    onSuccess: () => {
      setInvNotes("");
      setInvFindings("");
      done("Investigation tersimpan.");
    },
    onError: fail,
  });
  const fixMutation = useMutation({
    mutationFn: () => api.addFix(incidentId, { description: fixDesc, reference: fixRef }),
    onSuccess: () => {
      setFixDesc("");
      setFixRef("");
      done("Fix tersimpan.");
    },
    onError: fail,
  });
  const verifyMutation = useMutation({
    mutationFn: () => api.verify(incidentId, { result: verifyResult, reason: verifyReason }),
    onSuccess: (v) => {
      setVerifyReason("");
      done(
        v.result === "PASS"
          ? "Verification PASS — incident RESOLVED."
          : "Verification FAIL — incident kembali ke FIXING.",
      );
    },
    onError: fail,
  });
  const closeMutation = useMutation({
    mutationFn: () => api.closeIncident(incidentId),
    onSuccess: () => done("Incident CLOSED."),
    onError: fail,
  });
  const reopenMutation = useMutation({
    mutationFn: () => api.reopenIncident(incidentId, reopenReason),
    onSuccess: () => {
      setReopenReason("");
      done("Incident dibuka kembali (INVESTIGATING).");
    },
    onError: fail,
  });
  const uploadMutation = useMutation({
    mutationFn: (file: File) => api.uploadAttachment(incidentId, file),
    onSuccess: () => {
      setPendingFile(null);
      done("File terunggah.");
    },
    onError: fail,
  });

  function submitStatus(e: FormEvent) {
    e.preventDefault();
    if (nextStatus) statusMutation.mutate(nextStatus);
  }

  function submitAssign(e: FormEvent) {
    e.preventDefault();
    if (!teamId && !picId) {
      setActionError("Pilih team atau PIC.");
      return;
    }
    assignMutation.mutate();
  }

  function submitComment(e: FormEvent) {
    e.preventDefault();
    if (commentBody.trim() === "") {
      setActionError("Komentar tidak boleh kosong.");
      return;
    }
    commentMutation.mutate(commentBody);
  }

  function submitInvestigation(e: FormEvent) {
    e.preventDefault();
    if (invNotes.trim() === "" && invFindings.trim() === "") {
      setActionError("Notes atau findings wajib diisi.");
      return;
    }
    investigationMutation.mutate();
  }

  function submitFix(e: FormEvent) {
    e.preventDefault();
    if (fixDesc.trim() === "") {
      setActionError("Fix description wajib diisi.");
      return;
    }
    fixMutation.mutate();
  }

  function submitVerify(e: FormEvent) {
    e.preventDefault();
    if (verifyResult === "FAIL" && verifyReason.trim() === "") {
      setActionError("Reason wajib diisi untuk FAIL.");
      return;
    }
    verifyMutation.mutate();
  }

  function submitReopen(e: FormEvent) {
    e.preventDefault();
    if (reopenReason.trim() === "") {
      setActionError("Reason wajib diisi untuk reopen.");
      return;
    }
    reopenMutation.mutate();
  }

  function submitUpload(e: FormEvent) {
    e.preventDefault();
    if (pendingFile === null) {
      setActionError("Pilih file dulu (png/jpg/gif/webp/pdf/txt/zip, maks 10 MiB).");
      return;
    }
    uploadMutation.mutate(pendingFile);
  }

  async function downloadFile(aid: string, fileName: string) {
    try {
      const blob = await api.downloadAttachment(incidentId, aid);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = fileName;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      fail(err);
    }
  }

  function formatBytes(n: number): string {
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KiB`;
    return `${(n / 1024 / 1024).toFixed(1)} MiB`;
  }

  if (incidentQuery.isPending) {
    return (
      <section className="w-full p-6" aria-label="Memuat">
        <div className="h-8 w-1/2 animate-pulse rounded bg-lilac" />
        <div className="mt-4 h-40 animate-pulse rounded bg-lilac" />
      </section>
    );
  }

  if (incidentQuery.isError) {
    return (
      <section className="w-full p-6">
        <div className="rounded border border-red-200 bg-red-50 p-4 text-sm">
          <p className="text-red-700">Gagal memuat incident.</p>
          <button
            onClick={() => incidentQuery.refetch()}
            className="mt-2 rounded border px-3 py-1 hover:bg-white"
          >
            Coba lagi
          </button>
        </div>
        <Link to="/" className="mt-4 inline-block text-sm text-iris hover:underline">
          ← Kembali ke daftar
        </Link>
      </section>
    );
  }

  const incident = incidentQuery.data;
  const allowed = ALLOWED_TRANSITIONS[incident.status] ?? [];
  const canAssign = user !== null && isCoordinator(user.role);
  const canVerify =
    user !== null && (user.role === "QA" || isCoordinator(user.role));
  const showClose = canAssign && incident.status === "RESOLVED";
  const showReopen =
    canAssign && (incident.status === "RESOLVED" || incident.status === "CLOSED");
  const showWorkSections =
    incident.status === "ASSIGNED" ||
    incident.status === "INVESTIGATING" ||
    incident.status === "FIXING" ||
    incident.status === "VERIFYING";
  const inputClass =
    "mt-1 w-full rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none";

  return (
    <section className="w-full space-y-6 p-6">
      <div>
        <Link to="/" className="text-sm text-iris hover:underline">
          ← Kembali ke daftar
        </Link>
        <p className="mt-2 font-mono text-xs text-veil">{incident.incident_no}</p>
        <h1 className="mt-1 text-2xl font-semibold">{incident.title}</h1>
        <div className="mt-2 flex flex-wrap gap-2">
          <StatusBadge status={incident.status} />
          <SeverityBadge severity={incident.severity} />
          <PriorityBadge priority={incident.priority} />
        </div>
      </div>

      <div className="rounded border bg-white p-4 text-sm">
        <p className="whitespace-pre-wrap">{incident.description || "—"}</p>
        <dl className="mt-4 grid grid-cols-2 gap-2 text-veil">
          <div>
            <dt className="text-xs uppercase text-veil">Source</dt>
            <dd>{incident.source}</dd>
          </div>
          <div>
            <dt className="text-xs uppercase text-veil">Environment</dt>
            <dd>{incident.environment ?? "—"}</dd>
          </div>
          <div>
            <dt className="text-xs uppercase text-veil">Team</dt>
            <dd>{(incident.team_id && teamById.get(incident.team_id)) || "—"}</dd>
          </div>
          <div>
            <dt className="text-xs uppercase text-veil">PIC</dt>
            <dd>{(incident.pic_id && userById.get(incident.pic_id)?.name) || "—"}</dd>
          </div>
          <div>
            <dt className="text-xs uppercase text-veil">Dibuat</dt>
            <dd>{formatTime(incident.created_at)}</dd>
          </div>
          <div>
            <dt className="text-xs uppercase text-veil">Diubah</dt>
            <dd>{formatTime(incident.updated_at)}</dd>
          </div>
        </dl>
      </div>

      {actionError !== null && (
        <p className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{actionError}</p>
      )}
      {success !== null && (
        <p className="rounded border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-700">{success}</p>
      )}

      <div className="rounded-lg border border-mist bg-paper p-6">
        <h2 className="font-semibold">Ubah Status</h2>
        {allowed.length === 0 ? (
          <p className="mt-2 text-sm text-veil">
            Status terminal — hanya bisa via reopen khusus.
          </p>
        ) : (
          <form onSubmit={submitStatus} className="mt-2 flex gap-2">
            <select
              value={nextStatus}
              onChange={(e) => setNextStatus(e.target.value as IncidentStatus)}
              className="flex-1 rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none"
            >
              <option value="">— pilih status —</option>
              {allowed.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
            <button
              type="submit"
              disabled={!nextStatus || statusMutation.isPending}
              className="rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50"
            >
              {statusMutation.isPending ? "…" : "Ubah"}
            </button>
          </form>
        )}
      </div>

      {canAssign && (
        <div className="rounded-lg border border-mist bg-paper p-6">
          <h2 className="font-semibold">Assignment</h2>
          <form onSubmit={submitAssign} className="mt-2 space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <label className="block">
                <span className="text-sm">Team</span>
                <select
                  value={teamId}
                  onChange={(e) => setTeamId(e.target.value)}
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
              <label className="block">
                <span className="text-sm">PIC</span>
                <select
                  value={picId}
                  onChange={(e) => setPicId(e.target.value)}
                  className={inputClass}
                >
                  <option value="">—</option>
                  {(usersQuery.data?.data ?? []).map((u) => (
                    <option key={u.id} value={u.id}>
                      {u.name} ({u.role})
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <label className="block">
              <span className="text-sm">Catatan</span>
              <input
                value={note}
                onChange={(e) => setNote(e.target.value)}
                className={inputClass}
              />
            </label>
            <button
              type="submit"
              disabled={assignMutation.isPending}
              className="rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50"
            >
              {assignMutation.isPending ? "Menyimpan…" : "Simpan Assignment"}
            </button>
          </form>
        </div>
      )}

      {showWorkSections && (
        <div className="rounded-lg border border-mist bg-paper p-6">
          <h2 className="font-semibold">Investigation</h2>
          <ul className="mt-2 space-y-3">
            {(investigationsQuery.data?.data ?? []).map((inv) => (
              <li key={inv.id} className="rounded-lg bg-chalk p-3 text-sm">
                {inv.notes && <p className="whitespace-pre-wrap">{inv.notes}</p>}
                {inv.findings && (
                  <p className="mt-1 whitespace-pre-wrap">
                    <span className="font-medium">Findings: </span>
                    {inv.findings}
                  </p>
                )}
                <p className="mt-1 text-xs text-veil">
                  {(inv.author_id && userById.get(inv.author_id)?.name) || "?"} ·{" "}
                  {formatTime(inv.created_at)}
                </p>
              </li>
            ))}
            {investigationsQuery.data && investigationsQuery.data.data.length === 0 && (
              <li className="text-sm text-veil">Belum ada investigation.</li>
            )}
          </ul>
          <form onSubmit={submitInvestigation} className="mt-3 space-y-2">
            <textarea
              value={invNotes}
              onChange={(e) => setInvNotes(e.target.value)}
              placeholder="Notes investigasi…"
              rows={2}
              className={inputClass}
            />
            <textarea
              value={invFindings}
              onChange={(e) => setInvFindings(e.target.value)}
              placeholder="Findings / akar masalah…"
              rows={2}
              className={inputClass}
            />
            <button
              type="submit"
              disabled={investigationMutation.isPending}
              className="rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50"
            >
              {investigationMutation.isPending ? "Menyimpan…" : "Simpan Investigation"}
            </button>
          </form>
        </div>
      )}

      {showWorkSections && (
        <div className="rounded-lg border border-mist bg-paper p-6">
          <h2 className="font-semibold">Fix</h2>
          <ul className="mt-2 space-y-3">
            {(fixesQuery.data?.data ?? []).map((f) => (
              <li key={f.id} className="rounded-lg bg-chalk p-3 text-sm">
                <p className="whitespace-pre-wrap">{f.description}</p>
                {f.reference && (
                  <p className="mt-1 text-xs text-veil">Ref: {f.reference}</p>
                )}
                <p className="mt-1 text-xs text-veil">
                  {(f.author_id && userById.get(f.author_id)?.name) || "?"} ·{" "}
                  {formatTime(f.created_at)}
                </p>
              </li>
            ))}
            {fixesQuery.data && fixesQuery.data.data.length === 0 && (
              <li className="text-sm text-veil">Belum ada fix.</li>
            )}
          </ul>
          <form onSubmit={submitFix} className="mt-3 space-y-2">
            <textarea
              value={fixDesc}
              onChange={(e) => setFixDesc(e.target.value)}
              placeholder="Deskripsi perbaikan…"
              rows={2}
              className={inputClass}
            />
            <input
              value={fixRef}
              onChange={(e) => setFixRef(e.target.value)}
              placeholder="Referensi deployment / PR (opsional)"
              className={inputClass}
            />
            <button
              type="submit"
              disabled={fixMutation.isPending}
              className="rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50"
            >
              {fixMutation.isPending ? "Menyimpan…" : "Simpan Fix"}
            </button>
          </form>
        </div>
      )}

      {canVerify && incident.status === "VERIFYING" && (
        <div className="rounded-lg border border-mist bg-paper p-6">
          <h2 className="font-semibold">Verification</h2>
          <ul className="mt-2 space-y-2 text-sm">
            {(verificationsQuery.data?.data ?? []).map((v) => (
              <li key={v.id} className="rounded-lg bg-chalk p-3">
                <span
                  className={`rounded px-2 py-1 text-xs font-medium ${
                    v.result === "PASS"
                      ? "bg-green-100 text-green-700"
                      : "bg-red-100 text-red-700"
                  }`}
                >
                  {v.result}
                </span>
                {v.reason && <p className="mt-1 whitespace-pre-wrap">{v.reason}</p>}
                <p className="mt-1 text-xs text-veil">
                  {(v.verifier_id && userById.get(v.verifier_id)?.name) || "?"} ·{" "}
                  {formatTime(v.created_at)}
                </p>
              </li>
            ))}
          </ul>
          <form onSubmit={submitVerify} className="mt-3 space-y-2">
            <div className="flex gap-2">
              <select
                value={verifyResult}
                onChange={(e) => setVerifyResult(e.target.value)}
                className="rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none"
                aria-label="Hasil verifikasi"
              >
                <option value="PASS">PASS</option>
                <option value="FAIL">FAIL</option>
              </select>
              <input
                value={verifyReason}
                onChange={(e) => setVerifyReason(e.target.value)}
                placeholder={verifyResult === "FAIL" ? "Reason (wajib)" : "Reason (opsional)"}
                className="flex-1 rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none"
              />
            </div>
            <button
              type="submit"
              disabled={verifyMutation.isPending}
              className="rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50"
            >
              {verifyMutation.isPending ? "Menyimpan…" : "Submit Verification"}
            </button>
          </form>
        </div>
      )}

      {(showClose || showReopen) && (
        <div className="rounded-lg border border-mist bg-paper p-6">
          <h2 className="font-semibold">Close / Reopen</h2>
          {showClose && (
            <button
              onClick={() => closeMutation.mutate()}
              disabled={closeMutation.isPending}
              className="mt-2 rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50"
            >
              {closeMutation.isPending ? "…" : "Close Incident"}
            </button>
          )}
          {showReopen && (
            <form onSubmit={submitReopen} className="mt-3 flex gap-2">
              <input
                value={reopenReason}
                onChange={(e) => setReopenReason(e.target.value)}
                placeholder="Alasan reopen (wajib)"
                className="flex-1 rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none"
              />
              <button
                type="submit"
                disabled={reopenMutation.isPending}
                className="rounded border border-amber-500 px-4 py-2 text-sm font-medium text-amber-700 hover:bg-amber-50 disabled:opacity-50"
              >
                {reopenMutation.isPending ? "…" : "Reopen"}
              </button>
            </form>
          )}
        </div>
      )}

      <div className="rounded-lg border border-mist bg-paper p-6">
        <h2 className="font-semibold">Attachment</h2>
        <ul className="mt-2 space-y-2 text-sm">
          {(attachmentsQuery.data?.data ?? []).map((a) => (
            <li
              key={a.id}
              className="flex items-center justify-between gap-2 rounded-lg bg-chalk p-3"
            >
              <span className="min-w-0">
                <span className="block truncate font-medium">{a.file_name}</span>
                <span className="text-xs text-veil">
                  {a.mime_type} · {formatBytes(a.size_bytes)}
                </span>
              </span>
              <button
                type="button"
                onClick={() => downloadFile(a.id, a.file_name)}
                className="shrink-0 rounded-full border border-mist px-3 py-1 text-xs font-semibold text-deep hover:bg-lilac"
              >
                Unduh
              </button>
            </li>
          ))}
          {attachmentsQuery.data && attachmentsQuery.data.data.length === 0 && (
            <li className="text-sm text-veil">Belum ada file.</li>
          )}
        </ul>
        <form onSubmit={submitUpload} className="mt-3 flex gap-2">
          <input
            type="file"
            accept=".png,.jpg,.jpeg,.gif,.webp,.pdf,.txt,.zip"
            onChange={(e) => setPendingFile(e.target.files?.[0] ?? null)}
            className="flex-1 rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none"
            aria-label="Pilih file"
          />
          <button
            type="submit"
            disabled={uploadMutation.isPending}
            className="rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50"
          >
            {uploadMutation.isPending ? "Mengunggah…" : "Unggah"}
          </button>
        </form>
      </div>

      <div className="rounded-lg border border-mist bg-paper p-6">
        <h2 className="font-semibold">Komentar</h2>
        <ul className="mt-2 space-y-3">
          {(commentsQuery.data?.data ?? []).map((c) => (
            <li key={c.id} className="rounded-lg bg-chalk p-3 text-sm">
              <p className="whitespace-pre-wrap">{c.body}</p>
              <p className="mt-1 text-xs text-veil">
                {(c.author_id && userById.get(c.author_id)?.name) || "?"} ·{" "}
                {formatTime(c.created_at)}
              </p>
            </li>
          ))}
          {commentsQuery.data && commentsQuery.data.data.length === 0 && (
            <li className="text-sm text-veil">Belum ada komentar.</li>
          )}
        </ul>
        <form onSubmit={submitComment} className="mt-3 flex gap-2">
          <input
            value={commentBody}
            onChange={(e) => setCommentBody(e.target.value)}
            placeholder="Tulis komentar…"
            className="flex-1 rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none"
          />
          <button
            type="submit"
            disabled={commentMutation.isPending}
            className="rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50"
          >
            {commentMutation.isPending ? "…" : "Kirim"}
          </button>
        </form>
      </div>

      <div className="rounded-lg border border-mist bg-paper p-6">
        <h2 className="font-semibold">Timeline</h2>
        <ul className="mt-2 space-y-2 text-sm">
          {(timelineQuery.data?.data ?? []).map((a) => (
            <li key={a.id} className="flex justify-between gap-2 border-b py-1 last:border-0">
              <span>
                {activityLabel(a.type, a.from_status, a.to_status)}
                <span className="text-veil">
                  {" "}
                  · {(a.actor_id && userById.get(a.actor_id)?.name) || "sistem"}
                </span>
              </span>
              <span className="shrink-0 text-xs text-veil">
                {formatTime(a.created_at)}
              </span>
            </li>
          ))}
          {timelineQuery.data && timelineQuery.data.data.length === 0 && (
            <li className="text-veil">Belum ada aktivitas.</li>
          )}
        </ul>
      </div>
    </section>
  );
}
