import { useState } from 'react';
import { css } from '@emotion/css';
import { useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { createFileRoute } from '@tanstack/react-router';
import {
  ALERT_METRICS,
  ALERT_CONDITIONS,
  type AlertRule,
  type AlertRuleRequest,
} from '@vyzorix/api-client';
import {
  useAlertRules,
  useCreateAlertRule,
  useDeleteAlertRule,
  useEvaluateAlertRule,
} from '@/hooks/alerts/use-alerts';
import { Page } from '@/ui/components/layout';

const getStyles = (theme: VyzorixTheme) => ({
  h2: css({ marginBottom: theme.spacing(2), fontWeight: theme.typography.fontWeightMedium }),
  h3: css({ fontWeight: theme.typography.fontWeightMedium, margin: 0 }),
  panel: css({ marginBottom: theme.spacing(4), border: `1px solid ${theme.colors.border.medium}`, borderRadius: theme.shape.radius.default, padding: theme.spacing(2) }),
  grid: css({ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: theme.spacing(1.5) }),
  input: css({ borderRadius: theme.shape.radius.sm, border: `1px solid ${theme.colors.border.medium}`, padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`, background: theme.colors.background.primary, color: theme.colors.text.primary }),
  submit: css({ marginTop: theme.spacing(2), borderRadius: theme.shape.radius.sm, background: theme.colors.primary.main, color: theme.colors.primary.contrastText, border: 'none', padding: `${theme.spacing(1)} ${theme.spacing(2)}`, cursor: 'pointer', '&:disabled': { opacity: 0.5 } }),
  card: css({ borderRadius: theme.shape.radius.default, border: `1px solid ${theme.colors.border.medium}`, padding: theme.spacing(2) }),
  row: css({ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }),
  rowActions: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1) }),
  meta: css({ fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary, margin: 0 }),
  badge: css({ borderRadius: theme.shape.radius.pill, padding: `2px ${theme.spacing(1)}`, fontSize: theme.typography.sm.fontSize, fontWeight: theme.typography.fontWeightMedium }),
  smallBtn: css({ borderRadius: theme.shape.radius.sm, border: `1px solid ${theme.colors.border.medium}`, padding: `2px ${theme.spacing(1)}`, fontSize: theme.typography.sm.fontSize, background: 'transparent', color: theme.colors.text.primary, cursor: 'pointer' }),
  smallBtnDanger: css({ borderRadius: theme.shape.radius.sm, border: `1px solid ${theme.colors.border.medium}`, padding: `2px ${theme.spacing(1)}`, fontSize: theme.typography.sm.fontSize, background: 'transparent', color: theme.colors.error.text, cursor: 'pointer' }),
  stack: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1.5) }),
  error: css({ color: theme.colors.error.text }),
  mt2: css({ marginTop: theme.spacing(1) }),
  mt1: css({ marginTop: theme.spacing(0.5) }),
  strong: css({ fontWeight: theme.typography.fontWeightMedium }),
});

const METRIC_LABELS: Record<(typeof ALERT_METRICS)[number], string> = {
  device_offline_count: 'Offline devices (count)',
  device_offline_percent: 'Offline devices (%)',
  command_failure_rate: 'CommandResponse failure rate (%)',
};

const STATE_COLORS: Record<string, string> = {
  firing: '#ef4444',
  pending: '#f59e0b',
  inactive: '#22c55e',
};

function RuleForm({
  onSubmit,
  disabled,
}: {
  onSubmit: (req: AlertRuleRequest) => void;
  disabled: boolean;
}) {
  const [name, setName] = useState('');
  const [metric, setMetric] = useState<(typeof ALERT_METRICS)[number]>('device_offline_count');
  const [condition, setCondition] = useState<(typeof ALERT_CONDITIONS)[number]>('gt');
  const [threshold, setThreshold] = useState('1');
  const [forSeconds, setForSeconds] = useState('0');
  const [webhookUrl, setWebhookUrl] = useState('');
  const styles = useStyles2(getStyles);

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit({
          name,
          metric,
          condition,
          threshold: Number(threshold),
          for_seconds: Number(forSeconds),
          webhook_url: webhookUrl || undefined,
          enabled: true,
        });
      }}
    >
      <div className={styles.grid}>
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Rule name"
          required
          className={styles.input}
        />
        <select value={metric} onChange={(e) => setMetric(e.target.value as (typeof ALERT_METRICS)[number])} className={styles.input}>
          {ALERT_METRICS.map((m) => (
            <option key={m} value={m}>{METRIC_LABELS[m]}</option>
          ))}
        </select>
        <select value={condition} onChange={(e) => setCondition(e.target.value as (typeof ALERT_CONDITIONS)[number])} className={styles.input}>
          {ALERT_CONDITIONS.map((c) => (
            <option key={c} value={c}>{c}</option>
          ))}
        </select>
        <input
          value={threshold}
          onChange={(e) => setThreshold(e.target.value)}
          type="number"
          step="any"
          placeholder="Threshold"
          required
          className={styles.input}
        />
        <input
          value={forSeconds}
          onChange={(e) => setForSeconds(e.target.value)}
          type="number"
          min="0"
          placeholder="Pending (seconds)"
          className={styles.input}
        />
        <input
          value={webhookUrl}
          onChange={(e) => setWebhookUrl(e.target.value)}
          type="url"
          placeholder="Webhook URL (optional)"
          className={styles.input}
        />
      </div>
      <button
        type="submit"
        disabled={disabled}
        className={styles.submit}
      >
        Create rule
      </button>
    </form>
  );
}

function RuleCard({ rule }: { rule: AlertRule }) {
  const styles = useStyles2(getStyles);
  const deleteRule = useDeleteAlertRule();
  const evaluateRule = useEvaluateAlertRule();

  return (
    <div className={styles.card}>
      <div className={styles.row}>
        <div>
          <h3 className={styles.h3}>{rule.name}</h3>
          <p className={styles.meta}>
            {METRIC_LABELS[(rule.metric ?? "device_offline_count") as keyof typeof METRIC_LABELS]} {rule.condition} {rule.threshold}
            {(rule.for_seconds ?? 0) > 0 ? ` for ${rule.for_seconds}s` : ''}
          </p>
        </div>
        <div className={styles.rowActions}>
          <span
            className={styles.badge}
            style={{ backgroundColor: `${STATE_COLORS[(rule.instances?.[0]?.state ?? 'inactive')] ?? '#6b7280'}20`, color: STATE_COLORS[(rule.instances?.[0]?.state ?? 'inactive')] ?? '#6b7280' }}
          >
            {rule.instances?.[0]?.state ?? 'inactive'}
          </span>
          <button
            onClick={() => evaluateRule.mutate({ id: rule.id! })}
            className={styles.smallBtn}
            title="Evaluate now"
          >
            Evaluate
          </button>
          <button
            onClick={() => deleteRule.mutate({ id: rule.id! })}
            className={styles.smallBtnDanger}
            title="Delete rule"
          >
            Delete
          </button>
        </div>
      </div>
      {rule.instances?.[0]?.state !== 'inactive' && (
        <p className={styles.mt2}>Current value: <strong className={styles.strong}>{rule.instances?.[0]?.value ?? '—'}</strong></p>
      )}
      {rule.webhook_url && (
        <p className={styles.mt1}>Webhook: {rule.webhook_url}</p>
      )}
    </div>
  );
}

function AlertsPage() {
  const styles = useStyles2(getStyles);
  const rules = useAlertRules();
  const createRule = useCreateAlertRule();

  return (
    <Page title="Alerts" subTitle="Alert rules and their current state">
      <div className={styles.panel}>
        <h2 className={styles.h2}>New rule</h2>
        <RuleForm onSubmit={(req) => createRule.mutate({ data: req })} disabled={createRule.isPending} />
      </div>

      {rules.isLoading && <p>Loading rules…</p>}
      {rules.isError && <p className={styles.error}>Failed to load rules.</p>}

      <div className={styles.stack}>
        {(rules.data?.rules ?? []).map((rule) => (
          <RuleCard key={rule.id} rule={rule} />
        ))}
        {(rules.data?.rules ?? []).length === 0 && (
          <p className={styles.meta}>No alert rules yet. Create one above.</p>
        )}
      </div>
    </Page>
  );
}

export const Route = createFileRoute('/alerts/')({
  component: AlertsPage,
});
