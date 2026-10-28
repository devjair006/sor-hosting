import { QueryClient } from "@tanstack/react-query";
import { ApiError, StaleSessionError } from "../services/api";
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 15_000,
      retry: (n, e) =>
        !(e instanceof StaleSessionError) &&
        !(e instanceof ApiError && [401, 403, 404, 422].includes(e.status)) &&
        n < 1,
      refetchOnWindowFocus: false,
    },
  },
});
