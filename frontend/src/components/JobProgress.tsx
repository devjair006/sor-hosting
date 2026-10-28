import { useEffect } from "react";
import { useJob } from "../hooks/useJob";
import { queryClient } from "../lib/queryClient";
import { Status, ErrorState } from "./State";
export function JobProgress({ id }: { id: string | null }) {
  const q = useJob(id);
  const job = q.data?.job;
  useEffect(() => {
    if (job && ["succeeded", "failed", "interrupted"].includes(job.state))
      void queryClient.invalidateQueries({
        predicate: (q) =>
          q.queryKey[0] !== "job" && q.queryKey[0] !== "session",
      });
  }, [job?.state]);
  if (!id) return null;
  if (q.error) return <ErrorState error={q.error} retry={q.refetch} />;
  return (
    <div className="job-progress" role="status" aria-live="polite">
      <Status value={job?.state ?? "queued"} />
      <p>{job?.phase ?? "Registrando operación…"}</p>
      {job?.error_code && (
        <p className={job.state === "running" ? "muted" : "field-error"}>
          {job.state === "running"
            ? "Recuperación automática en curso. El sitio permanece bloqueado."
            : "La operación necesita revisión. Código: " + job.error_code}
        </p>
      )}
      <small>
        Referencia: <code>{id.slice(0, 12)}</code>
      </small>
    </div>
  );
}
