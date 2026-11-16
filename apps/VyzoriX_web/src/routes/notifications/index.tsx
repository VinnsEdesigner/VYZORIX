import { useState } from 'react';
import { useStyles2 } from '@vyzorix/ui';
import { listPageStyles } from '@/ui/styles/list-page';
import { createFileRoute } from '@tanstack/react-router';
import {
  CONTACT_POINT_CHANNELS,
  type ContactPoint,
  type ContactPointChannel,
  type ContactPointRequest,
} from '@vyzorix/api-client';
import {
  useContactPoints,
  useCreateContactPoint,
  useDeleteContactPoint,
  useTestContactPoint,
} from '@/hooks/notifications/use-contact-points';

const CHANNEL_LABELS: Record<ContactPointChannel, string> = {
  email: 'Email',
  webhook: 'Webhook',
  slack: 'Slack',
};

const CHANNEL_COLORS: Record<string, string> = {
  email: '#3b82f6',
  webhook: '#8b5cf6',
  slack: '#059669',
};

function ContactPointForm({
  onSubmit,
  disabled,
}: {
  onSubmit: (req: ContactPointRequest) => void;
  disabled: boolean;
}) {
  const styles = useStyles2(listPageStyles);
  const [name, setName] = useState('');
  const [channel, setChannel] = useState<ContactPointChannel>('webhook');
  const [secret, setSecret] = useState('');
  const [config, setConfig] = useState('');
  const [enabled, setEnabled] = useState(true);

  const configFromText = (text: string): Record<string, string> => {
    const obj: Record<string, string> = {};
    text.split('\n').forEach((line) => {
      const [key, ...rest] = line.split('=');
      if (key && rest.length > 0) obj[key.trim()] = rest.join('=').trim();
    });
    return obj;
  };

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit({
          name,
          channel,
          secret: secret || undefined,
          config: configFromText(config),
          enabled,
        });
      }}
    >
      <div className={styles.grid}>
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Contact point name"
          required
          className={styles.input}
        />
        <select
          value={channel}
          onChange={(e) => setChannel(e.target.value as ContactPointChannel)}
          className={styles.input}
        >
          {CONTACT_POINT_CHANNELS.map((c) => (
            <option key={c} value={c}>
              {CHANNEL_LABELS[c]}
            </option>
          ))}
        </select>
        <input
          value={secret}
          onChange={(e) => setSecret(e.target.value)}
          type="password"
          placeholder="Webhook secret (optional)"
          className={styles.input}
        />
        <label className={styles.labelRow}>
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          <span>Enabled</span>
        </label>
      </div>
      <textarea
        value={config}
        onChange={(e) => setConfig(e.target.value)}
        placeholder="Config (one per line: to=ops@example.com, url=https://hooks.slack.com/...)"
        rows={4}
        className={styles.textarea}
      />
      <button
        type="submit"
        disabled={disabled}
        className={styles.submit}
      >
        Create contact point
      </button>
    </form>
  );
}

function ContactPointCard({ point }: { point: ContactPoint }) {
  const styles = useStyles2(listPageStyles);
  const deletePoint = useDeleteContactPoint();
  const testPoint = useTestContactPoint();

  return (
    <div className={styles.card}>
      <div className={styles.row}>
        <div>
          <h3 className={styles.h3}>{point.name}</h3>
          <p className={styles.meta}>
            {CHANNEL_LABELS[point.channel as keyof typeof CHANNEL_LABELS] ?? point.channel}
            {point.secret && ' (signed)'}
          </p>
        </div>
        <div className={styles.rowActions}>
          <span
            className={styles.badge}
            style={{
              backgroundColor: `${CHANNEL_COLORS[point.channel ?? ''] ?? '#6b7280'}20`,
              color: CHANNEL_COLORS[point.channel ?? ''] ?? '#6b7280',
            }}
          >
            {point.channel}
          </span>
          <button
            onClick={() => point.id && testPoint.mutate({ id: point.id })}
            className={styles.smallBtn}
            title="Send test notification"
          >
            Test
          </button>
          <button
            onClick={() => point.id && deletePoint.mutate({ id: point.id })}
            className={styles.smallBtnDanger}
            title="Delete"
          >
            Delete
          </button>
        </div>
      </div>
      <div className={styles.metaSm}>
        {Object.entries(point.config ?? {}).map(([k, v]) => (
          <div key={k}>
            <strong>{k}:</strong> {v.length > 40 ? v.slice(0, 40) + '…' : v}
          </div>
        ))}
      </div>
    </div>
  );
}

function NotificationsPage() {
  const styles = useStyles2(listPageStyles);
  const points = useContactPoints();
  const createPoint = useCreateContactPoint();

  return (
    <div className={styles.page}>
      <h1 className={styles.h1}>Contact Points</h1>

      <div className={styles.panel}>
        <h2 className={styles.h2}>New contact point</h2>
        <ContactPointForm onSubmit={(req) => createPoint.mutate({ data: req })} disabled={createPoint.isPending} />
      </div>

      {points.isLoading && <p>Loading contact points…</p>}
      {points.isError && <p className={styles.error}>Failed to load contact points.</p>}

      <div className={styles.stack}>
        {(points.data?.contact_points ?? []).map((point) => (
          <ContactPointCard key={point.id} point={point} />
        ))}
        {(points.data?.contact_points ?? []).length === 0 && (
          <p className={styles.meta}>No contact points yet. Create one above.</p>
        )}
      </div>
    </div>
  );
}

export const Route = createFileRoute('/notifications/')({
  component: NotificationsPage,
});
