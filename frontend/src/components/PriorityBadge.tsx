const COLORS: Record<string, string> = {
  P1: "bg-red-100 text-red-700",
  P2: "bg-orange-100 text-orange-700",
  P3: "bg-blue-100 text-blue-700",
  P4: "bg-slate-200 text-slate-600",
};

export function PriorityBadge({ priority }: { priority: string }) {
  const cls = COLORS[priority] ?? "bg-slate-200 text-slate-600";
  return (
    <span className={`rounded px-2 py-1 text-xs font-medium ${cls}`}>{priority}</span>
  );
}
