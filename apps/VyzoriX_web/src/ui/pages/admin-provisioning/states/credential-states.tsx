import { Button, LoadingPlaceholder } from '@vyzorix/ui';
import { EmptySurface, StatusSurface } from './surface-state';
import { describeError } from '@/lib/error';

export function CredentialListLoading() {
  return <LoadingPlaceholder text="Loading credentials…" />;
}

export function CredentialListError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  return (
    <StatusSurface
      icon="alert-triangle"
      tone="error"
      title="Could not load credentials"
      detail={describeError(error)}
      actions={
        <Button variant="secondary" size="sm" icon="refresh-cw" onClick={onRetry}>
          Try again
        </Button>
      }
    />
  );
}

export function CredentialListEmpty({ onAdd }: { onAdd?: () => void }) {
  return (
    <EmptySurface
      illustration="credentials"
      title="No credentials yet"
      detail="A credential is the git token VYZORIX reads your sources with. It is stored encrypted and never shown again after you save it."
      actions={
        onAdd ? (
          <Button variant="primary" size="sm" icon="plus" onClick={onAdd}>
            Add a credential
          </Button>
        ) : undefined
      }
    />
  );
}
