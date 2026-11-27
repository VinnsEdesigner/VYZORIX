import { Button, LoadingPlaceholder } from '@vyzorix/ui';
import { EmptySurface, StatusSurface } from './surface-state';
import { describeError } from '@/lib/error';

export function ActivityLoading() {
  return <LoadingPlaceholder text="Loading activity…" />;
}

export function ActivityError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  return (
    <StatusSurface
      icon="alert-triangle"
      tone="error"
      title="Could not load activity"
      detail={describeError(error)}
      actions={
        <Button variant="secondary" size="sm" icon="refresh-cw" onClick={onRetry}>
          Try again
        </Button>
      }
    />
  );
}

export function ActivityEmpty() {
  return (
    <EmptySurface
      illustration="activity"
      title="Nothing has happened yet"
      detail="Every credential, source, and sync action is recorded here with who caused it — a person, a service account, the scheduler, or a git push."
    />
  );
}

export function OverviewLoading() {
  return <LoadingPlaceholder text="Loading provisioning status…" />;
}

export function OverviewError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  return (
    <StatusSurface
      icon="alert-triangle"
      tone="error"
      title="Could not load the overview"
      detail={describeError(error)}
      actions={
        <Button variant="secondary" size="sm" icon="refresh-cw" onClick={onRetry}>
          Try again
        </Button>
      }
    />
  );
}

export function OverviewEmpty({ onAdd }: { onAdd?: () => void }) {
  return (
    <EmptySurface
      illustration="overview"
      title="Provisioning is not set up"
      detail="Install a git provider first. That authorises VYZORIX to read the repositories your device configuration lives in."
      actions={
        onAdd ? (
          <Button variant="primary" size="sm" icon="plus" onClick={onAdd}>
            Add a provider
          </Button>
        ) : undefined
      }
    />
  );
}
