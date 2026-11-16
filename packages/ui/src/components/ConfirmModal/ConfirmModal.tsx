import { Modal } from '../Modal';
import { Button } from '../Button';
import { Text } from '../Text';
import { useStyles2 } from '../../theme/ThemeContext';
import { css } from '@emotion/css';
import { type VyzorixTheme } from '../../theme/types';

export interface ConfirmModalProps {
  title: string;
  body: React.ReactNode;
  confirmText?: string;
  dismissText?: string;
  onConfirm: () => void;
  onDismiss: () => void;
  isOpen: boolean;
  /** Use the destructive colour for the confirm action. */
  destructive?: boolean;
  isConfirming?: boolean;
}

const getStyles = (theme: VyzorixTheme) => ({
  actions: css({ display: 'flex', justifyContent: 'flex-end', gap: theme.spacing(1), marginTop: theme.spacing(3) }),
});

/** A confirmation dialog for an action that is hard to undo. */
export function ConfirmModal({
  title,
  body,
  confirmText = 'Confirm',
  dismissText = 'Cancel',
  onConfirm,
  onDismiss,
  isOpen,
  destructive,
  isConfirming,
}: ConfirmModalProps) {
  const styles = useStyles2(getStyles);

  return (
    <Modal title={title} isOpen={isOpen} onDismiss={onDismiss} width={400}>
      <Text color="secondary">{body}</Text>
      <div className={styles.actions}>
        <Button variant="secondary" fill="text" onClick={onDismiss}>
          {dismissText}
        </Button>
        <Button variant={destructive ? 'destructive' : 'primary'} onClick={onConfirm} disabled={isConfirming}>
          {confirmText}
        </Button>
      </div>
    </Modal>
  );
}
