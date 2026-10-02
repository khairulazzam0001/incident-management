import { useMemo, useState } from "react";
import type { FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useLocation, useParams } from "react-router-dom";
import { ApiError, api } from "../api/client";
import type { ApprovalDecision, Change, ChangeInput, User } from "../api/types";
import { canViewChanges, useAuth } from "../auth/AuthContext";
import { useMeta, useUsers } from "../hooks/useMeta";
import { ChangeForm } from "../components/ChangeForm";
import { ChangeStatusBadge } from "../components/ChangeStatusBadge";
import { ChangeTypeBadge } from "../components/ChangeTypeBadge";
import { RiskBadge } from "../components/RiskBadge";
import { StatusBadge } from "../components/StatusBadge";

function formatTime(iso: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString("id-ID");
}

function payloadField(payload: unknown, key: string): string {
  if (typeof payload !== "object" || payload === null) return "";
  const v = (payload as Record<string, unknown>)[key];
  return typeof v === "string" ? v : "";
}

const DECISION_LABEL: Record<string, string> = {
  APPROVED: "Approved",
  REJECTED: "Rejected",
  CHANGES_REQUESTED: "Revisi diminta",
  AUTO_APPROVED: "Auto-approved (standard)",
};

function activityLabel(type: string, from: string | null, to: string | null, payload: unknown): string {
  switch (type) {
    case "created":
      return "Change request dibuat";
    case "updated":
      return "Draft diubah";
    case "status_change": {
      const reason = payloadField(payload, "reason");
      return `Status ${from ?? "?"} → ${to ?? "?"}${reason ? ` (${reason})` : ""}`;
    }
    case "approval":
      return `Keputusan: ${DECISION_LABEL[payloadField(payload, "decision")] ?? "?"}`;
    case "comment":
      return "Komentar ditambahkan";
    case "incident_link":
      return `Ditautkan ke ${payloadField(payload, "incident_no")} (${payloadField(payload, "relation")})`;
    default:
      return type;
  }
}

// Aturan approver untuk UX saja; backend yang menegakkan (§5, Q1–Q2).
function canDecide(user: User | null, change: Change, requester: User | undefined): boolean {
  if (user === null || user.id === change.requester_id) return false;
  if (user.role === "ManagerLead") return true;
  return (
    user.role === "SystemAnalyst" &&
    requester?.role === "ManagerLead" &&
    (change.risk === "LOW" || change.risk === "MEDIUM")
  );
}

function toInput(c: Change): ChangeInput {
  return {
    title: c.title,
    description: c.description,
    justification: c.justification,
    type: c.type,
    risk: c.risk,
    application_id: c.application_id,
    environment: c.environment,
    implementation_plan: c.implementation_plan,
    rollback_plan: c.rollback_plan,
    test_plan: c.test_plan,
    implementer_id: c.implementer_id,
    team_id: c.team_id,
  };
}

const CANCELLABLE = ["DRAFT", "SUBMITTED", "APPROVED", "SCHEDULED"];
const inputClass =
  "w-full rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none";
const primaryButton =
  "rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50";
const secondaryButton =
  "rounded-full border border-mist px-4 py-2 text-sm font-semibold text-deep hover:bg-lilac disabled:opacity-50";

