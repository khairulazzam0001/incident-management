const COLORS: Record<string, string> = {
  LOW: "bg-green-100 text-green-700",
  MEDIUM: "bg-yellow-100 text-yellow-800",
  HIGH: "bg-red-100 text-red-700",
};

export function RiskBadge({ risk }: { risk: string }) {
  const cls = COLORS[risk] ?? "bg-chalk text-veil";
  return (
    <span className={`rounded-full border border-mist px-2 py-0.5 text-xs font-semibold ${cls}`}>
      Risk {risk}
    </span>
  );
}
