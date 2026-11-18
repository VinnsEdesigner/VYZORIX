import { useEffect, useRef, useState } from 'react';
import { css, cx } from '@emotion/css';
import {
  Button,
  IconButton,
  Portal,
  Slider,
  getFocusStyles,
  useStyles2,
  type VyzorixTheme,
} from '@vyzorix/ui';
import { useVyzorTranslation } from '@/hooks/i18n';

const OUTPUT_SIZE = 512;
const VIEW_SIZE = 300;
const MIN_ZOOM = 1;
const MAX_ZOOM = 3;

interface Effect {
  id: string;
  label: string;
  filter: string;
}

const EFFECTS: Effect[] = [
  { id: 'none', label: 'avatar.effect.original', filter: 'none' },
  { id: 'mono', label: 'avatar.effect.mono', filter: 'grayscale(1)' },
  { id: 'warm', label: 'avatar.effect.warm', filter: 'sepia(0.35) saturate(1.25)' },
  { id: 'cool', label: 'avatar.effect.cool', filter: 'hue-rotate(180deg) saturate(1.15)' },
  { id: 'vivid', label: 'avatar.effect.vivid', filter: 'saturate(1.6) contrast(1.1)' },
];

interface Offset {
  x: number;
  y: number;
}

function coverScale(image: HTMLImageElement, zoom: number): number {
  return Math.max(VIEW_SIZE / image.width, VIEW_SIZE / image.height) * zoom;
}

function clampOffset(offset: Offset, image: HTMLImageElement, zoom: number): Offset {
  const scale = coverScale(image, zoom);
  const maxX = Math.max(0, (image.width * scale - VIEW_SIZE) / 2);
  const maxY = Math.max(0, (image.height * scale - VIEW_SIZE) / 2);
  return {
    x: Math.min(maxX, Math.max(-maxX, offset.x)),
    y: Math.min(maxY, Math.max(-maxY, offset.y)),
  };
}

function paint(
  canvas: HTMLCanvasElement,
  image: HTMLImageElement,
  zoom: number,
  offset: Offset,
  filter: string,
): void {
  const ctx = canvas.getContext('2d');
  if (!ctx) return;
  const size = canvas.width;
  const scale = Math.max(size / image.width, size / image.height) * zoom;
  const width = image.width * scale;
  const height = image.height * scale;
  const maxX = Math.max(0, (width - size) / 2);
  const maxY = Math.max(0, (height - size) / 2);
  const dx = Math.min(maxX, Math.max(-maxX, (offset.x / VIEW_SIZE) * size));
  const dy = Math.min(maxY, Math.max(-maxY, (offset.y / VIEW_SIZE) * size));

  ctx.clearRect(0, 0, size, size);
  ctx.save();
  ctx.filter = filter;
  ctx.drawImage(image, (size - width) / 2 + dx, (size - height) / 2 + dy, width, height);
  ctx.restore();
}

export interface AvatarEditorProps {
  file: File;
  isSaving?: boolean;
  onCancel: () => void;
  onSave: (image: Blob) => void;
}

