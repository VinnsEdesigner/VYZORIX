import { css, cx, keyframes } from '@emotion/css';
import { useCallback, useEffect, useId, useRef, useState } from 'react';
import { Link, useNavigate } from '@tanstack/react-router';
import {
  Divider,
  Icon,
  Portal,
  SHEET_HEIGHT,
  SHEET_MIN_HEIGHT,
  getFocusStyles,
  useStyles2,
  type IconName,
  type VyzorixTheme,
} from '@vyzorix/ui';
import { useCurrentUser } from '@/hooks/_shared';
import { useLogout } from '@/hooks/auth';
import { useVyzorTranslation } from '@/hooks/i18n';
import { useUnreadNotifications } from '@/hooks/settings';

interface AccountMenuItem {
  id: string;
  label: string;
  icon: IconName;
  badge?: number;

  to?: string;

  search?: Record<string, string>;

  danger?: boolean;
  onSelect?: () => void;
}

export function initialsFor(name: string | null, email: string | null): string {
  const source = name?.trim() || email?.split('@')[0]?.trim() || '';
  if (!source) return '?';
  const words = source.split(/[\s._-]+/).filter(Boolean);
  const letters = words.slice(0, 2).map((word) => word[0]);
  return letters.join('').toUpperCase() || '?';
}

const sheetUp = keyframes`
  from { transform: translateY(101%); }
  to   { transform: translateY(0); }
`;
const scrimIn = keyframes`
  from { opacity: 0; }
  to   { opacity: 1; }
`;

