const COLORS: Record<string, string> = {
  STANDARD: "bg-chalk text-veil",
  NORMAL: "bg-lilac text-iris",
  EMERGENCY: "bg-red-100 text-red-700",
};

export function ChangeTypeBadge({ type }: { type: string }) {
  const cls = COLORS[type] ?? "bg-chalk text-veil";
  return (
    <span className={`rounded-full border border-mist px-2 py-0.5 text-xs font-semibold ${cls}`}>
      {type}
    </span>
  );
}
