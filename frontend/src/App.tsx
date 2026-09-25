import { Navigate, Route, Routes } from "react-router-dom";
import type { ReactElement } from "react";
import { useAuth } from "./auth/AuthContext";
import { ApiStatus } from "./components/ApiStatus";
import { IncidentDetail } from "./pages/IncidentDetail";
import { IncidentList } from "./pages/IncidentList";
import { IncidentNew } from "./pages/IncidentNew";
import { Login } from "./pages/Login";

function RequireAuth({ children }: { children: ReactElement }) {
  const { user, isLoading } = useAuth();
  if (isLoading) {
    return (
      <div className="mx-auto max-w-3xl p-6" aria-label="Memuat">
        <div className="h-8 w-1/3 animate-pulse rounded bg-slate-200" />
      </div>
    );
  }
  if (user === null) return <Navigate to="/login" replace />;
  return children;
}

export function App() {
  const { user, logout } = useAuth();

  return (
    <div className="min-h-screen bg-slate-50 text-slate-900">
      <header className="flex items-center justify-between border-b bg-white px-6 py-3">
        <span className="font-semibold">Incident Management</span>
        <div className="flex items-center gap-3">
          <ApiStatus />
          {user !== null && (
            <>
              <span className="text-sm text-slate-600">
                {user.name} ({user.role})
              </span>
              <button
                onClick={logout}
                className="rounded border px-2 py-1 text-xs hover:bg-slate-100"
              >
                Keluar
              </button>
            </>
          )}
        </div>
      </header>
      <main>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/"
            element={
              <RequireAuth>
                <IncidentList />
              </RequireAuth>
            }
          />
          <Route
            path="/incidents/new"
            element={
              <RequireAuth>
                <IncidentNew />
              </RequireAuth>
            }
          />
          <Route
            path="/incidents/:id"
            element={
              <RequireAuth>
                <IncidentDetail />
              </RequireAuth>
            }
          />
        </Routes>
      </main>
    </div>
  );
}
