import { useEffect, useState } from "react";
import type { ReactElement } from "react";
import { useLocation } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";

export function AdminLayout({ children }: { children: ReactElement }) {
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const { pathname } = useLocation();

  const { data } = useQuery({
    queryKey: ["notifications", "unread-count"],
    queryFn: ({ signal }) => api.getNotifications(true, signal),
    refetchInterval: 30_000,
    staleTime: 15_000,
  });

  // Tutup drawer mobile setiap pindah halaman (navigasi terprediksi).
  useEffect(() => {
    setMobileOpen(false);
  }, [pathname]);

  return (
    <div className="min-h-screen bg-chalk font-sans text-ink">
      {/* Sidebar desktop: statis, bisa diciutkan ke ikon */}
      <aside
        aria-hidden={false}
        className={`fixed inset-y-0 left-0 z-40 hidden transition-[width] duration-200 motion-reduce:transition-none lg:block ${
          collapsed ? "w-[76px]" : "w-64"
        }`}
      >
        <Sidebar
          collapsed={collapsed}
          onToggleCollapse={() => setCollapsed((c) => !c)}
          unread={data?.unread ?? 0}
        />
      </aside>

      {/* Drawer mobile */}
      {mobileOpen && (
        <button
          type="button"
          aria-label="Tutup menu navigasi"
          onClick={() => setMobileOpen(false)}
          className="fixed inset-0 z-40 bg-ink/40 lg:hidden"
        />
      )}
      <aside
        className={`fixed inset-y-0 left-0 z-50 w-72 transition-transform duration-200 motion-reduce:transition-none lg:hidden ${
          mobileOpen ? "translate-x-0" : "-translate-x-full"
        }`}
        aria-hidden={!mobileOpen}
      >
        <Sidebar
          collapsed={false}
          onToggleCollapse={() => setMobileOpen(false)}
          unread={data?.unread ?? 0}
        />
      </aside>

      <div
        className={`flex min-h-screen min-w-0 flex-col transition-[margin] duration-200 motion-reduce:transition-none ${
          collapsed ? "lg:ml-[76px]" : "lg:ml-64"
        }`}
      >
        <Topbar onOpenMenu={() => setMobileOpen(true)} />
        <main className="min-w-0 flex-1">{children}</main>
      </div>
    </div>
  );
}
