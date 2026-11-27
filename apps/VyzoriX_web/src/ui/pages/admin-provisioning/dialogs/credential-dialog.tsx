import { useEffect, useState } from 'react';
import { css } from '@emotion/css';
import { Button, Combobox, Field, Input, Modal, Switch, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { describeError } from '@/lib/error';
import {
  PROVIDER_TYPES,
  providerLabel,
  useCreateProvisioningConnection,
  useUpdateProvisioningConnection,
  validateConnectionDraft,
  type ProvisioningConnection,
} from '@/hooks/provisioning';
import { notifyError, notifySuccess } from '@/stores/toast-store';

interface CredentialDialogProps {
  isOpen: boolean;
  onDismiss: () => void;

  connection?: ProvisioningConnection;
}

const DEFAULT_HOST: Record<string, string> = {
  github: 'github.com',
  gitlab: 'gitlab.com',
  generic: '',
};

export function CredentialDialog({ isOpen, onDismiss, connection }: CredentialDialogProps) {
  const styles = useStyles2(getStyles);
  const create = useCreateProvisioningConnection();
  const update = useUpdateProvisioningConnection();

  const editing = Boolean(connection);
  const [name, setName] = useState('');
  const [type, setType] = useState<string>('github');
  const [host, setHost] = useState(DEFAULT_HOST.github ?? '');
  const [token, setToken] = useState('');
  const [writeEnabled, setWriteEnabled] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [serverError, setServerError] = useState('');

  useEffect(() => {
    if (!isOpen) return;
    setName(connection?.name ?? '');
    setType(connection?.type ?? 'github');
    setHost(connection?.host ?? DEFAULT_HOST.github ?? '');
    setToken('');
    setWriteEnabled(connection?.writeEnabled ?? false);
    setErrors({});
    setServerError('');
  }, [isOpen, connection]);

  const onProviderChange = (next: string) => {
    setType(next);

    if (!host || host === DEFAULT_HOST[type]) {
      setHost(DEFAULT_HOST[next] ?? '');
    }
  };

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setServerError('');

    const draft = { name, type, host, token };
    const found = validateConnectionDraft(editing ? { ...draft, token: token || 'unchanged' } : draft);
    if (Object.keys(found).length > 0) {
      setErrors(found);
      return;
    }
    setErrors({});

    const data = {
      name: name.trim(),
      type,
      host: host.trim(),
      ...(token ? { token } : {}),
      write_enabled: writeEnabled,
    };

    try {
      if (editing && connection) {
        await update.mutateAsync({ id: connection.id, data });
        notifySuccess('Credential updated', token ? 'The token was replaced and re-checked.' : undefined);
      } else {
        const created = await create.mutateAsync({ data });

        if (created.status === 'error') {
          notifyError('Saved, but the check failed', created.last_error || 'Check the token and host.');
        } else {
          notifySuccess('Credential added', `Reached ${created.host || host}.`);
        }
      }
      onDismiss();
    } catch (error) {
      setServerError(describeError(error));
    }
  };

  const busy = create.isPending || update.isPending;

  return (
    <Modal title={editing ? 'Edit credential' : 'Add credential'} isOpen={isOpen} onDismiss={onDismiss}>
      <form onSubmit={submit} className={styles.form}>
        <Field label="Name" required error={errors.name} description="What you will recognise it by.">
          <Input
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="GitHub — platform team"
            autoFocus
          />
        </Field>

        <Field label="Provider" required error={errors.type}>
          <Combobox
            options={PROVIDER_TYPES.map((value) => ({ value, label: providerLabel(value) }))}
            value={type}
            onChange={(next) => onProviderChange(next ?? 'github')}
            aria-label="Provider"
          />
        </Field>

        <Field
          label="Host"
          error={errors.host}
          description="Where the provider lives. Leave the default unless you self-host."
        >
          <Input value={host} onChange={(event) => setHost(event.target.value)} placeholder="github.com" />
        </Field>

        <Field
          label="Token"
          required={!editing}
          error={errors.token}
          description={
            editing
              ? 'Leave empty to keep the stored token. It can never be read back.'
              : 'Stored encrypted. Never shown again after saving.'
          }
        >
          <Input
            type="password"
            value={token}
            onChange={(event) => setToken(event.target.value)}
            placeholder={editing ? 'Unchanged' : 'ghp_…'}
            autoComplete="off"
          />
        </Field>

        <label className={styles.switchRow}>
          <Switch checked={writeEnabled} onChange={(event) => setWriteEnabled(event.target.checked)} />
          <span>
            <span className={styles.switchLabel}>Write access</span>
            <span className={styles.switchHint}>
              Lets VYZORIX commit edits back. Off by default, because syncing only reads.
            </span>
          </span>
        </label>

        {serverError ? <p className={styles.serverError}>{serverError}</p> : null}

        <div className={styles.actions}>
          <Button type="button" variant="secondary" onClick={onDismiss}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" loading={busy}>
            {editing ? 'Save' : 'Add credential'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  form: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  switchRow: css({ display: 'flex', alignItems: 'flex-start', gap: theme.spacing(1.5), cursor: 'pointer' }),
  switchLabel: css({
    display: 'block',
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  switchHint: css({ display: 'block', fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  serverError: css({
    margin: 0,
    padding: theme.spacing(1.5),
    borderRadius: theme.shape.radius.default,
    background: theme.colors.error.subtleBackground,
    border: `1px solid ${theme.colors.error.subtleBorder}`,
    color: theme.colors.error.text,
    fontSize: theme.typography.sm.fontSize,
  }),
  actions: css({ display: 'flex', justifyContent: 'flex-end', gap: theme.spacing(1), marginTop: theme.spacing(1) }),
});
