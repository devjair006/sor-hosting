import { useQuery } from "@tanstack/react-query";
import { api } from "../services/api";
import type { Job } from "../services/types";
export function useJob(id: string | null) {
  return useQuery({
    queryKey: ["job", id],
    queryFn: ({ signal }) => api<{ job: Job }>("/jobs/" + id, { signal }),
    enabled: !!id,
    refetchInterval: (q) =>
      q.state.data &&
      ["succeeded", "failed", "interrupted"].includes(q.state.data.job.state)
        ? false
        : 2000,
  });
}
