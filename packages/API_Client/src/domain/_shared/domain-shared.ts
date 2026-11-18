export type DeviceStatus = "online" | "offline" | "deregistered";

export interface MetricThreshold {
  warning: number;
  critical: number;
}
