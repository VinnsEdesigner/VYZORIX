export type {
  AlertRule,
  AlertRuleListResult,
  AlertRuleRequest,
  AlertInstance,
  AlertHistoryEvent,
  AlertHistoryResult,
  AlertEvaluateResult,
} from '../../generated/vyzorixUpdateServerAPI.schemas';

export const ALERT_CONDITIONS = ['gt', 'gte', 'lt', 'lte'] as const;
export type AlertCondition = (typeof ALERT_CONDITIONS)[number];

export const ALERT_METRICS = ['device_offline_count', 'device_offline_percent', 'command_failure_rate'] as const;
export type AlertMetric = (typeof ALERT_METRICS)[number];

export const ALERT_STATES = ['inactive', 'pending', 'firing', 'no_data', 'error'] as const;
export type AlertState = (typeof ALERT_STATES)[number];