export function ChangeDetail() {
  const { id } = useParams();
  const changeId = id ?? "";
  const location = useLocation();
  const queryClient = useQueryClient();
  const { user } = useAuth();
  const meta = useMeta();
  const usersQuery = useUsers();
  const allowed = user !== null && canViewChanges(user.role);
  const enabled = changeId !== "" && allowed;

  const initialNotice =
    typeof location.state === "object" && location.state !== null
      ? payloadField(location.state, "notice")
      : "";
  const [success, setSuccess] = useState<string | null>(initialNotice || null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);
  const [decisionReason, setDecisionReason] = useState("");
  const [cancelReason, setCancelReason] = useState("");
  const [commentBody, setCommentBody] = useState("");

  const changeQuery = useQuery({
    queryKey: ["change", changeId],
    queryFn: ({ signal }) => api.getChange(changeId, signal),
    enabled,
  });
  const approvalsQuery = useQuery({
    queryKey: ["change-approvals", changeId],
    queryFn: ({ signal }) => api.getChangeApprovals(changeId, signal),
    enabled,
  });
  const linksQuery = useQuery({
    queryKey: ["change-incidents", changeId],
    queryFn: ({ signal }) => api.getChangeIncidents(changeId, signal),
    enabled,
  });
  const commentsQuery = useQuery({
    queryKey: ["change-comments", changeId],
    queryFn: ({ signal }) => api.getChangeComments(changeId, signal),
    enabled,
  });
  const timelineQuery = useQuery({
    queryKey: ["change-timeline", changeId],
    queryFn: ({ signal }) => api.getChangeTimeline(changeId, signal),
    enabled,
  });

  const userById = useMemo(
    () => new Map((usersQuery.data?.data ?? []).map((u) => [u.id, u] as const)),
    [usersQuery.data],
  );
  const appName = useMemo(
    () => new Map((meta.data?.applications ?? []).map((a) => [a.id, a.name] as const)),
    [meta.data],
  );
  const teamName = useMemo(
    () => new Map((meta.data?.teams ?? []).map((t) => [t.id, t.name] as const)),
    [meta.data],
  );

  function refresh() {
    for (const key of ["change", "change-approvals", "change-incidents", "change-comments", "change-timeline"]) {
      queryClient.invalidateQueries({ queryKey: [key, changeId] });
    }
    queryClient.invalidateQueries({ queryKey: ["changes"] });
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

  const updateMutation = useMutation({
    mutationFn: async ({ input, submitAfter }: { input: ChangeInput; submitAfter: boolean }) => {
      const saved = await api.updateChange(changeId, input);
      return submitAfter ? api.submitChange(saved.id) : saved;
    },
    onSuccess: (c) => {
      setEditing(false);
      setEditError(null);
      done(c.status === "DRAFT" ? "Draft tersimpan." : `Change disubmit (${c.status}).`);
    },
    onError: (err: unknown) => {
      setEditError(err instanceof ApiError ? err.message : "Gagal menyimpan change.");
      refresh();
    },
  });
  const submitMutation = useMutation({
    mutationFn: () => api.submitChange(changeId),
    onSuccess: (c) =>
      done(
        c.status === "APPROVED"
          ? "Standard change otomatis APPROVED."
          : "Change disubmit, menunggu approval.",
      ),
    onError: fail,
  });
  const decideMutation = useMutation({
    mutationFn: (decision: ApprovalDecision) =>
      api.decideChange(changeId, decision, decisionReason.trim()),
    onSuccess: (c) => {
      setDecisionReason("");
      done(
        c.status === "APPROVED"
          ? "Change APPROVED."
          : c.status === "REJECTED"
            ? "Change REJECTED."
            : "Revisi diminta — change kembali ke DRAFT.",
      );
    },
    onError: fail,
  });
  const cancelMutation = useMutation({
    mutationFn: () => api.cancelChange(changeId, cancelReason.trim()),
    onSuccess: () => {
      setCancelReason("");
      done("Change CANCELLED.");
    },
    onError: fail,
  });
  const commentMutation = useMutation({
    mutationFn: (body: string) => api.addChangeComment(changeId, body),
    onSuccess: () => {
      setCommentBody("");
      setActionError(null);
      refresh();
    },
    onError: fail,
  });

  if (!allowed) {
    return (
      <section className="w-full p-6">
        <p className="rounded-lg border border-mist bg-paper p-6 text-sm text-veil">
          Anda tidak berhak mengakses Change Management.
        </p>
      </section>
    );
  }

  if (changeQuery.isPending) {
    return (
      <section className="w-full p-6" aria-label="Memuat">
        <div className="h-8 w-1/2 animate-pulse rounded bg-lilac" />
        <div className="mt-4 h-40 animate-pulse rounded bg-lilac" />
      </section>
    );
  }

  if (changeQuery.isError) {
    return (
      <section className="w-full p-6">
        <div className="rounded border border-red-200 bg-red-50 p-4 text-sm">
          <p className="text-red-700">
            {changeQuery.error instanceof ApiError
              ? changeQuery.error.message
              : "Gagal memuat change."}
          </p>
          <button onClick={() => changeQuery.refetch()} className={`mt-2 ${secondaryButton}`}>
            Coba lagi
          </button>
        </div>
        <Link to="/changes" className="mt-4 inline-block text-sm text-iris hover:underline">
          ← Kembali ke daftar change
        </Link>
      </section>
    );
  }

  const change = changeQuery.data;
  const requester = userById.get(change.requester_id);
  const isOwnerOrManager =
    user !== null && (user.id === change.requester_id || user.role === "ManagerLead");
  const showDraftActions = change.status === "DRAFT" && isOwnerOrManager;
  const showDecision = change.status === "SUBMITTED" && canDecide(user, change, requester);
  const showCancel = isOwnerOrManager && CANCELLABLE.includes(change.status);
  const anyPending =
    submitMutation.isPending || decideMutation.isPending || cancelMutation.isPending;

  function decide(e: FormEvent, decision: ApprovalDecision) {
    e.preventDefault();
    if (decision !== "APPROVE" && decisionReason.trim() === "") {
      setActionError("Reason wajib diisi untuk reject / request changes.");
      return;
    }
    decideMutation.mutate(decision);
  }

  function submitCancel(e: FormEvent) {
    e.preventDefault();
    if (cancelReason.trim() === "") {
      setActionError("Reason wajib diisi untuk cancel.");
      return;
    }
    cancelMutation.mutate();
  }

  function submitComment(e: FormEvent) {
    e.preventDefault();
    if (commentBody.trim() === "") {
      setActionError("Komentar tidak boleh kosong.");
      return;
    }
    commentMutation.mutate(commentBody);
  }

  const plans: { label: string; value: string }[] = [
    { label: "Justifikasi", value: change.justification },
    { label: "Implementation plan", value: change.implementation_plan },
    { label: "Rollback plan", value: change.rollback_plan },
    { label: "Test plan", value: change.test_plan },
  ];

  return (
    <section className="w-full space-y-6 p-6">
      <div>
        <Link to="/changes" className="text-sm text-iris hover:underline">
          ← Kembali ke daftar change
        </Link>
        <p className="mt-2 font-mono text-xs text-veil">
          {change.change_no} · revisi {change.revision}
        </p>
        <h1 className="mt-1 text-2xl font-semibold">{change.title}</h1>
        <div className="mt-2 flex flex-wrap gap-2">
          <ChangeStatusBadge status={change.status} />
          <ChangeTypeBadge type={change.type} />
          <RiskBadge risk={change.risk} />
        </div>
      </div>

      {actionError !== null && (
        <p className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{actionError}</p>
      )}
      {success !== null && (
        <p className="rounded border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-700">{success}</p>
      )}

      {editing ? (
        <div className="rounded-lg border border-mist bg-paper p-6">
          <h2 className="font-semibold">Edit Draft</h2>
          <ChangeForm
            initial={toInput(change)}
            pending={updateMutation.isPending}
            error={editError}
            submitLabel="Simpan Draft"
            onSave={(input, submitAfter) => {
              setEditError(null);
              updateMutation.mutate({ input, submitAfter });
            }}
            onCancel={() => {
              setEditing(false);
              setEditError(null);
            }}
          />
        </div>
      ) : (
        <div className="rounded-lg border border-mist bg-paper p-6 text-sm">
          <p className="whitespace-pre-wrap">{change.description || "—"}</p>
          <dl className="mt-4 grid grid-cols-2 gap-3 text-veil sm:grid-cols-3">
            <div>
              <dt className="text-xs uppercase">Application</dt>
              <dd className="text-ink">{appName.get(change.application_id) ?? "—"}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase">Environment</dt>
              <dd className="text-ink">{change.environment}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase">Team</dt>
              <dd className="text-ink">{(change.team_id && teamName.get(change.team_id)) || "—"}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase">Requester</dt>
              <dd className="text-ink">{requester?.name ?? "—"}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase">Implementer</dt>
              <dd className="text-ink">
                {(change.implementer_id && userById.get(change.implementer_id)?.name) || "—"}
              </dd>
            </div>
            <div>
              <dt className="text-xs uppercase">Diubah</dt>
              <dd className="text-ink">{formatTime(change.updated_at)}</dd>
            </div>
          </dl>
          <div className="mt-4 space-y-3">
            {plans.map((p) => (
              <div key={p.label}>
                <h3 className="text-xs font-semibold uppercase text-veil">{p.label}</h3>
                <p className="mt-1 whitespace-pre-wrap rounded-lg bg-chalk p-3 text-ink">
                  {p.value || "—"}
                </p>
              </div>
            ))}
          </div>
          {showDraftActions && (
            <div className="mt-4 flex flex-wrap gap-2">
              <button
                onClick={() => submitMutation.mutate()}
                disabled={anyPending}
                className={primaryButton}
              >
                {submitMutation.isPending ? "…" : "Submit untuk Approval"}
              </button>
              <button onClick={() => setEditing(true)} disabled={anyPending} className={secondaryButton}>
                Edit Draft
              </button>
            </div>
          )}
        </div>
      )}

      {showDecision && (
        <div className="rounded-lg border border-mist bg-paper p-6">
          <h2 className="font-semibold">Keputusan Approval</h2>
          <p className="mt-1 text-sm text-veil">
            Periksa risk, implementation plan, dan rollback plan sebelum memutuskan.
          </p>
          <form onSubmit={(e) => decide(e, "APPROVE")} className="mt-3 space-y-2">
            <input
              value={decisionReason}
              onChange={(e) => setDecisionReason(e.target.value)}
              placeholder="Reason (wajib untuk reject / request changes)"
              className={inputClass}
            />
            <div className="flex flex-wrap gap-2">
              <button type="submit" disabled={anyPending} className={primaryButton}>
                Approve
              </button>
              <button
                type="button"
                disabled={anyPending}
                onClick={(e) => decide(e, "REQUEST_CHANGES")}
                className={secondaryButton}
              >
                Minta Revisi
              </button>
              <button
                type="button"
                disabled={anyPending}
                onClick={(e) => decide(e, "REJECT")}
                className="rounded-full border border-red-200 px-4 py-2 text-sm font-semibold text-red-700 hover:bg-red-50 disabled:opacity-50"
              >
                Reject
              </button>
            </div>
          </form>
        </div>
      )}

      {change.status === "SUBMITTED" && !showDecision && (
        <p className="rounded-lg border border-mist bg-lilac px-4 py-3 text-sm text-deep">
          Menunggu keputusan approver (Manager/Lead).
        </p>
      )}

      <div className="rounded-lg border border-mist bg-paper p-6">
        <h2 className="font-semibold">Riwayat Approval</h2>
        <ul className="mt-2 space-y-2 text-sm">
          {(approvalsQuery.data?.data ?? []).map((a) => (
            <li key={a.id} className="rounded-lg bg-chalk p-3">
              <span className="font-medium">{DECISION_LABEL[a.decision] ?? a.decision}</span>
              <span className="text-veil"> · revisi {a.revision}</span>
              {a.reason && <p className="mt-1 whitespace-pre-wrap">{a.reason}</p>}
              <p className="mt-1 text-xs text-veil">
                {(a.approver_id && userById.get(a.approver_id)?.name) || "sistem"} ·{" "}
                {formatTime(a.created_at)}
              </p>
            </li>
          ))}
          {approvalsQuery.data && approvalsQuery.data.data.length === 0 && (
            <li className="text-veil">Belum ada keputusan.</li>
          )}
        </ul>
      </div>

      <div className="rounded-lg border border-mist bg-paper p-6">
        <h2 className="font-semibold">Linked Incidents</h2>
        <ul className="mt-2 space-y-2 text-sm">
          {(linksQuery.data?.data ?? []).map((l) => (
            <li key={l.id} className="flex flex-wrap items-center gap-2 rounded-lg bg-chalk p-3">
              <span className="rounded-full border border-mist bg-paper px-2 py-0.5 text-xs font-semibold text-deep">
                {l.relation}
              </span>
              <Link to={`/incidents/${l.incident_id}`} className="font-mono text-xs text-iris hover:underline">
                {l.incident_no}
              </Link>
              <span className="min-w-0 flex-1 truncate">{l.incident_title}</span>
              <StatusBadge status={l.incident_status} />
            </li>
          ))}
          {linksQuery.data && linksQuery.data.data.length === 0 && (
            <li className="text-veil">Belum ada incident yang ditautkan.</li>
          )}
        </ul>
      </div>

      {showCancel && (
        <div className="rounded-lg border border-mist bg-paper p-6">
          <h2 className="font-semibold">Batalkan Change</h2>
          <form onSubmit={submitCancel} className="mt-2 flex flex-wrap gap-2">
            <input
              value={cancelReason}
              onChange={(e) => setCancelReason(e.target.value)}
              placeholder="Alasan cancel (wajib)"
              className={`flex-1 ${inputClass}`}
            />
            <button
              type="submit"
              disabled={anyPending}
              className="rounded-full border border-amber-500 px-4 py-2 text-sm font-semibold text-amber-700 hover:bg-amber-50 disabled:opacity-50"
            >
              {cancelMutation.isPending ? "…" : "Cancel Change"}
            </button>
          </form>
        </div>
      )}

      <div className="rounded-lg border border-mist bg-paper p-6">
        <h2 className="font-semibold">Komentar</h2>
        <ul className="mt-2 space-y-3">
          {(commentsQuery.data?.data ?? []).map((c) => (
            <li key={c.id} className="rounded-lg bg-chalk p-3 text-sm">
              <p className="whitespace-pre-wrap">{c.body}</p>
              <p className="mt-1 text-xs text-veil">
                {(c.author_id && userById.get(c.author_id)?.name) || "?"} · {formatTime(c.created_at)}
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
            className={`flex-1 ${inputClass}`}
          />
          <button type="submit" disabled={commentMutation.isPending} className={primaryButton}>
            {commentMutation.isPending ? "…" : "Kirim"}
          </button>
        </form>
      </div>

      <div className="rounded-lg border border-mist bg-paper p-6">
        <h2 className="font-semibold">Timeline</h2>
        <ul className="mt-2 space-y-2 text-sm">
          {(timelineQuery.data?.data ?? []).map((a) => (
            <li key={a.id} className="flex justify-between gap-2 border-b border-mist py-1 last:border-0">
              <span>
                {activityLabel(a.type, a.from_status, a.to_status, a.payload)}
                <span className="text-veil">
                  {" "}
                  · {(a.actor_id && userById.get(a.actor_id)?.name) || "sistem"}
                </span>
              </span>
              <span className="shrink-0 text-xs text-veil">{formatTime(a.created_at)}</span>
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