export function AvatarEditor({ file, isSaving, onCancel, onSave }: AvatarEditorProps) {
  const styles = useStyles2(getStyles);
  const { t } = useVyzorTranslation();
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const dragRef = useRef<{ x: number; y: number; ox: number; oy: number } | null>(null);

  const [image, setImage] = useState<HTMLImageElement | null>(null);
  const [failed, setFailed] = useState(false);
  const [zoom, setZoom] = useState(MIN_ZOOM);
  const [offset, setOffset] = useState<Offset>({ x: 0, y: 0 });
  const [effectId, setEffectId] = useState('none');

  const effect = EFFECTS.find((item) => item.id === effectId) ?? EFFECTS[0]!;

  useEffect(() => {
    const url = URL.createObjectURL(file);
    const next = new Image();
    next.onload = () => setImage(next);
    next.onerror = () => setFailed(true);
    next.src = url;
    return () => URL.revokeObjectURL(url);
  }, [file]);

  useEffect(() => {
    if (!image || !canvasRef.current) return;
    paint(canvasRef.current, image, zoom, clampOffset(offset, image, zoom), effect.filter);
  }, [image, zoom, offset, effect]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onCancel();
    };
    document.addEventListener('keydown', onKey);
    const previous = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.removeEventListener('keydown', onKey);
      document.body.style.overflow = previous;
    };
  }, [onCancel]);

  const onPointerDown = (event: React.PointerEvent<HTMLCanvasElement>) => {
    if (!image) return;
    event.currentTarget.setPointerCapture(event.pointerId);
    dragRef.current = { x: event.clientX, y: event.clientY, ox: offset.x, oy: offset.y };
  };

  const onPointerMove = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const drag = dragRef.current;
    if (!drag || !image) return;
    const next = { x: drag.ox + (event.clientX - drag.x), y: drag.oy + (event.clientY - drag.y) };
    setOffset(clampOffset(next, image, zoom));
  };

  const onPointerUp = () => {
    dragRef.current = null;
  };

  const onZoom = (value: number) => {
    setZoom(value);
    if (image) setOffset((current) => clampOffset(current, image, value));
  };

  const onSaveClick = () => {
    if (!image) return;
    const output = document.createElement('canvas');
    output.width = OUTPUT_SIZE;
    output.height = OUTPUT_SIZE;
    paint(output, image, zoom, clampOffset(offset, image, zoom), effect.filter);
    output.toBlob(
      (blob) => {
        if (blob) onSave(blob);
      },
      'image/jpeg',
      0.92,
    );
  };

  return (
    <Portal>
      <div className={styles.backdrop} onMouseDown={(event) => event.target === event.currentTarget && onCancel()}>
        <div role="dialog" aria-modal="true" aria-label={t('avatar.title')} className={styles.panel}>
          <header className={styles.head}>
            <div className={styles.headText}>
              <span className={styles.title}>{t('avatar.title')}</span>
              <span className={styles.subtitle}>{t('avatar.hint')}</span>
            </div>
            <IconButton name="x" aria-label={t('avatar.closeEditor')} fill="text" size="sm" onClick={onCancel} />
          </header>

          <div className={styles.stage}>
            {failed ? (
              <div className={styles.failed}>{t('avatar.couldNotRead')}</div>
            ) : (
              <canvas
                ref={canvasRef}
                width={VIEW_SIZE}
                height={VIEW_SIZE}
                className={styles.canvas}
                onPointerDown={onPointerDown}
                onPointerMove={onPointerMove}
                onPointerUp={onPointerUp}
                onPointerCancel={onPointerUp}
                aria-label={t('avatar.cropPreview')}
              />
            )}
          </div>

          <label className={styles.control}>
            <span className={styles.controlLabel}>{t('avatar.zoom')}</span>
            <Slider
              min={MIN_ZOOM}
              max={MAX_ZOOM}
              step={0.01}
              value={zoom}
              onChange={(event) => onZoom(Number(event.target.value))}
              aria-label={t('avatar.zoom')}
            />
          </label>

          <div className={styles.control}>
            <span className={styles.controlLabel}>{t('avatar.effects')}</span>
            <div className={styles.effects}>
              {EFFECTS.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  className={cx(styles.effect, item.id === effectId && styles.effectActive)}
                  aria-pressed={item.id === effectId}
                  onClick={() => setEffectId(item.id)}
                >
                  {t(item.label)}
                </button>
              ))}
            </div>
          </div>

          <footer className={styles.foot}>
            <Button variant="secondary" fill="text" onClick={onCancel} disabled={isSaving}>
              {t('avatar.discard')}
            </Button>
            <Button variant="accent" onClick={onSaveClick} loading={isSaving} disabled={!image || failed}>
              {t('avatar.save')}
            </Button>
          </footer>
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
    background: 'rgba(0, 0, 0, 0.65)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    padding: theme.spacing(2),
  }),
  panel: css({
    zIndex: theme.zIndex.modal,
    width: '100%',
    maxWidth: 420,
    maxHeight: '92vh',
    overflowY: 'auto',
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2),
    padding: theme.spacing(2.5),
    background: theme.colors.background.primary,
    borderRadius: theme.shape.radius.lg,
    boxShadow: theme.shadows.z3,
  }),
  head: css({ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: theme.spacing(2) }),
  headText: css({ display: 'flex', flexDirection: 'column', gap: 2 }),
  title: css({ color: theme.colors.text.primary, fontWeight: theme.typography.fontWeightMedium }),
  subtitle: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  stage: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    background: theme.colors.background.canvas,
    borderRadius: theme.shape.radius.default,
    padding: theme.spacing(1.5),
  }),
  canvas: css({
    width: VIEW_SIZE,
    maxWidth: '100%',
    height: 'auto',
    aspectRatio: '1 / 1',
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.background.secondary,
    cursor: 'grab',
    touchAction: 'none',
    '&:active': { cursor: 'grabbing' },
  }),
  failed: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
    padding: theme.spacing(4, 2),
    textAlign: 'center',
  }),
  control: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  controlLabel: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  effects: css({ display: 'flex', flexWrap: 'wrap', gap: theme.spacing(0.75) }),
  effect: css({
    border: 0,
    borderRadius: theme.shape.radius.pill,
    padding: `${theme.spacing(0.5)} ${theme.spacing(1.25)}`,
    background: theme.colors.background.secondary,
    color: theme.colors.text.secondary,
    fontFamily: 'inherit',
    fontSize: theme.typography.sm.fontSize,
    cursor: 'pointer',
    '&:hover': { color: theme.colors.text.primary },
    '&:focus-visible': getFocusStyles(theme),
  }),
  effectActive: css({
    background: theme.colors.accent.subtleBackground,
    color: theme.colors.accent.text,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  foot: css({ display: 'flex', justifyContent: 'flex-end', gap: theme.spacing(1) }),
});
