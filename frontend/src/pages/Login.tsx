import { useState } from "react";
import type { FormEvent } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { ApiError } from "../api/client";
import { useAuth } from "../auth/AuthContext";

export function Login() {
  const { user, login } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isPending, setIsPending] = useState(false);

  if (user !== null) return <Navigate to="/" replace />;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setIsPending(true);
    try {
      await login(email.trim(), password);
      navigate("/", { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Login gagal. Coba lagi.");
    } finally {
      setIsPending(false);
    }
  }

  return (
    <section className="mx-auto mt-16 max-w-sm rounded-lg border border-mist bg-paper p-6 shadow-md">
      <h1 className="text-2xl font-semibold">Masuk</h1>
      <p className="mt-1 text-sm text-veil">Incident Management</p>
      <form onSubmit={onSubmit} className="mt-6 space-y-4">
        <label className="block">
          <span className="text-sm font-medium">Email</span>
          <input
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="mt-1 w-full rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none"
            autoComplete="username"
          />
        </label>
        <label className="block">
          <span className="text-sm font-medium">Password</span>
          <input
            type="password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="mt-1 w-full rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none"
            autoComplete="current-password"
          />
        </label>
        {error !== null && (
          <p className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
        )}
        <button
          type="submit"
          disabled={isPending}
          className="w-full rounded-full bg-iris px-4 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50"
        >
          {isPending ? "Memeriksa…" : "Masuk"}
        </button>
      </form>
    </section>
  );
}
