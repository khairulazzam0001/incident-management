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
  change_types: MasterItem[];
  change_risks: MasterItem[];
  change_statuses: MasterItem[];
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

export interface Attachment {
  id: string;
  incident_id: string;
  file_name: string;
  mime_type: string;
  size_bytes: number;
  uploaded_by: string | null;
  created_at: string;
}

export interface AttachmentListResponse {
  data: Attachment[];
}

export interface Notification {
  id: string;
  // Salah satu dari incident_id / change_id terisi (PRD_Change_Management.md §13).
  incident_id: string | null;
  change_id: string | null;
  type: string;
  recipient_id: string;
  actor_id: string | null;
  read_at: string | null;
  email_status: string;
  created_at: string;
}

export interface NotificationListResponse {
  data: Notification[];
  unread: number;
}

export interface NamedCount {
  id: string | null;
  name: string;
  count: number;
}

export interface Dashboard {
  open_total: number;
  by_status: Record<string, number>;
  by_severity: Record<string, number>;
  by_priority: Record<string, number>;
  by_application: NamedCount[];
  by_team: NamedCount[];
  my_open: number;
  avg_hours_create_to_assign: number | null;
  avg_hours_create_to_resolve: number | null;
  avg_hours_create_to_close: number | null;
  reopen_rate: number | null;
  verification_failure_rate: number | null;
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

// ===== Change Management (PRD_Change_Management.md) =====

export type ChangeStatus =
  | "DRAFT"
  | "SUBMITTED"
  | "APPROVED"
  | "SCHEDULED"
  | "IMPLEMENTING"
  | "REVIEWING"
  | "CLOSED"
  | "REJECTED"
  | "CANCELLED";

export type ChangeType = "STANDARD" | "NORMAL" | "EMERGENCY";
export type ChangeRisk = "LOW" | "MEDIUM" | "HIGH";
export type ApprovalDecision = "APPROVE" | "REJECT" | "REQUEST_CHANGES";

export interface Change {
  id: string;
  change_no: string;
  title: string;
  description: string;
  justification: string;
  type: ChangeType;
  risk: ChangeRisk;
  status: ChangeStatus;
  application_id: string;
  environment: string;
  implementation_plan: string;
  rollback_plan: string;
  test_plan: string;
  requester_id: string;
  implementer_id: string | null;
  team_id: string | null;
  revision: number;
  planned_start: string | null;
  planned_end: string | null;
  actual_start: string | null;
  actual_end: string | null;
  outcome: "SUCCESS" | "FAILED" | "ROLLED_BACK" | null;
  outcome_notes: string;
  review_notes: string;
  created_at: string;
  updated_at: string;
  closed_at: string | null;
  closed_by: string | null;
}

export interface ChangeInput {
  title: string;
  description: string;
  justification: string;
  type: ChangeType | "";
  risk: ChangeRisk | "";
  application_id: string;
  environment: string;
  implementation_plan: string;
  rollback_plan: string;
  test_plan: string;
  implementer_id: string | null;
  team_id: string | null;
  incident_id?: string | null;
}

export interface ChangeListResponse {
  data: Change[];
  meta: PageMeta;
}

export interface ChangeApproval {
  id: string;
  change_id: string;
  approver_id: string | null;
  decision: "APPROVED" | "REJECTED" | "CHANGES_REQUESTED" | "AUTO_APPROVED";
  reason: string;
  revision: number;
  created_at: string;
}

export interface ChangeActivity {
  id: string;
  change_id: string;
  type: string;
  actor_id: string | null;
  from_status: string | null;
  to_status: string | null;
  payload: unknown;
  created_at: string;
}

export interface ChangeComment {
  id: string;
  change_id: string;
  author_id: string | null;
  body: string;
  created_at: string;
}

export interface ChangeIncidentLink {
  id: string;
  change_id: string;
  change_no: string;
  change_title: string;
  change_status: ChangeStatus;
  incident_id: string;
  incident_no: string;
  incident_title: string;
  incident_status: IncidentStatus;
  relation: "FIX_FOR" | "CAUSED_BY";
  created_by: string | null;
  created_at: string;
}

export type ChangeOutcome = "SUCCESS" | "FAILED" | "ROLLED_BACK";
export type ChangeRelation = "FIX_FOR" | "CAUSED_BY";

export interface ScheduleResult {
  change: Change;
  conflicts: Change[];
}

export interface ChangeAttachment {
  id: string;
  change_id: string;
  file_name: string;
  mime_type: string;
  size_bytes: number;
  uploaded_by: string | null;
  created_at: string;
}

export interface ChangeSummary {
  active_total: number;
  by_status: Record<string, number>;
  by_type: Record<string, number>;
  by_risk: Record<string, number>;
  upcoming: Change[];
  success_rate: number | null;
  failure_rate: number | null;
  emergency_ratio: number | null;
  avg_hours_submit_to_approve: number | null;
  avg_hours_approve_to_implement: number | null;
  pir_completion_rate: number | null;
}

export interface DataList<T> {
  data: T[];
}
