import type {
  TelemetryEntry,
  TelemetryHistoryQueryResult,
  TelemetryHistoryEntry,
  TelemetryStatsResult,
  MetricAggregateResult,
} from '../../generated/vyzorixUpdateServerAPI.schemas';
import type { MetricThreshold } from '../_shared';

export type {
  TelemetryEntry,
  TelemetryHistoryQueryResult,
  TelemetryHistoryEntry,
  TelemetryStatsResult,
  MetricAggregateResult,
};
export type { MetricThreshold };

export interface TelemetryFrame {
  timestamp: Date;
  riskScore: number;
  thermalTemp: number;
  bufferLevel: number;
  latencyMs?: number;
}

export function isRiskWarning(value: number, threshold: MetricThreshold): boolean {
  return value >= threshold.warning && value < threshold.critical;
}

export function isRiskCritical(value: number, threshold: MetricThreshold): boolean {
  return value >= threshold.critical;
}

export function getRiskStatus(value: number, threshold: MetricThreshold): 'normal' | 'warning' | 'critical' {
  if (isRiskCritical(value, threshold)) return 'critical';
  if (isRiskWarning(value, threshold)) return 'warning';
  return 'normal';
}
