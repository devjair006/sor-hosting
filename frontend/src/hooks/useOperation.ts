import { useRef, useState } from "react";
import { submit, key } from "../services/api";
import type { Job } from "../services/types";
export function useOperation() {
  const [busy, setBusy] = useState(false);
  const [jobId, setJobId] = useState<string | null>(null);
  const [error, setError] = useState("");
  const retry = useRef<{ fingerprint: string; key: string } | null>(null);
  const pending = useRef(false);
  async function run(path: string, body: unknown) {
    if (pending.current) return;
    pending.current = true;
    setBusy(true);
    setError("");
    const fingerprint = path + JSON.stringify(body);
    if (retry.current?.fingerprint !== fingerprint)
      retry.current = { fingerprint, key: key() };
    try {
      const result = await submit<{ job: Job }>(path, body, retry.current.key);
      setJobId(result.job.id);
      retry.current = null;
      return result.job;
    } catch (e) {
      setError(
        e instanceof Error ? e.message : "No se pudo registrar la operación.",
      );
      return undefined;
    } finally {
      setBusy(false);
      pending.current = false;
    }
  }
  return { busy, jobId, error, run };
}
