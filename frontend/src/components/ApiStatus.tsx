import { useHealth } from "../hooks/useHealth";

export function ApiStatus() {
  const { data, isPending, isError } = useHealth();

  if (isPending) {
    return (
      <span className="rounded-full bg-chalk px-2 py-1 text-xs text-veil">
        API: menghubungkan…
      </span>
    );
  }
  if (isError || data?.status !== "ok") {
    return (
      <span className="rounded-full bg-red-100 px-2 py-1 text-xs font-semibold text-red-700">
        API: tidak terjangkau
      </span>
    );
  }
  return (
    <span className="rounded-full bg-green-100 px-2 py-1 text-xs font-semibold text-green-700">
      API: OK
    </span>
  );
}
