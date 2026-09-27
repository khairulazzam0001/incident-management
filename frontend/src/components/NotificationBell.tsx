import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../api/client";
import { BellIcon } from "./icons";

export function NotificationBell() {
  const { data } = useQuery({
    queryKey: ["notifications", "unread-count"],
    queryFn: ({ signal }) => api.getNotifications(true, signal),
    refetchInterval: 30_000,
    staleTime: 15_000,
  });

  const unread = data?.unread ?? 0;

  return (
    <Link
      to="/notifications"
      className="relative rounded-full border border-mist bg-paper p-2 text-deep transition-colors duration-200 hover:bg-lilac focus-visible:outline-2 focus-visible:outline-iris"
      aria-label={`Notifikasi (${unread} belum dibaca)`}
    >
      <BellIcon />
      {unread > 0 && (
        <span className="absolute -right-1 -top-1 rounded-full bg-iris px-1 text-[10px] font-bold text-white">
          {unread > 99 ? "99+" : unread}
        </span>
      )}
    </Link>
  );
}
