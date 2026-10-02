import type {
  ApiErrorBody,
  ApprovalDecision,
  Attachment,
  Change,
  ChangeActivity,
  ChangeApproval,
  ChangeComment,
  ChangeIncidentLink,
  ChangeInput,
  ChangeListResponse,
  ChangeOutcome,
  ChangeRelation,
  ChangeSummary,
  ScheduleResult,
  DataList,
  AttachmentListResponse,
  Comment,
  CommentListResponse,
  CreateIncidentInput,
  Dashboard,
  Fix,
  FixListResponse,
  HealthResponse,
  Incident,
  IncidentListResponse,
  Investigation,
  InvestigationListResponse,
  LoginResponse,
  Meta,
  NotificationListResponse,
  TimelineResponse,
  UserListResponse,
  Verification,
  VerificationListResponse,
} from "./types";

const BASE_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";
const TOKEN_KEY = "im_token";
const USER_KEY = "im_user";

export function getStoredToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setStoredToken(token: string | null): void {
  try {
    if (token === null) localStorage.removeItem(TOKEN_KEY);
    else localStorage.setItem(TOKEN_KEY, token);
  } catch {
    // abaikan: storage tidak tersedia
  }
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details: unknown;
  readonly requestId: string | undefined;

  constructor(status: number, body: ApiErrorBody) {
    super(body.message);
    this.name = "ApiError";
    this.status = status;
    this.code = body.code;
    this.details = body.details;
    this.requestId = body.request_id;
  }
}

function isApiErrorBody(v: unknown): v is ApiErrorBody {
  if (typeof v !== "object" || v === null) return false;
  const o = v as Record<string, unknown>;
  return typeof o.code === "string" && typeof o.message === "string";
}

interface RequestOptions {
  method?: string;
  body?: unknown;
  signal?: AbortSignal;
}

async function request<T>(path: string, options?: RequestOptions): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  const token = getStoredToken();
  if (token !== null) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(`${BASE_URL}${path}`, {
    method: options?.method,
    headers,
    body: options?.body === undefined ? undefined : JSON.stringify(options.body),
    signal: options?.signal,
  });
  const data: unknown = await res.json().catch(() => null);
  if (!res.ok) {
    if (isApiErrorBody(data)) throw new ApiError(res.status, data);
    throw new ApiError(res.status, {
      code: "UNKNOWN_ERROR",
      message: `Request gagal (HTTP ${res.status}).`,
    });
  }
  return data as T;
}

export interface IncidentFilters {
  status?: string;
  severity?: string;
  priority?: string;
  q?: string;
  application?: string;
  team?: string;
  assignee?: string;
  created_from?: string;
  created_to?: string;
  sort?: string;
  order?: string;
  page?: number;
  limit?: number;
  signal?: AbortSignal;
}

function toQuery(f: IncidentFilters): string {
  const params = new URLSearchParams();
  if (f.status) params.set("status", f.status);
  if (f.severity) params.set("severity", f.severity);
  if (f.priority) params.set("priority", f.priority);
  if (f.q) params.set("q", f.q);
  if (f.application) params.set("application", f.application);
  if (f.team) params.set("team", f.team);
  if (f.assignee) params.set("assignee", f.assignee);
  if (f.created_from) params.set("created_from", f.created_from);
  if (f.created_to) params.set("created_to", f.created_to);
  if (f.sort) params.set("sort", f.sort);
  if (f.order) params.set("order", f.order);
  if (f.page !== undefined) params.set("page", String(f.page));
  if (f.limit !== undefined) params.set("limit", String(f.limit));
  const s = params.toString();
  return s ? `?${s}` : "";
}

export interface ChangeFilters {
  status?: string;
  type?: string;
  risk?: string;
  application_id?: string;
  environment?: string;
  requester?: string;
  implementer?: string;
  scheduled_from?: string;
  scheduled_to?: string;
  sort?: string;
  q?: string;
  page?: number;
  limit?: number;
  signal?: AbortSignal;
}

function changeQuery(f: ChangeFilters): string {
  const params = new URLSearchParams();
  const keys = [
    "status",
    "type",
    "risk",
    "application_id",
    "environment",
    "requester",
    "implementer",
    "scheduled_from",
    "scheduled_to",
    "sort",
    "q",
  ] as const;
  for (const k of keys) {
    const v = f[k];
    if (v) params.set(k, v);
  }
  if (f.page !== undefined) params.set("page", String(f.page));
  if (f.limit !== undefined) params.set("limit", String(f.limit));
  const s = params.toString();
  return s ? `?${s}` : "";
}

export interface AssignInput {
  team_id?: string | null;
  pic_id?: string | null;
  note?: string;
}

