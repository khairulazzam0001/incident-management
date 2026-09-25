// Tipe domain (PRD §6–§7, §11). Kontrak: AGENTS.md §7.

export type IncidentStatus =
  | "NEW"
  | "ASSIGNED"
  | "INVESTIGATING"
  | "FIXING"
  | "VERIFYING"
  | "RESOLVED"
  | "CLOSED";

export type Severity = "S1" | "S2" | "S3" | "S4";
export type Priority = "P1" | "P2" | "P3" | "P4";

// Peta transisi untuk UX saja — backend yang menegakkan (PRD §6).
export const ALLOWED_TRANSITIONS: Record<IncidentStatus, IncidentStatus[]> = {
  NEW: ["ASSIGNED"],
  ASSIGNED: ["INVESTIGATING"],
  INVESTIGATING: ["FIXING"],
  FIXING: ["VERIFYING"],
  VERIFYING: ["RESOLVED", "FIXING"],
  RESOLVED: ["CLOSED"],
  CLOSED: [],
};

export interface HealthResponse {
  status: string;
  service: string;
}

// Bentuk error standar backend (PRD §16).
export interface ApiErrorBody {
  code: string;
  message: string;
  details?: unknown;
  request_id?: string;
}

export interface User {
  id: string;
  email: string;
  name: string;
  role: string;
  team_id: string | null;
  is_active: boolean;
}

export interface LoginResponse {
  token: string;
  user: User;
}

export interface Incident {
  id: string;
  incident_no: string;
  title: string;
  description: string;
  source: string;
  severity: Severity;
  priority: Priority;
  status: IncidentStatus;
  application_id: string | null;
  environment: string | null;
  reporter_id: string | null;
  team_id: string | null;
  pic_id: string | null;
  created_at: string;
  updated_at: string;
  resolved_at: string | null;
  closed_at: string | null;
  closed_by: string | null;
}

export interface Comment {
  id: string;
  incident_id: string;
  author_id: string | null;
  body: string;
  created_at: string;
}

export interface Activity {
  id: string;
  incident_id: string;
  type: string;
  actor_id: string | null;
  from_status: string | null;
  to_status: string | null;
  payload: unknown;
  created_at: string;
}

export interface MasterItem {
  code: string;
  name: string;
}

export interface MasterIDItem {
  id: string;
  code: string;
  name: string;
}

export interface Meta {
  statuses: MasterItem[];
  severities: MasterItem[];
  priorities: MasterItem[];
  environments: MasterItem[];
  sources: MasterItem[];
  applications: MasterIDItem[];
  teams: MasterIDItem[];
}

export interface PageMeta {
  page: number;
  limit: number;
  total: number;
}

export interface IncidentListResponse {
  data: Incident[];
  meta: PageMeta;
}

export interface TimelineResponse {
  data: Activity[];
}

export interface CommentListResponse {
  data: Comment[];
}

export interface UserListResponse {
  data: User[];
}

export interface Investigation {
  id: string;
  incident_id: string;
  author_id: string | null;
  notes: string;
  findings: string;
  created_at: string;
}

export interface Fix {
  id: string;
  incident_id: string;
  author_id: string | null;
  description: string;
  reference: string;
  created_at: string;
}

export interface Verification {
  id: string;
  incident_id: string;
  verifier_id: string | null;
  result: "PASS" | "FAIL";
  reason: string;
  created_at: string;
}

export interface InvestigationListResponse {
  data: Investigation[];
}

export interface FixListResponse {
  data: Fix[];
}

export interface VerificationListResponse {
  data: Verification[];
}

export interface CreateIncidentInput {
  title: string;
  description: string;
  source: string;
  severity: Severity | "";
  priority: Priority;
  application_id?: string | null;
  environment?: string | null;
}
