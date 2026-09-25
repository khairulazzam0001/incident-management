import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";

export function useMeta() {
  return useQuery({
    queryKey: ["meta"],
    queryFn: ({ signal }) => api.getMeta(signal),
    staleTime: 5 * 60_000,
  });
}

export function useUsers() {
  return useQuery({
    queryKey: ["users"],
    queryFn: ({ signal }) => api.getUsers(signal),
    staleTime: 60_000,
  });
}
