import type { SLAState, SLASummary } from "../api/types";
import { useNow } from "../hooks/useNow";

// Badge SLA ringkas (PRD_SLA_Escalation.md §11): hijau on track, kuning
// lewat ambang peringatan, merah breach, abu terpenuhi.

export function formatDuration(ms: number): string {
  const totalMin = Math.max(0, Math.round(Math.abs(ms) / 60_000));
  const d = Math.floor(totalMin / 1440);
  const h = Math.floor((totalMin % 1440) / 60);
  const m = totalMin % 60;
  if (d > 0) return `${d}h ${h}j`;
  if (h > 0) return `${h}j ${m}m`;
  return `${m}m`;
}

export type SlaTone = "ok" | "warn" | "breach" | "met" | "none";

export function slaTone(st: SLAState, now: number): SlaTone {
  if (st.status === "BREACHED") return "breach";
  if (st.status === "MET") return "met";
  if (st.status !== "RUNNING") return "none";
  if (now >= new Date(st.target_at).getTime()) return "breach";
  if (now >= new Date(st.warn_at).getTime()) return "warn";
  return "ok";
}

export const TONE_CLASS: Record<SlaTone, string> = {
  ok: "bg-green-100 text-green-700",
  warn: "bg-amber-100 text-amber-800",
  breach: "bg-red-100 text-red-700",
  met: "bg-chalk text-veil",
  none: "bg-chalk text-veil",
};

// Metrik yang relevan: response selama belum berhenti, lalu resolution.
function activeState(sla: SLASummary): { label: string; st: SLAState } | null {
  if (sla.response && sla.response.stopped_at === null && sla.response.status !== "CANCELLED") {
    return { label: "Response", st: sla.response };
  }
  if (sla.resolution) return { label: "Resolution", st: sla.resolution };
  return null;
}

export function SlaBadge({ sla }: { sla: SLASummary | null }) {
  const now = useNow();
  if (sla === null) return null;
  const active = activeState(sla);
  if (active === null) return null;
  const tone = slaTone(active.st, now);
  if (tone === "none") return null;
  const remaining = new Date(active.st.target_at).getTime() - now;
  const text =
    tone === "met"
      ? "SLA terpenuhi"
      : tone === "breach"
        ? `${active.label} breach`
        : `${active.label} · sisa ${formatDuration(remaining)}`;
  return (
    <span
      className={`rounded-full border border-mist px-2 py-0.5 text-xs font-semibold ${TONE_CLASS[tone]}`}
      title={`Target ${active.label.toLowerCase()}: ${new Date(active.st.target_at).toLocaleString("id-ID")}`}
    >
      {text}
    </span>
  );
}
