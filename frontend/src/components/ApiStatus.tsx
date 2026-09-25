import { useHealth } from "../hooks/useHealth";

export function ApiStatus() {
  const { data, isPending, isError } = useHealth();

  if (isPending) {
    return (
      <span className="rounded bg-slate-200 px-2 py-1 text-xs text-slate-600">
        API: menghubungkan…
      </span>
    );
  }
  if (isError || data?.status !== "ok") {
    return (
      <span className="rounded bg-red-100 px-2 py-1 text-xs text-red-700">
        API: tidak terjangkau
      </span>
    );
  }
  return (
    <span className="rounded bg-green-100 px-2 py-1 text-xs text-green-700">
      API: OK
    </span>
  );
}
