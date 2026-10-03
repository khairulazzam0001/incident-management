import { Navigate, Route, Routes } from "react-router-dom";
import type { ReactElement } from "react";
import { useAuth } from "./auth/AuthContext";
import { AdminLayout } from "./components/AdminLayout";
import { ChangeCalendar } from "./pages/ChangeCalendar";
import { ChangeDetail } from "./pages/ChangeDetail";
import { ChangeList } from "./pages/ChangeList";
import { ChangeNew } from "./pages/ChangeNew";
import { Dashboard } from "./pages/Dashboard";
import { IncidentDetail } from "./pages/IncidentDetail";
import { IncidentList } from "./pages/IncidentList";
import { IncidentNew } from "./pages/IncidentNew";
import { Login } from "./pages/Login";
import { MasterData } from "./pages/MasterData";
import { MyIncidents } from "./pages/MyIncidents";
import { Notifications } from "./pages/Notifications";

function RequireAuth({ children }: { children: ReactElement }) {
  const { user, isLoading } = useAuth();
  if (isLoading) {
    return (
      <div className="mx-auto max-w-3xl p-6" aria-label="Memuat">
        <div className="h-8 w-1/3 animate-pulse rounded bg-lilac" />
      </div>
    );
  }
  if (user === null) return <Navigate to="/login" replace />;
  return children;
}

function Shell() {
  return (
    <AdminLayout>
      <>
        <Routes>
          <Route path="/" element={<IncidentList />} />
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/my" element={<MyIncidents />} />
          <Route path="/master" element={<MasterData />} />
          <Route path="/notifications" element={<Notifications />} />
          <Route path="/incidents/new" element={<IncidentNew />} />
          <Route path="/incidents/:id" element={<IncidentDetail />} />
          <Route path="/changes" element={<ChangeList />} />
          <Route path="/changes/new" element={<ChangeNew />} />
          <Route path="/changes/calendar" element={<ChangeCalendar />} />
          <Route path="/changes/:id" element={<ChangeDetail />} />
        </Routes>
      </>
    </AdminLayout>
  );
}

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        path="/*"
        element={
          <RequireAuth>
            <Shell />
          </RequireAuth>
        }
      />
    </Routes>
  );
}
