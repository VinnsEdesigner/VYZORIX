import { useMemo, useState } from 'react';
import { css, cx } from '@emotion/css';
import { useNavigate } from '@tanstack/react-router';
import { Button, Icon, getFocusStyles, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import {
  AVATAR_STYLES,
  BACKGROUND_SWATCHES,
  avatarStyleById,
  createAvatarDataUri,
  randomSeed,
  renderAvatarPng,
  type AvatarOptions,
} from '@/lib/avatar';
import { useCurrentUser } from '@/hooks/_shared';
import { useUploadAvatar } from '@/hooks/profile/use-profile';
import { notifyError, notifySuccess } from '@/stores/toast-store';

const STYLE_FILTERS: { id: string; label: string; styles: string[] }[] = [
  { id: 'all', label: 'All', styles: [] },
  {
    id: 'people',
    label: 'People',
    styles: ['notionists', 'adventurer', 'avataaars', 'lorelei', 'micah', 'miniavs', 'open-peeps', 'personas', 'big-ears', 'croodles', 'dylan'],
  },
  { id: 'characters', label: 'Characters', styles: ['bottts', 'fun-emoji', 'thumbs', 'pixel-art'] },
  { id: 'abstract', label: 'Abstract', styles: ['identicon', 'rings', 'shapes', 'glass', 'icons'] },
];

const SWATCH_LABELS: Record<string, string> = {
  transparent: 'Transparent',
  '#f43f5e': 'Rose 500',
  '#e11d48': 'Rose 600',
  '#fb7185': 'Rose 400',
  '#f59e0b': 'Amber',
  '#22c55e': 'Green',
  '#0ea5e9': 'Sky',
  '#6366f1': 'Indigo',
  '#a855f7': 'Purple',
  '#171717': 'Near black',
  '#1c1c1c': 'Charcoal',
  '#e5e5e5': 'Light',
};

export function AvatarChooserPage() {
  const styles = useStyles2(getStyles);
  const navigate = useNavigate();
  const user = useCurrentUser();
  const upload = useUploadAvatar();

  const [filter, setFilter] = useState('all');
  const [styleId, setStyleId] = useState('notionists');
  const [seed, setSeed] = useState(() => user.name || user.email || 'vyzorix');
  const [background, setBackground] = useState('transparent');

  const visibleStyles = useMemo(() => {
    const active = STYLE_FILTERS.find((item) => item.id === filter);
    if (!active || active.styles.length === 0) return AVATAR_STYLES;
    return AVATAR_STYLES.filter((item) => active.styles.includes(item.id));
  }, [filter]);

  const options: AvatarOptions = { styleId, seed, background };
  const preview = createAvatarDataUri(options, 256);
  const activeStyle = avatarStyleById(styleId);

  const onSave = async () => {
    try {
      const blob = await renderAvatarPng(options, 512);
      const file = new File([blob], 'avatar.png', { type: 'image/png' });
      await upload.mutateAsync(file);
      notifySuccess('Avatar saved');
      void navigate({ to: '/profile' });
    } catch {
      notifyError('Could not save the avatar', 'Please try again.');
    }
  };

  return (
    <div className={styles.page}>
      <div className={styles.hero}>
        <div className={styles.previewWrap}>
          <img className={styles.preview} src={preview} alt="Avatar preview" />
        </div>
        <div className={styles.heroBody}>
          <div className={styles.heroTitle}>Choose an avatar</div>
          <div className={styles.heroHint}>
            Pick a style, then shuffle the seed until you like the look. You can change the
            background to match any theme.
          </div>
          <div className={styles.heroActions}>
            <Button variant="secondary" fill="outline" icon="refresh-cw" onClick={() => setSeed(randomSeed())}>
              Shuffle
            </Button>
            <Button variant="secondary" fill="text" onClick={() => setSeed(user.name || user.email || 'vyzorix')}>
              Use my name
            </Button>
          </div>
        </div>
      </div>

      <div className={styles.filters}>
        {STYLE_FILTERS.map((item) => (
          <button
            key={item.id}
            type="button"
            className={cx(styles.filter, item.id === filter && styles.filterActive)}
            aria-pressed={item.id === filter}
            onClick={() => setFilter(item.id)}
          >
            {item.label}
          </button>
        ))}
      </div>

      <div className={styles.grid}>
        {visibleStyles.map((item) => (
          <button
            key={item.id}
            type="button"
            className={cx(styles.styleTile, item.id === styleId && styles.styleTileActive)}
            aria-pressed={item.id === styleId}
            onClick={() => setStyleId(item.id)}
          >
            <img
              className={styles.styleThumb}
              src={createAvatarDataUri({ styleId: item.id, seed, background }, 96)}
              alt=""
            />
            <span className={styles.styleLabel}>{item.label}</span>
            {item.id === styleId ? (
              <span className={styles.styleCheck} aria-hidden="true">
                <Icon name="check" size="sm" />
              </span>
            ) : null}
          </button>
        ))}
      </div>

      <div className={styles.options}>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>Seed</span>
          <input
            className={styles.textInput}
            value={seed}
            onChange={(event) => setSeed(event.target.value)}
            placeholder="A word, a name, anything"
          />
        </label>

        <div className={styles.field}>
          <span className={styles.fieldLabel}>Background</span>
          <div className={styles.swatches}>
            {BACKGROUND_SWATCHES.map((swatch) => (
              <button
                key={swatch}
                type="button"
                className={cx(styles.swatch, swatch === background && styles.swatchActive)}
                style={swatch === 'transparent' ? undefined : { background: swatch }}
                aria-label={SWATCH_LABELS[swatch] ?? swatch}
                aria-pressed={swatch === background}
                title={SWATCH_LABELS[swatch] ?? swatch}
                onClick={() => setBackground(swatch)}
              >
                {swatch === 'transparent' ? <Icon name="x" size="sm" /> : null}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className={styles.footer}>
        <span className={styles.footerStyle}>
          Style: <b>{activeStyle.label}</b>
        </span>
        <div className={styles.footerActions}>
          <Button variant="secondary" fill="text" onClick={() => void navigate({ to: '/profile' })} disabled={upload.isPending}>
            Cancel
          </Button>
          <Button variant="accent" onClick={onSave} loading={upload.isPending}>
            Save avatar
          </Button>
        </div>
      </div>
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  page: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3) }),
  hero: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(3),
    padding: theme.spacing(3),
    background: theme.colors.background.primary,
    borderRadius: theme.shape.radius.lg,
    [theme.breakpoints.down('sm')]: { flexDirection: 'column', textAlign: 'center' },
  }),
  previewWrap: css({
    display: 'grid',
    placeItems: 'center',
    width: 128,
    height: 128,
    flexShrink: 0,
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.background.secondary,
    overflow: 'hidden',
  }),
  preview: css({ width: '100%', height: '100%', objectFit: 'cover' }),
  heroBody: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5), minWidth: 0 }),
  heroTitle: css({ fontSize: theme.typography.lg.fontSize, fontWeight: theme.typography.fontWeightMedium }),
  heroHint: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  heroActions: css({ display: 'flex', gap: theme.spacing(1), marginTop: theme.spacing(1), flexWrap: 'wrap' }),
  filters: css({ display: 'flex', flexWrap: 'wrap', gap: theme.spacing(0.75) }),
  filter: css({
    border: 0,
    borderRadius: theme.shape.radius.pill,
    padding: `${theme.spacing(0.5)} ${theme.spacing(1.5)}`,
    background: theme.colors.background.secondary,
    color: theme.colors.text.secondary,
    fontFamily: 'inherit',
    fontSize: theme.typography.sm.fontSize,
    cursor: 'pointer',
    '&:hover': { color: theme.colors.text.primary },
    '&:focus-visible': getFocusStyles(theme),
  }),
  filterActive: css({
    background: theme.colors.accent.subtleBackground,
    color: theme.colors.accent.text,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(104px, 1fr))',
    gap: theme.spacing(1.5),
  }),
  styleTile: css({
    position: 'relative',
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(1.5),
    border: `1px solid transparent`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    cursor: 'pointer',
    '&:hover': { background: theme.colors.background.secondary },
    '&:focus-visible': getFocusStyles(theme),
  }),
  styleTileActive: css({
    borderColor: theme.colors.accent.main,
    background: theme.colors.background.secondary,
  }),
  styleThumb: css({
    width: 64,
    height: 64,
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.background.secondary,
  }),
  styleLabel: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  styleCheck: css({
    position: 'absolute',
    top: theme.spacing(0.75),
    right: theme.spacing(0.75),
    display: 'grid',
    placeItems: 'center',
    width: 22,
    height: 22,
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.accent.main,
    color: theme.colors.accent.contrastText,
  }),
  options: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2),
    padding: theme.spacing(2.5),
    background: theme.colors.background.primary,
    borderRadius: theme.shape.radius.lg,
  }),
  field: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1), maxWidth: 420 }),
  fieldLabel: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  textInput: css({
    borderRadius: theme.shape.radius.sm,
    border: `1px solid ${theme.colors.border.medium}`,
    padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`,
    background: theme.colors.background.primary,
    color: theme.colors.text.primary,
    fontFamily: 'inherit',
    fontSize: theme.typography.base.fontSize,
    '&:focus-visible': getFocusStyles(theme, true),
  }),
  swatches: css({ display: 'flex', flexWrap: 'wrap', gap: theme.spacing(1) }),
  swatch: css({
    display: 'grid',
    placeItems: 'center',
    width: 30,
    height: 30,
    padding: 0,
    border: `1px solid ${theme.colors.border.medium}`,
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.background.secondary,
    color: theme.colors.text.disabled,
    cursor: 'pointer',
    '&:focus-visible': getFocusStyles(theme),
  }),
  swatchActive: css({ boxShadow: `0 0 0 2px ${theme.colors.background.primary}, 0 0 0 4px ${theme.colors.accent.main}` }),
  footer: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
  }),
  footerStyle: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  footerActions: css({ display: 'flex', gap: theme.spacing(1), marginLeft: 'auto' }),
});
