import { useLocation } from "react-router-dom";
import { ApiStatus } from "./ApiStatus";
import { NotificationBell } from "./NotificationBell";
import { MenuIcon } from "./icons";

const TITLES: Record<string, string> = {
  "/": "Incidents",
  "/dashboard": "Dashboard",
  "/my": "My Incidents",
  "/master": "Master Data",
  "/notifications": "Notifikasi",
  "/incidents/new": "Buat Incident",
};

function titleFor(pathname: string): string {
  if (pathname.startsWith("/incidents/") && pathname !== "/incidents/new") {
    return "Detail Incident";
  }
  return TITLES[pathname] ?? "Incident Management";
}

export function Topbar({ onOpenMenu }: { onOpenMenu: () => void }) {
  const { pathname } = useLocation();

  return (
    <div className="sticky top-0 z-30 bg-paper shadow-sm">
      <div className="flex h-14 items-center gap-3 px-4 lg:px-6">
        <button
          type="button"
          onClick={onOpenMenu}
          aria-label="Buka menu navigasi"
          className="rounded-full p-2 text-deep transition-colors duration-200 hover:bg-chalk focus-visible:outline-2 focus-visible:outline-iris lg:hidden"
        >
          <MenuIcon />
        </button>
        <h1 className="text-base font-semibold text-ink">{titleFor(pathname)}</h1>
        <div className="ml-auto flex items-center gap-3">
          <ApiStatus />
          <NotificationBell />
        </div>
      </div>
    </div>
  );
}
