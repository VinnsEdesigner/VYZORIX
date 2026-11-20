import { useState } from 'react';
import { css, cx } from '@emotion/css';
import { getFocusStyles, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { AvatarLightbox } from '@/ui/pages/profile/avatar-lightbox';

export interface UserAvatarProps {
  name?: string | null;
  email?: string | null;
  src?: string | null;
  size?: number;
  expandable?: boolean;
}

export function initialsFor(name: string | null | undefined, email: string | null | undefined): string {
  const source = name?.trim() || email?.split('@')[0]?.trim() || '';
  if (!source) return '?';
  return source
    .split(/[\s._-]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((word) => word[0])
    .join('')
    .toUpperCase();
}

export function UserAvatar({ name, email, src, size = 28, expandable = false }: UserAvatarProps) {
  const styles = useStyles2(getStyles);
  const [open, setOpen] = useState(false);
  const label = name || email || 'User';
  const frameStyle = { width: size, height: size, fontSize: Math.round(size * 0.38) };

  const content = src ? (
    <img className={styles.image} src={src} alt="" />
  ) : (
    <span aria-hidden="true">{initialsFor(name, email)}</span>
  );

  if (!expandable || !src) {
    return (
      <span className={styles.frame} style={frameStyle} aria-hidden="true">
        {content}
      </span>
    );
  }

  return (
    <>
      <button
        type="button"
        className={cx(styles.frame, styles.expandable)}
        style={frameStyle}
        aria-label={`View ${label}'s profile picture`}
        onClick={() => setOpen(true)}
      >
        {content}
      </button>
      {open ? <AvatarLightbox src={src} alt={`${label} profile picture`} onClose={() => setOpen(false)} /> : null}
    </>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  frame: css({
    display: 'inline-grid',
    placeItems: 'center',
    flexShrink: 0,
    overflow: 'hidden',
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.background.secondary,
    color: theme.colors.text.primary,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: 1,
  }),
  expandable: css({
    padding: 0,
    border: 0,
    cursor: 'zoom-in',
    '&:focus-visible': getFocusStyles(theme),
  }),
  image: css({ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }),
});
