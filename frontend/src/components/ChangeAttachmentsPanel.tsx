import { useState } from "react";
import type { FormEvent } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import type { Change, User } from "../api/types";

// Attachment change request (PRD_Change_Management.md CM-FR-13): runbook,
// evidence test, screenshot PIR. Aturan file sama dengan incident (FR-09).

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KiB`;
  return `${(n / 1024 / 1024).toFixed(1)} MiB`;
}

export function ChangeAttachmentsPanel({
  change,
  user,
  userById,
  onDone,
  onError,
}: {
  change: Change;
  user: User | null;
  userById: Map<string, User>;
  onDone: (message: string) => void;
  onError: (err: unknown) => void;
}) {
  const [pendingFile, setPendingFile] = useState<File | null>(null);
  const [inputKey, setInputKey] = useState(0);
  const [localError, setLocalError] = useState<string | null>(null);

  const attachments = useQuery({
    queryKey: ["change-attachments", change.id],
    queryFn: ({ signal }) => api.getChangeAttachments(change.id, signal),
  });
  const upload = useMutation({
    mutationFn: (file: File) => api.uploadChangeAttachment(change.id, file),
    onSuccess: () => {
      setPendingFile(null);
      setInputKey((k) => k + 1);
      void attachments.refetch();
      onDone("File terunggah.");
    },
    onError,
  });

  // Aturan peran untuk UX saja; backend yang menegakkan.
  const canUpload =
    user !== null &&
    change.status !== "REJECTED" &&
    change.status !== "CANCELLED" &&
    (user.role === "ManagerLead" ||
      user.role === "SystemAnalyst" ||
      user.id === change.requester_id ||
      user.id === change.implementer_id);

  function submit(e: FormEvent) {
    e.preventDefault();
    if (pendingFile === null) {
      setLocalError("Pilih file dulu (png/jpg/gif/webp/pdf/txt/zip, maks 10 MiB).");
      return;
    }
    setLocalError(null);
    upload.mutate(pendingFile);
  }

  async function download(aid: string, fileName: string) {
    try {
      const blob = await api.downloadChangeAttachment(change.id, aid);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = fileName;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      onError(err);
    }
  }

  return (
    <div className="rounded-lg border border-mist bg-paper p-6">
      <h2 className="font-semibold">Attachment</h2>
      {attachments.isPending && <div className="mt-2 h-10 animate-pulse rounded bg-lilac" />}
      {attachments.isError && (
        <p className="mt-2 text-sm text-red-700">
          Gagal memuat file.{" "}
          <button onClick={() => attachments.refetch()} className="text-iris hover:underline">
            Coba lagi
          </button>
        </p>
      )}
      <ul className="mt-2 space-y-2 text-sm">
        {(attachments.data?.data ?? []).map((a) => (
          <li key={a.id} className="flex items-center justify-between gap-2 rounded-lg bg-chalk p-3">
            <span className="min-w-0">
              <span className="block truncate font-medium">{a.file_name}</span>
              <span className="text-xs text-veil">
                {a.mime_type} · {formatBytes(a.size_bytes)} ·{" "}
                {(a.uploaded_by && userById.get(a.uploaded_by)?.name) || "?"}
              </span>
            </span>
            <button
              type="button"
              onClick={() => download(a.id, a.file_name)}
              className="shrink-0 rounded-full border border-mist px-3 py-1 text-xs font-semibold text-deep hover:bg-lilac"
            >
              Unduh
            </button>
          </li>
        ))}
        {attachments.data && attachments.data.data.length === 0 && (
          <li className="text-veil">Belum ada file.</li>
        )}
      </ul>
      {localError !== null && (
        <p className="mt-2 rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{localError}</p>
      )}
      {canUpload && (
        <form onSubmit={submit} className="mt-3 flex flex-wrap gap-2">
          <input
            key={inputKey}
            type="file"
            accept=".png,.jpg,.jpeg,.gif,.webp,.pdf,.txt,.zip"
            onChange={(e) => setPendingFile(e.target.files?.[0] ?? null)}
            className="min-w-52 flex-1 rounded border border-mist bg-paper px-3 py-2 text-sm focus:border-iris focus:outline-none"
            aria-label="Pilih file"
          />
          <button
            type="submit"
            disabled={upload.isPending}
            className="rounded-full bg-iris px-5 py-2 text-sm font-semibold text-white shadow-sm hover:brightness-95 disabled:opacity-50"
          >
            {upload.isPending ? "Mengunggah…" : "Unggah"}
          </button>
        </form>
      )}
    </div>
  );
}
