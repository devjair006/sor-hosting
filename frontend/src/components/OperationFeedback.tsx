import { JobProgress } from "./JobProgress";
export function OperationFeedback({
  jobId,
  error,
}: {
  jobId: string | null;
  error: string;
}) {
  return (
    <>
      {error && (
        <p role="alert" className="field-error">
          {error}
        </p>
      )}
      <JobProgress id={jobId} />
    </>
  );
}
