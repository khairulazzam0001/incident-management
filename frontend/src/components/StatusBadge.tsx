const COLORS: Record<string, string> = {
  NEW: "bg-slate-200 text-slate-700",
  ASSIGNED: "bg-blue-100 text-blue-700",
  INVESTIGATING: "bg-amber-100 text-amber-800",
  FIXING: "bg-orange-100 text-orange-700",
  VERIFYING: "bg-purple-100 text-purple-700",
  RESOLVED: "bg-green-100 text-green-700",
  CLOSED: "bg-slate-100 text-slate-500",
};

export function StatusBadge({ status }: { status: string }) {
  const cls = COLORS[status] ?? "bg-slate-200 text-slate-700";
  return (
    <span className={`rounded px-2 py-1 text-xs font-medium ${cls}`}>{status}</span>
  );
}