export function AccountMenu({ className }: { className?: string }) {
  const styles = useStyles2(getStyles);
  const user = useCurrentUser();
  const logout = useLogout();
  const navigate = useNavigate();
  const { t } = useVyzorTranslation();
  const unread = useUnreadNotifications();
  const menuId = useId();

  const [open, setOpen] = useState(false);
  const wrapperRef = useRef<HTMLDivElement>(null);
  const sheetRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const itemRefs = useRef<Array<HTMLElement | null>>([]);

  const close = (refocus = false) => {
    setOpen(false);
    if (refocus) triggerRef.current?.focus();
  };

  useEffect(() => {
    if (!open) return;
    const onDown = (event: MouseEvent) => {
      const target = event.target as Node;

      if (wrapperRef.current?.contains(target) || sheetRef.current?.contains(target)) return;
      setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') close(true);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const attachSheet = useCallback((node: HTMLDivElement | null) => {
    sheetRef.current = node;
    if (node) itemRefs.current[0]?.focus();
  }, []);

  const onItemKeyDown = (index: number) => (event: React.KeyboardEvent) => {
    const count = items.length + 1;
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      itemRefs.current[(index + 1) % count]?.focus();
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      itemRefs.current[(index - 1 + count) % count]?.focus();
    } else if (event.key === 'Home') {
      event.preventDefault();
      itemRefs.current[0]?.focus();
    } else if (event.key === 'End') {
      event.preventDefault();
      itemRefs.current[count - 1]?.focus();
    }
  };

  const signOut = () => {
    close();
    logout.mutate(
      {},
      {
        onSettled: () => {
          void navigate({ to: '/' });
        },
      },
    );
  };

  const items: AccountMenuItem[] = [
    { id: 'profile', label: t('account.profile'), icon: 'user', to: '/profile' },
    { id: 'preferences', label: t('account.preferences'), icon: 'settings', to: '/admin/default-preferences' },
    { id: 'settings', label: t('account.organisationSettings'), icon: 'sliders', to: '/settings' },
    { id: 'api-keys', label: t('account.apiKeys'), icon: 'key', to: '/settings', search: { tab: 'api-keys' } },
    { id: 'notifications', label: t('account.notificationHistory'), icon: 'bell', to: '/notifications', badge: unread },
  ];

  const displayName = user.name ?? user.email ?? 'Account';

  return (
    <div ref={wrapperRef} className={cx(styles.wrapper, className)}>
      <button
        ref={triggerRef}
        type="button"
        className={styles.trigger}
        aria-label={`${displayName} — account`}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        onClick={() => setOpen((value) => !value)}
      >
        {user.avatarUrl ? (
          <img className={styles.triggerImg} src={user.avatarUrl} alt="" />
        ) : (
          <span aria-hidden="true">{initialsFor(user.name, user.email)}</span>
        )}
      </button>

      {open ? (
        <Portal>
          <div className={styles.scrim} onClick={() => close(true)} aria-hidden="true" />
          <div id={menuId} ref={attachSheet} role="menu" aria-label="Account" className={styles.sheet}>
            <div className={styles.grab} aria-hidden="true" />

            <div className={styles.head}>
              <span className={cx(styles.avatar, styles.avatarLarge)} aria-hidden="true">
                {user.avatarUrl ? (
                  <img className={styles.headImg} src={user.avatarUrl} alt="" />
                ) : (
                  initialsFor(user.name, user.email)
                )}
              </span>
              <span className={styles.identity}>
                <span className={styles.name}>{user.name ?? 'Signed in'}</span>
                {user.email ? <span className={styles.email}>{user.email}</span> : null}
              </span>
            </div>

            <Divider spacing={0.5} />

            {items.map((item, index) =>
              item.to ? (
                <Link
                  key={item.id}
                  ref={(node: HTMLAnchorElement | null) => {
                    itemRefs.current[index] = node;
                  }}
                  role="menuitem"
                  to={item.to}
                  search={item.search}
                  className={cx(styles.item, item.danger && styles.itemDanger)}
                  onClick={() => close()}
                  onKeyDown={onItemKeyDown(index)}
                >
                  <Icon name={item.icon} size="lg" />
                  {item.label}
                  {item.badge && item.badge > 0 ? (
                    <span className={styles.badge} aria-label={t('account.unreadCount', { count: item.badge })}>
                      {item.badge}
                    </span>
                  ) : null}
                </Link>
              ) : (
                <button
                  key={item.id}
                  ref={(node) => {
                    itemRefs.current[index] = node;
                  }}
                  type="button"
                  role="menuitem"
                  className={cx(styles.item, item.danger && styles.itemDanger)}
                  onClick={item.onSelect}
                  onKeyDown={onItemKeyDown(index)}
                >
                  <Icon name={item.icon} size="lg" />
                  {item.label}
                </button>
              ),
            )}

            <Divider spacing={0.5} />

            <button
              ref={(node) => {
                itemRefs.current[items.length] = node;
              }}
              type="button"
              role="menuitem"
              className={cx(styles.item, styles.itemDanger)}
              onClick={signOut}
              onKeyDown={onItemKeyDown(items.length)}
            >
              <Icon name="log-out" size="lg" />
              {logout.isPending ? t('account.signingOut') : t('account.signOut')}
            </button>
          </div>
        </Portal>
      ) : null}
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  wrapper: css({ position: 'relative', display: 'inline-flex', flexShrink: 0 }),
  trigger: css({
    display: 'grid',
    placeItems: 'center',
    width: 32,
    height: 32,
    flexShrink: 0,
    padding: 0,
    overflow: 'hidden',
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.background.secondary,
    color: theme.colors.text.primary,
    fontFamily: 'inherit',
    fontSize: 11,
    fontWeight: theme.typography.fontWeightMedium,
    letterSpacing: '0.02em',
    cursor: 'pointer',
    '&:hover': { borderColor: theme.colors.border.medium, background: theme.colors.action.hover },
    '&:focus-visible': getFocusStyles(theme),
  }),
  triggerImg: css({ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }),
  scrim: css({
    position: 'fixed',
    inset: 0,
    zIndex: theme.zIndex.modalBackdrop,
    background: 'rgba(0, 0, 0, 0.5)',

    [theme.transitions.handleMotion('no-preference')]: {
      animation: `${scrimIn} ${theme.transitions.duration.enteringScreen}ms ${theme.transitions.easing.easeOut}`,
    },
  }),

  sheet: css({
    position: 'fixed',
    left: 0,
    right: 0,
    bottom: 0,
    zIndex: theme.zIndex.modal,
    display: 'flex',
    flexDirection: 'column',

    height: SHEET_HEIGHT,
    maxHeight: '85dvh',
    minHeight: SHEET_MIN_HEIGHT,
    padding: theme.spacing(0, 0.75, 0.75),
    paddingBottom: `calc(${theme.spacing(0.75)} + env(safe-area-inset-bottom, 0px))`,
    overflowY: 'auto',
    background: theme.colors.background.primary,
    borderTop: `1px solid ${theme.colors.border.medium}`,
    borderRadius: '16px 16px 0 0',
    boxShadow: theme.shadows.z3,
    [theme.transitions.handleMotion('no-preference')]: {
      animation: `${sheetUp} ${theme.transitions.duration.short}ms ${theme.transitions.easing.easeOut}`,
    },
  }),
  grab: css({
    width: 36,
    height: 4,
    flexShrink: 0,
    margin: `${theme.spacing(1)} auto ${theme.spacing(0.5)}`,
    borderRadius: 2,
    background: theme.colors.border.medium,
  }),
  head: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    padding: theme.spacing(1, 1.5),
  }),
  avatar: css({
    display: 'grid',
    placeItems: 'center',
    flexShrink: 0,
    overflow: 'hidden',
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.background.secondary,
    color: theme.colors.text.primary,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  avatarLarge: css({ width: 40, height: 40, fontSize: 14 }),
  headImg: css({ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }),
  identity: css({ display: 'flex', flexDirection: 'column', minWidth: 0 }),
  name: css({
    color: theme.colors.text.primary,
    fontSize: theme.typography.base.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
  email: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),

  item: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.75),
    width: '100%',
    minHeight: 44,
    padding: theme.spacing(1, 1.5),
    border: 0,
    borderRadius: theme.shape.radius.default,
    background: 'transparent',
    color: theme.colors.text.primary,
    fontFamily: 'inherit',
    fontSize: theme.typography.base.fontSize,
    textAlign: 'left',
    textDecoration: 'none',
    cursor: 'pointer',
    '&:hover': { background: theme.colors.action.hover },
    '&:focus-visible': getFocusStyles(theme, true),
    '& svg': { color: 'currentColor' },
  }),
  badge: css({
    marginLeft: 'auto',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    minWidth: 20,
    height: 20,
    padding: `0 ${theme.spacing(0.5)}`,
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.error.main,
    color: theme.colors.error.contrastText,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: 1,
  }),
  itemDanger: css({
    color: theme.colors.error.text,
    '& svg': { color: 'currentColor' },
  }),
});
