import { css } from '@emotion/css';
import { useEffect } from 'react';
import { Alert, Portal, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { TOAST_TIMEOUT, useToastStore, type Toast } from '@/stores/toast-store';

const getStyles = (theme: VyzorixTheme) => ({
  container: css({
    position: 'fixed',
    top: theme.spacing(8),
    right: theme.spacing(1),
    zIndex: theme.zIndex.portal,
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1),
    minWidth: 320,
    maxWidth: 520,
    pointerEvents: 'auto',
  }),

  visuallyHidden: css({
    position: 'absolute',
    width: 1,
    height: 1,
    padding: 0,
    margin: -1,
    overflow: 'hidden',
    clip: 'rect(0, 0, 0, 0)',
    whiteSpace: 'nowrap',
    border: 0,
  }),
});

function ToastItem({ toast }: { toast: Toast }) {
  const remove = useToastStore((s) => s.remove);

  useEffect(() => {
    const id = window.setTimeout(() => remove(toast.id), TOAST_TIMEOUT[toast.severity]);
    return () => window.clearTimeout(id);
  }, [toast.id, toast.severity, remove]);

  return (
    <Alert title={toast.title} severity={toast.severity} onRemove={() => remove(toast.id)}>
      {toast.text ? <span>{toast.text}</span> : null}
    </Alert>
  );
}

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const toasts = useToastStore((s) => s.toasts);
  const styles = useStyles2(getStyles);
  const liveMessage = toasts.map((t) => [t.title, t.text].filter(Boolean).join('. ')).join('. ');

  return (
    <>
      {children}
      <Portal>
        <div className={styles.visuallyHidden} role="log" aria-live="polite" aria-atomic="true">
          {liveMessage}
        </div>
        <div className={styles.container}>
          {toasts.map((toast) => (
            <ToastItem key={toast.id} toast={toast} />
          ))}
        </div>
      </Portal>
    </>
  );
}
