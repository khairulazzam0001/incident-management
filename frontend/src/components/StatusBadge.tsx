const COLORS: Record<string, string> = {
  NEW: "bg-chalk text-deep",
  ASSIGNED: "bg-lilac text-iris",
  INVESTIGATING: "bg-amber-100 text-amber-800",
  FIXING: "bg-orange-100 text-orange-700",
  VERIFYING: "bg-purple-100 text-purple-700",
  RESOLVED: "bg-green-100 text-green-700",
  CLOSED: "bg-chalk text-veil",
};

export function StatusBadge({ status }: { status: string }) {
  const cls = COLORS[status] ?? "bg-chalk text-deep";
  return (
    <span className={`rounded-full border border-mist px-2 py-0.5 text-xs font-semibold ${cls}`}>
      {status}
    </span>
  );
}
