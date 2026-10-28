import { useEffect } from "react";
import { useJob } from "../hooks/useJob";
import { queryClient } from "../lib/queryClient";
import { Status, ErrorState } from "./State";
export function JobProgress({ id }: { id: string | null }) {
  const q = useJob(id);
  const job = q.data?.job;
  useEffect(() => {
    if (job?.state === "succeeded")
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
        <p className="field-error">
          La operación necesita revisión. Código: {job.error_code}
        </p>
      )}
      <small>
        Referencia: <code>{id.slice(0, 12)}</code>
      </small>
    </div>
  );
}
