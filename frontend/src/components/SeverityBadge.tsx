const COLORS: Record<string, string> = {
  S1: "bg-red-100 text-red-700",
  S2: "bg-orange-100 text-orange-700",
  S3: "bg-yellow-100 text-yellow-800",
  S4: "bg-slate-200 text-slate-600",
};

export function SeverityBadge({ severity }: { severity: string }) {
  const cls = COLORS[severity] ?? "bg-slate-200 text-slate-600";
  return (
    <span className={`rounded px-2 py-1 text-xs font-medium ${cls}`}>{severity}</span>
  );
}
