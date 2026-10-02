import { NavLink } from "react-router-dom";
import { canViewChanges, useAuth } from "../auth/AuthContext";
import {
  BellIcon,
  ChangeIcon,
  CollapseIcon,
  DashboardIcon,
  DatabaseIcon,
  LogoutIcon,
  TicketIcon,
  UserIcon,
} from "./icons";
import type { JSX } from "react";

interface NavItem {
  to: string;
  label: string;
  icon: (props: { className?: string }) => JSX.Element;
  badge?: number;
}

function linkClass({ isActive }: { isActive: boolean }) {
  return `relative flex min-h-[44px] items-center gap-3 rounded-lg px-3 text-sm transition-colors duration-200 focus-visible:outline-2 focus-visible:outline-iris ${
    isActive ? "bg-lilac font-semibold text-iris" : "text-deep hover:bg-chalk"
  }`;
}

export function Sidebar({
  collapsed,
  onToggleCollapse,
  unread,
}: {
  collapsed: boolean;
  onToggleCollapse: () => void;
  unread: number;
}) {
  const { user, logout } = useAuth();

  const items: NavItem[] = [
    { to: "/", label: "Incidents", icon: TicketIcon },
    ...(user !== null && canViewChanges(user.role)
      ? [{ to: "/changes", label: "Changes", icon: ChangeIcon }]
      : []),
    { to: "/dashboard", label: "Dashboard", icon: DashboardIcon },
    { to: "/my", label: "My Incidents", icon: UserIcon },
    { to: "/master", label: "Master Data", icon: DatabaseIcon },
    {
      to: "/notifications",
      label: "Notifikasi",
      icon: BellIcon,
      badge: unread > 0 ? unread : undefined,
    },
  ];

  const initial = (user?.name ?? "?").trim().charAt(0).toUpperCase() || "?";

  return (
    <div className="flex h-full flex-col bg-paper">
      <div className="flex h-14 items-center justify-between border-b border-mist px-4">
        {!collapsed && <span className="font-bold text-iris">Incident Management</span>}
        <button
          type="button"
          onClick={onToggleCollapse}
          aria-label={collapsed ? "Bentangkan sidebar" : "Ciutkan sidebar"}
          aria-expanded={!collapsed}
          className="rounded-full p-2 text-veil transition-colors duration-200 hover:bg-chalk focus-visible:outline-2 focus-visible:outline-iris"
        >
          <span className={collapsed ? "rotate-180" : ""} style={{ display: "inline-flex" }}>
            <CollapseIcon />
          </span>
        </button>
      </div>

      <nav aria-label="Navigasi utama" className="flex-1 space-y-1 overflow-y-auto p-3">
        {items.map((item) => (
          <NavLink key={item.to} to={item.to} end={item.to === "/"} className={linkClass} title={collapsed ? item.label : undefined}>
            <item.icon className="shrink-0" />
            {!collapsed && <span className="truncate">{item.label}</span>}
            {!collapsed && item.badge !== undefined && (
              <span className="ml-auto rounded-full bg-iris px-1.5 text-[10px] font-bold text-white">
                {item.badge > 99 ? "99+" : item.badge}
              </span>
            )}
            {collapsed && item.badge !== undefined && (
              <span className="absolute right-1 top-1 rounded-full bg-iris px-1 text-[10px] font-bold text-white">
                {item.badge > 99 ? "99+" : item.badge}
              </span>
            )}
          </NavLink>
        ))}
      </nav>

      <div className="border-t border-mist p-3">
        <div className="flex items-center gap-3">
          <span
            aria-hidden
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-lilac text-sm font-bold text-iris"
          >
            {initial}
          </span>
          {!collapsed && (
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-semibold text-ink">{user?.name}</p>
              <p className="truncate text-xs text-veil">{user?.role}</p>
            </div>
          )}
          <button
            type="button"
            onClick={logout}
            aria-label="Keluar"
            title="Keluar"
            className="rounded-full p-2 text-veil transition-colors duration-200 hover:bg-chalk focus-visible:outline-2 focus-visible:outline-iris"
          >
            <LogoutIcon />
          </button>
        </div>
      </div>
    </div>
  );
}