export const api = {
  login(email: string, password: string): Promise<LoginResponse> {
    return request<LoginResponse>("/api/auth/login", {
      method: "POST",
      body: { email, password },
    });
  },
  getMeta(signal?: AbortSignal): Promise<Meta> {
    return request<Meta>("/api/meta", { signal });
  },
  getUsers(signal?: AbortSignal): Promise<UserListResponse> {
    return request<UserListResponse>("/api/users", { signal });
  },
  listIncidents(f: IncidentFilters): Promise<IncidentListResponse> {
    return request<IncidentListResponse>(`/api/incidents${toQuery(f)}`, {
      signal: f.signal,
    });
  },
  getIncident(id: string, signal?: AbortSignal): Promise<Incident> {
    return request<Incident>(`/api/incidents/${id}`, { signal });
  },
  createIncident(input: CreateIncidentInput): Promise<Incident> {
    return request<Incident>("/api/incidents", { method: "POST", body: input });
  },
  changeStatus(id: string, status: string): Promise<Incident> {
    return request<Incident>(`/api/incidents/${id}/status`, {
      method: "PATCH",
      body: { status },
    });
  },
  assignIncident(id: string, input: AssignInput): Promise<Incident> {
    return request<Incident>(`/api/incidents/${id}/assignment`, {
      method: "PATCH",
      body: input,
    });
  },
  addComment(id: string, body: string): Promise<Comment> {
    return request<Comment>(`/api/incidents/${id}/comments`, {
      method: "POST",
      body: { body },
    });
  },
  getComments(id: string, signal?: AbortSignal): Promise<CommentListResponse> {
    return request<CommentListResponse>(`/api/incidents/${id}/comments`, { signal });
  },
  getTimeline(id: string, signal?: AbortSignal): Promise<TimelineResponse> {
    return request<TimelineResponse>(`/api/incidents/${id}/timeline`, { signal });
  },
  addInvestigation(
    id: string,
    input: { notes: string; findings: string },
  ): Promise<Investigation> {
    return request<Investigation>(`/api/incidents/${id}/investigations`, {
      method: "POST",
      body: input,
    });
  },
  getInvestigations(id: string, signal?: AbortSignal): Promise<InvestigationListResponse> {
    return request<InvestigationListResponse>(`/api/incidents/${id}/investigations`, {
      signal,
    });
  },
  addFix(id: string, input: { description: string; reference: string }): Promise<Fix> {
    return request<Fix>(`/api/incidents/${id}/fixes`, { method: "POST", body: input });
  },
  getFixes(id: string, signal?: AbortSignal): Promise<FixListResponse> {
    return request<FixListResponse>(`/api/incidents/${id}/fixes`, { signal });
  },
  verify(id: string, input: { result: string; reason: string }): Promise<Verification> {
    return request<Verification>(`/api/incidents/${id}/verification`, {
      method: "POST",
      body: input,
    });
  },
  getVerifications(id: string, signal?: AbortSignal): Promise<VerificationListResponse> {
    return request<VerificationListResponse>(`/api/incidents/${id}/verifications`, {
      signal,
    });
  },
  closeIncident(id: string): Promise<Incident> {
    return request<Incident>(`/api/incidents/${id}/close`, { method: "POST" });
  },
  reopenIncident(id: string, reason: string): Promise<Incident> {
    return request<Incident>(`/api/incidents/${id}/reopen`, {
      method: "POST",
      body: { reason },
    });
  },
  getHealth(signal?: AbortSignal): Promise<HealthResponse> {
    return request<HealthResponse>("/health", { signal });
  },
  getAttachments(id: string, signal?: AbortSignal): Promise<AttachmentListResponse> {
    return request<AttachmentListResponse>(`/api/incidents/${id}/attachments`, {
      signal,
    });
  },
  async uploadAttachment(id: string, file: File): Promise<Attachment> {
    const headers: Record<string, string> = {};
    const token = getStoredToken();
    if (token !== null) headers.Authorization = `Bearer ${token}`;
    const form = new FormData();
    form.append("file", file);
    const res = await fetch(`${BASE_URL}/api/incidents/${id}/attachments`, {
      method: "POST",
      headers,
      body: form,
    });
    const data: unknown = await res.json().catch(() => null);
    if (!res.ok) {
      if (isApiErrorBody(data)) throw new ApiError(res.status, data);
      throw new ApiError(res.status, {
        code: "UNKNOWN_ERROR",
        message: `Upload gagal (HTTP ${res.status}).`,
      });
    }
    return data as Attachment;
  },
  async downloadAttachment(id: string, aid: string): Promise<Blob> {
    const headers: Record<string, string> = {};
    const token = getStoredToken();
    if (token !== null) headers.Authorization = `Bearer ${token}`;
    const res = await fetch(
      `${BASE_URL}/api/incidents/${id}/attachments/${aid}/download`,
      { headers },
    );
    if (!res.ok) {
      const data: unknown = await res.json().catch(() => null);
      if (isApiErrorBody(data)) throw new ApiError(res.status, data);
      throw new ApiError(res.status, {
        code: "UNKNOWN_ERROR",
        message: `Unduh gagal (HTTP ${res.status}).`,
      });
    }
    return res.blob();
  },
  getDashboard(signal?: AbortSignal): Promise<Dashboard> {
    return request<Dashboard>("/api/dashboard", { signal });
  },
  getNotifications(unreadOnly: boolean, signal?: AbortSignal): Promise<NotificationListResponse> {
    return request<NotificationListResponse>(
      `/api/notifications${unreadOnly ? "?unread=true" : ""}`,
      { signal },
    );
  },
  markNotificationRead(id: string): Promise<{ ok: boolean }> {
    return request<{ ok: boolean }>(`/api/notifications/${id}/read`, {
      method: "POST",
    });
  },
  createApplication(input: { code: string; name: string }) {
    return request(`/api/master/applications`, { method: "POST", body: input });
  },
  deleteApplication(id: string): Promise<void> {
    return request<void>(`/api/master/applications/${id}`, { method: "DELETE" });
  },
  createTeam(input: { code: string; name: string }) {
    return request(`/api/master/teams`, { method: "POST", body: input });
  },
  deleteTeam(id: string): Promise<void> {
    return request<void>(`/api/master/teams/${id}`, { method: "DELETE" });
  },
  listChanges(f: ChangeFilters): Promise<ChangeListResponse> {
    return request<ChangeListResponse>(`/api/changes${changeQuery(f)}`, { signal: f.signal });
  },
  getChange(id: string, signal?: AbortSignal): Promise<Change> {
    return request<Change>(`/api/changes/${id}`, { signal });
  },
  createChange(input: ChangeInput): Promise<Change> {
    return request<Change>("/api/changes", { method: "POST", body: input });
  },
  updateChange(id: string, input: ChangeInput): Promise<Change> {
    return request<Change>(`/api/changes/${id}`, { method: "PATCH", body: input });
  },
  submitChange(id: string): Promise<Change> {
    return request<Change>(`/api/changes/${id}/submit`, { method: "POST" });
  },
  decideChange(id: string, decision: ApprovalDecision, reason: string): Promise<Change> {
    return request<Change>(`/api/changes/${id}/approval`, {
      method: "POST",
      body: { decision, reason },
    });
  },
  cancelChange(id: string, reason: string): Promise<Change> {
    return request<Change>(`/api/changes/${id}/cancel`, { method: "POST", body: { reason } });
  },
  getChangeApprovals(id: string, signal?: AbortSignal): Promise<DataList<ChangeApproval>> {
    return request<DataList<ChangeApproval>>(`/api/changes/${id}/approvals`, { signal });
  },
  getChangeTimeline(id: string, signal?: AbortSignal): Promise<DataList<ChangeActivity>> {
    return request<DataList<ChangeActivity>>(`/api/changes/${id}/timeline`, { signal });
  },
  getChangeComments(id: string, signal?: AbortSignal): Promise<DataList<ChangeComment>> {
    return request<DataList<ChangeComment>>(`/api/changes/${id}/comments`, { signal });
  },
  addChangeComment(id: string, body: string): Promise<ChangeComment> {
    return request<ChangeComment>(`/api/changes/${id}/comments`, {
      method: "POST",
      body: { body },
    });
  },
  getChangeIncidents(id: string, signal?: AbortSignal): Promise<DataList<ChangeIncidentLink>> {
    return request<DataList<ChangeIncidentLink>>(`/api/changes/${id}/incidents`, { signal });
  },
  scheduleChange(id: string, plannedStart: string, plannedEnd: string): Promise<ScheduleResult> {
    return request<ScheduleResult>(`/api/changes/${id}/schedule`, {
      method: "POST",
      body: { planned_start: plannedStart, planned_end: plannedEnd },
    });
  },
  startChange(id: string): Promise<Change> {
    return request<Change>(`/api/changes/${id}/start`, { method: "POST" });
  },
  completeChange(id: string, outcome: ChangeOutcome, outcomeNotes: string): Promise<Change> {
    return request<Change>(`/api/changes/${id}/complete`, {
      method: "POST",
      body: { outcome, outcome_notes: outcomeNotes },
    });
  },
  closeChange(id: string, reviewNotes: string): Promise<Change> {
    return request<Change>(`/api/changes/${id}/close`, {
      method: "POST",
      body: { review_notes: reviewNotes },
    });
  },
  linkIncident(
    id: string,
    incidentId: string,
    relation: ChangeRelation,
  ): Promise<DataList<ChangeIncidentLink>> {
    return request<DataList<ChangeIncidentLink>>(`/api/changes/${id}/incidents`, {
      method: "POST",
      body: { incident_id: incidentId, relation },
    });
  },
  unlinkIncident(id: string, incidentId: string, relation: ChangeRelation): Promise<{ ok: boolean }> {
    return request<{ ok: boolean }>(
      `/api/changes/${id}/incidents/${incidentId}?relation=${relation}`,
      { method: "DELETE" },
    );
  },
  getChangeSummary(signal?: AbortSignal): Promise<ChangeSummary> {
    return request<ChangeSummary>("/api/changes/summary", { signal });
  },
  getRecentChanges(id: string, signal?: AbortSignal): Promise<DataList<Change>> {
    return request<DataList<Change>>(`/api/incidents/${id}/recent-changes`, { signal });
  },
  getIncidentChanges(id: string, signal?: AbortSignal): Promise<DataList<ChangeIncidentLink>> {
    return request<DataList<ChangeIncidentLink>>(`/api/incidents/${id}/changes`, { signal });
  },
};

export { TOKEN_KEY, USER_KEY };
