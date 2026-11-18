import { useEffect } from 'react';
import { css } from '@emotion/css';
import { Icon, Portal, getFocusStyles, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { useVyzorTranslation } from '@/hooks/i18n';

export interface AvatarLightboxProps {
  src: string;
  alt: string;
  onClose: () => void;
}

export function AvatarLightbox({ src, alt, onClose }: AvatarLightboxProps) {
  const styles = useStyles2(getStyles);
  const { t } = useVyzorTranslation();

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKey);
    const previous = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.removeEventListener('keydown', onKey);
      document.body.style.overflow = previous;
    };
  }, [onClose]);

  return (
    <Portal>
      <div
        className={styles.backdrop}
        role="dialog"
        aria-modal="true"
        aria-label={alt}
        onMouseDown={(event) => event.target === event.currentTarget && onClose()}
      >
        <div className={styles.frame}>
          <img className={styles.image} src={src} alt={alt} />
          <button type="button" className={styles.close} aria-label={t('close')} onClick={onClose}>
            <Icon name="x" size="md" />
          </button>
        </div>
      </div>
    </Portal>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  backdrop: css({
    position: 'fixed',
    inset: 0,
    zIndex: theme.zIndex.modalBackdrop,
    background: 'rgba(0, 0, 0, 0.85)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    padding: theme.spacing(3),
  }),
  frame: css({ position: 'relative', display: 'inline-flex', maxWidth: '90vw', maxHeight: '90vh' }),
  image: css({
    maxWidth: '90vw',
    maxHeight: '90vh',
    borderRadius: theme.shape.radius.lg,
    boxShadow: theme.shadows.z3,
    display: 'block',
  }),
  close: css({
    position: 'absolute',
    top: theme.spacing(1),
    right: theme.spacing(1),
    display: 'grid',
    placeItems: 'center',
    width: 36,
    height: 36,
    padding: 0,
    border: 0,
    borderRadius: theme.shape.radius.pill,
    background: 'rgba(0, 0, 0, 0.55)',
    color: '#fff',
    cursor: 'pointer',
    transition: 'background .15s ease',
    '&:hover': { background: 'rgba(0, 0, 0, 0.75)' },
    '&:focus-visible': getFocusStyles(theme),
  }),
});
