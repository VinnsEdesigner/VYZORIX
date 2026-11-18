import { useEffect, useRef, useState } from 'react';
import { css, cx } from '@emotion/css';
import { useNavigate } from '@tanstack/react-router';
import {
  Button,
  Icon,
  Tag,
  getFocusStyles,
  useStyles2,
  type VyzorixTheme,
} from '@vyzorix/ui';
import { useCurrentUser } from '@/hooks/_shared';
import { useMe } from '@/hooks/auth';
import { useSessions } from '@/hooks/session';
import { useProfileSummary, useUpdateProfileName, useUploadAvatar } from '@/hooks/profile/use-profile';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import { describeUserAgent } from '@/lib/session/describe-user-agent';
import { useVyzorTranslation } from '@/hooks/i18n';
import { AvatarEditor } from './avatar-editor';
import { AvatarLightbox } from './avatar-lightbox';

const ACCEPTED_IMAGE_TYPES = 'image/png,image/jpeg,image/gif,image/webp';
const MAX_AVATAR_BYTES = 5 * 1024 * 1024;

function initialsFor(name: string | null, email: string | null): string {
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

export function formatDate(ms: number | null | undefined): string {
  if (!ms) return '—';
  return new Date(ms).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

export function formatDateTime(ms: number | null | undefined): string {
  if (!ms) return '—';
  return new Date(ms).toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function MetaRow({ label, value }: { label: string; value: React.ReactNode }) {
  const styles = useStyles2(getStyles);
  return (
    <div className={styles.metaRow}>
      <span className={styles.metaLabel}>{label}</span>
      <span className={styles.metaValue}>{value}</span>
    </div>
  );
}

function SectionCard({
  title,
  subTitle,
  children,
}: {
  title: string;
  subTitle?: string;
  children: React.ReactNode;
}) {
  const styles = useStyles2(getStyles);
  return (
    <section className={styles.card}>
      <header className={styles.cardHead}>
        <h2 className={styles.cardTitle}>{title}</h2>
        {subTitle ? <span className={styles.cardSub}>{subTitle}</span> : null}
      </header>
      <div className={styles.cardBody}>{children}</div>
    </section>
  );
}

export function ProfilePage() {
  const styles = useStyles2(getStyles);
  const user = useCurrentUser();
  const { refetch: refetchMe } = useMe();
  const sessions = useSessions();
  const summary = useProfileSummary();
  const upload = useUploadAvatar();
  const updateName = useUpdateProfileName();
  const navigate = useNavigate();
  const { t } = useVyzorTranslation();

  const [me, setMe] = useState<{ created_at?: number; last_login_at?: number; mfa_enabled?: boolean } | null>(null);
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(user.name ?? '');
  const [lightbox, setLightbox] = useState(false);
  const [pendingFile, setPendingFile] = useState<File | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    let cancelled = false;
    void refetchMe().then((data) => {
      if (!cancelled && data) setMe(data as { created_at?: number; last_login_at?: number; mfa_enabled?: boolean });
    });
    return () => {
      cancelled = true;
    };
  }, [refetchMe]);

  useEffect(() => {
    setName(user.name ?? '');
  }, [user.name]);

  const currentSession = sessions.data?.sessions?.find((s) => s.is_current) ?? sessions.data?.sessions?.[0];
  const activeSessions = sessions.data?.total ?? summary.data?.active_sessions ?? 0;

  const displayName = user.name ?? user.email ?? 'Account';
  const dirty = name.trim() !== (user.name ?? '');

  const onPickFile = () => fileInputRef.current?.click();

  const onFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;

    if (!file.type.startsWith('image/')) {
      notifyError('Not an image', 'Choose a PNG, JPEG, GIF or WebP file.');
      return;
    }
    if (file.size > MAX_AVATAR_BYTES) {
      notifyError('Image too large', 'Choose an image under 5 MB.');
      return;
    }

    setPendingFile(file);
  };

  const onSaveAvatar = (image: Blob) => {
    const file = new File([image], 'avatar.jpg', { type: 'image/jpeg' });
    upload.mutate(file, {
      onSuccess: () => {
        notifySuccess('Profile picture updated');
        setPendingFile(null);
      },
      onError: () => notifyError('Could not update the picture', 'Please try again.'),
    });
  };

  const onSaveName = () => {
    if (!dirty) return;
    updateName.mutate(name.trim(), {
      onSuccess: () => {
        notifySuccess('Profile updated');
        setEditing(false);
      },
      onError: () => notifyError('Could not update the profile', 'Please try again.'),
    });
  };

  const onCancelEdit = () => {
    setName(user.name ?? '');
    setEditing(false);
  };

  return (
    <div className={styles.page}>
      <div className={styles.layout}>
        {}
        <aside className={styles.identity}>
          {user.avatarUrl ? (
            <button
              type="button"
              className={styles.avatarButton}
              onClick={() => setLightbox(true)}
              aria-label={t('profile.viewFullPicture')}
            >
              <img className={styles.avatarImg} src={user.avatarUrl} alt="" />
              <span className={cx('vyz-avatar-zoom', styles.avatarZoom)} aria-hidden="true">
                <Icon name="search" size="sm" />
              </span>
            </button>
          ) : (
            <div className={styles.avatar}>
              <span aria-hidden="true">{initialsFor(user.name, user.email)}</span>
            </div>
          )}
          <div className={styles.identityName}>{displayName}</div>
          {user.email ? <div className={styles.identityEmail}>{user.email}</div> : null}

          <div className={styles.metaList}>
            <MetaRow label={t('profile.email')} value={user.email ?? '—'} />
            <MetaRow label={t('profile.joined')} value={formatDate(me?.created_at)} />
            <MetaRow label={t('profile.lastLogin')} value={formatDateTime(me?.last_login_at)} />
          </div>

          <div className={styles.sessionBlock}>
            <div className={styles.sessionLabel}>{t('profile.currentSession')}</div>
            <div className={styles.metaList}>
              <MetaRow label={t('profile.ipAddress')} value={currentSession?.ip_address || '—'} />
              <MetaRow label={t('profile.browserOs')} value={describeUserAgent(currentSession?.user_agent)} />
            </div>
          </div>

          <div className={styles.uploadBlock}>
            <Button
              variant="accent"
              fullWidth
              onClick={onPickFile}
              disabled={!editing || upload.isPending}
              title={editing ? undefined : t('profile.clickEditPicture')}
            >
              {upload.isPending ? t('profile.saving') : user.avatarUrl ? t('profile.changeProfilePicture') : t('profile.uploadProfilePicture')}
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              accept={ACCEPTED_IMAGE_TYPES}
              className={styles.hiddenInput}
              onChange={onFileChange}
              data-testid="avatar-input"
            />
            <Button
              variant="secondary"
              fill="outline"
              fullWidth
              onClick={() => void navigate({ to: '/profile/avatar' })}
              disabled={!editing}
              title={editing ? undefined : t('profile.clickEditAvatar')}
            >
              {t('profile.chooseAvatar')}
            </Button>
          </div>
        </aside>

        {}
        <div className={styles.detail}>
          {}
          <SectionCard title={t('profile.title')} subTitle={t('profile.identity')}>
            {editing ? (
              <div className={styles.editForm}>
                <label className={styles.field}>
                  <span className={styles.fieldLabel}>{t('profile.displayName')}</span>
                  <input
                    className={styles.textInput}
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    autoComplete="name"
                    autoFocus
                  />
                </label>

                <div className={styles.field}>
                  <span className={styles.fieldLabel}>{t('profile.email')}</span>
                  <div className={styles.lockedField} title={t('profile.lockedHint')}>
                    <span className={styles.lockedValue}>{user.email ?? '—'}</span>
                    <Icon name="lock" size="sm" className={styles.lockIcon} />
                  </div>
                  <span className={styles.fieldHint}>{t('profile.emailLockedHint')}</span>
                </div>

                <div className={styles.field}>
                  <span className={styles.fieldLabel}>{t('profile.legalName')}</span>
                  <div className={styles.lockedField} title={t('profile.lockedHint')}>
                    <span className={styles.lockedValue}>{user.name ?? '—'}</span>
                    <Icon name="lock" size="sm" className={styles.lockIcon} />
                  </div>
                  <span className={styles.fieldHint}>{t('profile.legalNameLockedHint')}</span>
                </div>

                <div className={styles.editActions}>
                  <Button variant="primary" onClick={onSaveName} disabled={!dirty || updateName.isPending}>
                    {updateName.isPending ? t('profile.saving') : t('save')}
                  </Button>
                  <Button variant="secondary" fill="text" onClick={onCancelEdit}>
                    {t('cancel')}
                  </Button>
                </div>
              </div>
            ) : (
              <div className={styles.viewRow}>
                <div className={styles.viewIdentity}>
                  <div className={styles.viewName}>{displayName}</div>
                  <div className={styles.viewEmail}>{user.email ?? ''}</div>
                </div>
                <Button variant="secondary" fill="outline" onClick={() => setEditing(true)}>
                  {t('profile.editProfile')}
                </Button>
              </div>
            )}
          </SectionCard>

          {}
          <SectionCard title={t('profile.activity')}>
            <div className={styles.statRow}>
              {}
              <div className={cx(styles.stat, styles.statStatic)}>
                <span className={styles.statValue}>{activeSessions}</span>
                <span className={styles.statLabel}>{t('profile.activeSessions')}</span>
              </div>
              {}
              <div className={cx(styles.stat, styles.statStatic)}>
                <span className={styles.statValue}>{summary.data?.actions ?? '—'}</span>
                <span className={styles.statLabel}>{t('profile.actionsCount')}</span>
              </div>
            </div>
          </SectionCard>

          {}
          <SectionCard title={t('profile.security')}>
            <div className={styles.rowBetween}>
              <div>
                <div className={styles.rowTitle}>{t('profile.passwordMfa')}</div>
                <div className={styles.rowHint}>
                  {t('profile.passwordMfaHint')}
                </div>
              </div>
              <Button variant="secondary" fill="outline" disabled title={t('profile.comingSoon')}>
                {t('profile.manage')}
              </Button>
            </div>
            {me?.mfa_enabled ? (
              <div className={styles.badges}>
                <Tag name={t('profile.mfaOn')} colorIndex={1} />
              </div>
            ) : null}
          </SectionCard>
        </div>
      </div>

      {lightbox && user.avatarUrl ? (
        <AvatarLightbox
          src={user.avatarUrl}
          alt={`${displayName} profile picture`}
          onClose={() => setLightbox(false)}
        />
      ) : null}

      {pendingFile ? (
        <AvatarEditor
          file={pendingFile}
          isSaving={upload.isPending}
          onCancel={() => setPendingFile(null)}
          onSave={onSaveAvatar}
        />
      ) : null}
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  page: css({ maxWidth: 1080, margin: '0 auto', width: '100%' }),
  layout: css({
    display: 'grid',
    gridTemplateColumns: '320px 1fr',
    gap: theme.spacing(3),
    alignItems: 'start',
    [theme.breakpoints.down('md')]: { gridTemplateColumns: '1fr' },
  }),
  identity: css({
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    textAlign: 'center',
    gap: theme.spacing(2),
    padding: theme.spacing(3),
    background: theme.colors.background.primary,
    borderRadius: theme.shape.radius.lg,
    [theme.breakpoints.up('md')]: { position: 'sticky', top: 0 },
  }),
  avatar: css({
    display: 'grid',
    placeItems: 'center',
    width: 160,
    height: 160,
    overflow: 'hidden',
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.background.secondary,
    color: theme.colors.text.primary,
    fontSize: 52,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  avatarButton: css({
    position: 'relative',
    display: 'grid',
    placeItems: 'center',
    width: 160,
    height: 160,
    padding: 0,
    overflow: 'hidden',
    border: 0,
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.background.secondary,
    cursor: 'zoom-in',
    '&:focus-visible': getFocusStyles(theme),
    '&:hover .vyz-avatar-zoom, &:focus-visible .vyz-avatar-zoom': { opacity: 1 },
  }),
  avatarZoom: css({
    position: 'absolute',
    right: 10,
    bottom: 10,
    display: 'grid',
    placeItems: 'center',
    width: 32,
    height: 32,
    borderRadius: theme.shape.radius.pill,
    background: 'rgba(0, 0, 0, 0.55)',
    color: '#fff',
    opacity: 0,
    transition: 'opacity .15s ease',
  }),
  avatarImg: css({ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }),
  identityName: css({
    fontSize: theme.typography.lg.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  identityEmail: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  metaList: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1), width: '100%' }),
  metaRow: css({
    display: 'flex',
    alignItems: 'baseline',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    fontSize: theme.typography.sm.fontSize,
    textAlign: 'left',
  }),
  metaLabel: css({ color: theme.colors.text.secondary, flexShrink: 0 }),
  metaValue: css({ color: theme.colors.text.primary, textAlign: 'right', wordBreak: 'break-word' }),
  sessionBlock: css({ width: '100%' }),
  sessionLabel: css({
    textAlign: 'left',
    color: theme.colors.text.disabled,
    fontSize: theme.typography.sm.fontSize,
    marginBottom: theme.spacing(0.5),
  }),
  uploadBlock: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1), width: '100%' }),
  hiddenInput: css({ display: 'none' }),
  detail: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3), minWidth: 0 }),
  card: css({
    background: theme.colors.background.primary,
    borderRadius: theme.shape.radius.lg,
  }),
  cardHead: css({
    display: 'flex',
    alignItems: 'baseline',
    gap: theme.spacing(1.5),
    padding: theme.spacing(2, 2.5),
  }),
  cardTitle: css({ margin: 0, fontSize: theme.typography.h6.fontSize, fontWeight: theme.typography.fontWeightMedium }),
  cardSub: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  cardBody: css({ padding: theme.spacing(2.5) }),
  viewRow: css({ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: theme.spacing(2) }),
  viewIdentity: css({ minWidth: 0 }),
  viewName: css({ color: theme.colors.text.primary, fontWeight: theme.typography.fontWeightMedium }),
  viewEmail: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  editForm: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1.5) }),
  field: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5), maxWidth: 420 }),
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
  editActions: css({ display: 'flex', gap: theme.spacing(1) }),
  lockedField: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(1.5),
    maxWidth: 420,
    padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`,
    borderRadius: theme.shape.radius.sm,
    border: `1px solid ${theme.colors.border.weak}`,
    background: theme.colors.background.secondary,
    color: theme.colors.text.secondary,
  }),
  lockedValue: css({ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }),
  lockIcon: css({ flexShrink: 0, color: theme.colors.text.disabled }),
  fieldHint: css({ color: theme.colors.text.disabled, fontSize: theme.typography.sm.fontSize }),
  editMedia: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  editMediaActions: css({ display: 'flex', gap: theme.spacing(1), flexWrap: 'wrap' }),
  statRow: css({
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: theme.spacing(2),
    [theme.breakpoints.down('sm')]: { gridTemplateColumns: '1fr' },
  }),
  stat: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
    padding: theme.spacing(2),
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
    textAlign: 'left',
  }),
  statStatic: css({}),
  statValue: css({
    fontSize: theme.typography.xxl.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  statLabel: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  rowBetween: css({ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: theme.spacing(2) }),
  rowTitle: css({ color: theme.colors.text.primary, fontWeight: theme.typography.fontWeightMedium }),
  rowHint: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize, marginTop: theme.spacing(0.5) }),
  badges: css({ display: 'flex', gap: theme.spacing(1), marginTop: theme.spacing(1.5) }),
});
