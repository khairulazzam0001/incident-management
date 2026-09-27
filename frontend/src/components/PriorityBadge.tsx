const COLORS: Record<string, string> = {
  P1: "bg-red-100 text-red-700",
  P2: "bg-orange-100 text-orange-700",
  P3: "bg-lilac text-iris",
  P4: "bg-chalk text-veil",
};

export function PriorityBadge({ priority }: { priority: string }) {
  const cls = COLORS[priority] ?? "bg-chalk text-veil";
  return (
    <span className={`rounded-full border border-mist px-2 py-0.5 text-xs font-semibold ${cls}`}>
      {priority}
    </span>
  );
}
