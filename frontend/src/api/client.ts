import type {
  ApiErrorBody,
  Comment,
  CommentListResponse,
  CreateIncidentInput,
  Fix,
  FixListResponse,
  HealthResponse,
  Incident,
  IncidentListResponse,
  Investigation,
  InvestigationListResponse,
  LoginResponse,
  Meta,
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
  if (f.sort) params.set("sort", f.sort);
  if (f.order) params.set("order", f.order);
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
};

export { TOKEN_KEY, USER_KEY };
