import { Button, LoadingPlaceholder } from '@vyzorix/ui';
import { EmptySurface, StatusSurface } from './surface-state';
import { describeError } from '@/lib/error';

export function ResourceListLoading() {
  return <LoadingPlaceholder text="Loading resources…" />;
}

export function ResourceListError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  return (
    <StatusSurface
      icon="alert-triangle"
      tone="error"
      title="Could not load resources"
      detail={describeError(error)}
      actions={
        <Button variant="secondary" size="sm" icon="refresh-cw" onClick={onRetry}>
          Try again
        </Button>
      }
    />
  );
}

export function ResourceListEmpty() {
  return (
    <EmptySurface
      illustration="resources"
      title="This source owns nothing yet"
      detail="Run a sync and every manifest the repository declares becomes a resource here."
    />
  );
}

export function ResourceListNoMatches({ onClear }: { onClear: () => void }) {

  return (
    <EmptySurface
      illustration="resources"
      title="No resources match that filter"
      detail="This source has resources, but none of that kind. Clear the filter to see them all."
      actions={
        <Button variant="secondary" size="sm" onClick={onClear}>
          Clear filter
        </Button>
      }
    />
  );
}
