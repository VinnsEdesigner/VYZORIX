import { useState, useCallback, useMemo } from "react";
import type {
  OffsetPaginationParams,
  OffsetPagination,
} from "@vyzorix/api-client";

export interface PaginationState {
  page: number;
  limit: number;
}

export interface UsePaginationOptions {
  initialPage?: number;
  initialLimit?: number;
  maxLimit?: number;
  onPageChange?: (page: number) => void;
  onLimitChange?: (limit: number) => void;
}

export function usePagination(options: UsePaginationOptions = {}) {
  const {
    initialPage = 1,
    initialLimit = 20,
    maxLimit = 100,
    onPageChange,
    onLimitChange,
  } = options;

  const [state, setState] = useState<PaginationState>({
    page: initialPage,
    limit: Math.min(initialLimit, maxLimit),
  });

  const setPage = useCallback(
    (page: number) => {
      const newPage = Math.max(1, page);
      setState((prev) => ({ ...prev, page: newPage }));
      onPageChange?.(newPage);
    },
    [onPageChange]
  );

  const setLimit = useCallback(
    (limit: number) => {
      const newLimit = Math.min(Math.max(1, limit), maxLimit);
      setState((prev) => ({ ...prev, limit: newLimit, page: 1 }));
      onLimitChange?.(newLimit);
    },
    [maxLimit, onLimitChange]
  );

  const nextPage = useCallback(() => {
    setPage(state.page + 1);
  }, [state.page, setPage]);

  const prevPage = useCallback(() => {
    setPage(state.page - 1);
  }, [state.page, setPage]);

  const resetPagination = useCallback(() => {
    setState({ page: initialPage, limit: Math.min(initialLimit, maxLimit) });
  }, [initialPage, initialLimit, maxLimit]);

  const params = useMemo<OffsetPaginationParams>(
    () => ({
      page: state.page,
      limit: state.limit,
    }),
    [state.page, state.limit]
  );

  return {

    page: state.page,
    limit: state.limit,
    params,

    setPage,
    setLimit,

    nextPage,
    prevPage,
    resetPagination,

    canGoNext: (pagination?: OffsetPagination) =>
      pagination ? pagination.page < pagination.totalPages : true,
    canGoPrev: () => state.page > 1,
  };
}

export function usePaginationInfo(
  pagination: OffsetPagination | undefined,
  page: number,
  limit: number
) {
  return useMemo(() => {
    if (!pagination) {
      return {
        showingFrom: 0,
        showingTo: 0,
        total: 0,
        hasPagination: false,
      };
    }

    const showingFrom = (pagination.page - 1) * pagination.limit + 1;
    const showingTo = Math.min(
      pagination.page * pagination.limit,
      pagination.total
    );

    return {
      showingFrom,
      showingTo,
      total: pagination.total,
      hasPagination: pagination.totalPages > 1,
    };
  }, [pagination, page, limit]);
}

export function parsePaginationFromURL(
  searchParams: URLSearchParams
): OffsetPaginationParams {
  const page = searchParams.get("page");
  const limit = searchParams.get("limit");

  return {
    page: page ? parseInt(page, 10) : 1,
    limit: limit ? parseInt(limit, 10) : 20,
  };
}

export function updatePaginationURL(
  page: number,
  limit: number
): URLSearchParams {
  const params = new URLSearchParams();
  if (page > 1) params.set("page", String(page));
  if (limit !== 20) params.set("limit", String(limit));
  return params;
}
