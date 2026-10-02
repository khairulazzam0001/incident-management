const COLORS: Record<string, string> = {
  DRAFT: "bg-chalk text-deep",
  SUBMITTED: "bg-amber-100 text-amber-800",
  APPROVED: "bg-lilac text-iris",
  SCHEDULED: "bg-sky-100 text-sky-700",
  IMPLEMENTING: "bg-orange-100 text-orange-700",
  REVIEWING: "bg-purple-100 text-purple-700",
  CLOSED: "bg-green-100 text-green-700",
  REJECTED: "bg-red-100 text-red-700",
  CANCELLED: "bg-chalk text-veil",
};

export function ChangeStatusBadge({ status }: { status: string }) {
  const cls = COLORS[status] ?? "bg-chalk text-deep";
  return (
    <span className={`rounded-full border border-mist px-2 py-0.5 text-xs font-semibold ${cls}`}>
      {status}
    </span>
  );
}
