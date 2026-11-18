import { useState, useCallback, useMemo } from "react";

export const TIME_RANGES = {
  "1h": { label: "1h", value: "1h", hours: 1 },
  "6h": { label: "6h", value: "6h", hours: 6 },
  "24h": { label: "24h", value: "24h", hours: 24 },
  "7d": { label: "7d", value: "7d", hours: 168 },
} as const;

export type TimeRangeKey = keyof typeof TIME_RANGES;
export type TimeRangeValue = (typeof TIME_RANGES)[TimeRangeKey];

export interface TimeRange {
  range: TimeRangeKey;
  startTime: Date;
  endTime: Date;
}

export interface UseTimeRangeOptions {
  initialRange?: TimeRangeKey;
  onRangeChange?: (range: TimeRangeKey) => void;
}

export function useTimeRange(options: UseTimeRangeOptions = {}) {
  const { initialRange = "6h", onRangeChange } = options;

  const calculateRange = useCallback((range: TimeRangeKey): TimeRange => {
    const end = new Date();
    const start = new Date(end.getTime() - TIME_RANGES[range].hours * 60 * 60 * 1000);
    return { range, startTime: start, endTime: end };
  }, []);

  const [timeRange, setTimeRange] = useState<TimeRange>(() =>
    calculateRange(initialRange)
  );

  const setRange = useCallback(
    (range: TimeRangeKey) => {
      setTimeRange(calculateRange(range));
      onRangeChange?.(range);
    },
    [calculateRange, onRangeChange]
  );

  const refresh = useCallback(() => {
    setTimeRange(calculateRange(timeRange.range));
  }, [timeRange.range, calculateRange]);

  const getTimeParams = useCallback(
    (range?: TimeRangeKey) => {
      const r = range ?? timeRange.range;
      const end = new Date();
      const start = new Date(
        end.getTime() - TIME_RANGES[r].hours * 60 * 60 * 1000
      );

      return {
        startTime: start.toISOString(),
        endTime: end.toISOString(),
        range: r,
      };
    },
    [timeRange.range]
  );

  const getTimestamps = useCallback(
    (range?: TimeRangeKey) => {
      const r = range ?? timeRange.range;
      const end = Date.now();
      const start = end - TIME_RANGES[r].hours * 60 * 60 * 1000;
      return { start, end };
    },
    [timeRange.range]
  );

  return {

    range: timeRange.range,
    startTime: timeRange.startTime,
    endTime: timeRange.endTime,

    setRange,

    refresh,
    getTimeParams,
    getTimestamps,

    ranges: TIME_RANGES,
  };
}

export function calculateResolution(
  range: TimeRangeKey
): { resolution: string; maxPoints: number } {
  switch (range) {
    case "1h":
      return { resolution: "1m", maxPoints: 60 };
    case "6h":
      return { resolution: "5m", maxPoints: 72 };
    case "24h":
      return { resolution: "15m", maxPoints: 96 };
    case "7d":
      return { resolution: "1h", maxPoints: 168 };
    default:
      return { resolution: "5m", maxPoints: 72 };
  }
}

export function useResolution(range: TimeRangeKey) {
  return useMemo(() => calculateResolution(range), [range]);
}

export function parseTimeRangeFromURL(searchParams: URLSearchParams): TimeRangeKey {
  const range = searchParams.get("range");
  if (range && range in TIME_RANGES) {
    return range as TimeRangeKey;
  }
  return "6h";
}

export function updateTimeRangeURL(range: TimeRangeKey): URLSearchParams {
  const params = new URLSearchParams();
  if (range !== "6h") {
    params.set("range", range);
  }
  return params;
}

export function formatTimeRange(range: TimeRangeKey): string {
  return TIME_RANGES[range].label;
}

export function formatTimestamp(date: Date): string {
  return date.toISOString();
}
