const COLORS: Record<string, string> = {
  S1: "bg-red-100 text-red-700",
  S2: "bg-orange-100 text-orange-700",
  S3: "bg-yellow-100 text-yellow-800",
  S4: "bg-chalk text-veil",
};

export function SeverityBadge({ severity }: { severity: string }) {
  const cls = COLORS[severity] ?? "bg-chalk text-veil";
  return (
    <span className={`rounded-full border border-mist px-2 py-0.5 text-xs font-semibold ${cls}`}>
      {severity}
    </span>
  );
}
