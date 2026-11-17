import { css, cx } from '@emotion/css';
import { useEffect, useState } from 'react';
import { useStyles2 } from '../../theme/ThemeContext';
import { type VyzorixTheme } from '../../theme/types';
import { useBranding } from './BrandingContext';

export interface BootScreenProps {
  /** Message under the mark. */
  label?: string;
  className?: string;
  /** How long the one-shot animation runs before the loop takes over, in ms. */
  handoffMs?: number;
}

function prefersReducedMotion(): boolean {
  if (typeof window === 'undefined' || !window.matchMedia) return false;
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

const getStyles = (theme: VyzorixTheme) => ({
  screen: css({
    position: 'fixed',
    inset: 0,
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    gap: theme.spacing(3),
    background: theme.colors.background.canvas,
    zIndex: theme.zIndex.modal,
  }),
  mark: css({
    width: 96,
    height: 'auto',
    display: 'block',
  }),
  label: css({
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.secondary,
  }),
});

/**
 * The app-open screen.
 *
 * Plays the one-shot assemble animation, then hands off to the looping orbit so
 * a slow boot keeps moving. Both are SMIL inside the SVG, which is the only way
 * an `<img>`-loaded SVG can animate — the page's stylesheet does not reach it.
 *
 * Under `prefers-reduced-motion` it shows the still mark instead: SMIL inside an
 * `<img>` cannot be paused from the page, so the choice has to be made here by
 * swapping the source.
 */
export function BootScreen({ label, className, handoffMs = 1800 }: BootScreenProps) {
  const branding = useBranding();
  const styles = useStyles2(getStyles);

  const [reduced] = useState(prefersReducedMotion);
  const [phase, setPhase] = useState<'assemble' | 'loop'>('assemble');

  useEffect(() => {
    if (reduced) return;
    const id = window.setTimeout(() => setPhase('loop'), handoffMs);
    return () => window.clearTimeout(id);
  }, [reduced, handoffMs]);

  const src = reduced
    ? branding.loadingLogo
    : phase === 'assemble'
      ? branding.bootAnimation
      : branding.bootAnimationLoop;

  return (
    <div className={cx(styles.screen, className)} role="status" aria-label={label ?? 'Loading'}>
      <img src={src} alt="" className={styles.mark} aria-hidden />
      {label ? <div className={styles.label}>{label}</div> : null}
    </div>
  );
}
