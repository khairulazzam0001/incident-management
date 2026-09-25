import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";

export function useHealth() {
  return useQuery({
    queryKey: ["health"],
    queryFn: ({ signal }) => api.getHealth(signal),
    retry: 1,
    staleTime: 30_000,
  });
}
