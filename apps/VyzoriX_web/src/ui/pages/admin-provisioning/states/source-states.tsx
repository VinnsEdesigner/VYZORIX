import { Button, LoadingPlaceholder } from '@vyzorix/ui';
import { EmptySurface, StatusSurface } from './surface-state';
import { describeError } from '@/lib/error';

export function SourceListLoading() {
  return <LoadingPlaceholder text="Loading sources…" />;
}

export function SourceListError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  return (
    <StatusSurface
      icon="alert-triangle"
      tone="error"
      title="Could not load sources"
      detail={describeError(error)}
      actions={
        <Button variant="secondary" size="sm" icon="refresh-cw" onClick={onRetry}>
          Try again
        </Button>
      }
    />
  );
}

export function SourceListEmpty({ onAdd }: { onAdd?: () => void }) {
  return (
    <EmptySurface
      illustration="sources"
      title="No sources yet"
      detail="A source is a git repository whose manifests describe your devices. Add one to stop configuring devices by hand."
      actions={
        onAdd ? (
          <Button variant="primary" size="sm" icon="plus" onClick={onAdd}>
            Add a source
          </Button>
        ) : undefined
      }
    />
  );
}

export function SourceListNoMatches({ onClear }: { onClear: () => void }) {
  return (
    <StatusSurface
      icon="search"
      tone="progress"
      title="No sources match that filter"
      detail="Clear the filter to see every source."
      actions={
        <Button variant="secondary" size="sm" onClick={onClear}>
          Clear filter
        </Button>
      }
    />
  );
}
