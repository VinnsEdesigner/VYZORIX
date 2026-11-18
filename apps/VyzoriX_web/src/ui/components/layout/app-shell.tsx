import { css, cx } from '@emotion/css';
import { useEffect, useState } from 'react';
import { Link, useLocation } from '@tanstack/react-router';
import {
  BrandLogo,
  Icon,
  IconButton,
  darken,
  getFocusStyles,
  useBranding,
  useStyles2,
  type VyzorixTheme,
} from '@vyzorix/ui';
import { useVyzorTranslation } from '@/hooks/i18n';
import { appNav, pageTabs } from './app-nav';
import {
  backTargetFor,
  humanisePath,
  isNavItemActive,
  navTrailLabels,
  resolveNavTrail,
  splitUrl,
  stripTrailingSlash,
  type NavSection,
} from './nav-model';
import { AccountMenu } from './account-menu';

export interface AppShellProps {
  children: React.ReactNode;
  nav?: NavSection[];
  topBarActions?: React.ReactNode;
  className?: string;
}

const TOPBAR_HEIGHT = 52;
const APPBAR_HEIGHT = 44;
const TABBAR_HEIGHT = 40;

export function AppShell({ children, nav = appNav, topBarActions, className }: AppShellProps) {
  const styles = useStyles2(getStyles);
  const location = useLocation();
  const pathname = stripTrailingSlash(location.pathname);
  const [menuOpen, setMenuOpen] = useState(false);
  const branding = useBranding();
  const { t } = useVyzorTranslation();

  const trail = resolveNavTrail(nav, pathname);
  const tabs = pageTabs[pathname] ?? [];
  const hasTabBar = tabs.length > 1;
  const childIsTab = Boolean(trail.child && tabs.some((tab) => tab.id === trail.child?.id));
  const crumbs = navTrailLabels(trail, childIsTab);
  const activeTabId = new URLSearchParams(location.search).get('tab');
  const activeTab = activeTabId ?? tabs[0]?.id;
  const activeTabLabel = tabs.find((tab) => tab.id === activeTab)?.text;
  const backTarget = backTargetFor(pathname);

  const activeGroupId = trail.item?.children ? trail.item.id : undefined;
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(() => new Set());

  const toggleGroup = (id: string) => {
    setExpandedGroups((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  useEffect(() => {
    setMenuOpen(false);
  }, [pathname]);

  useEffect(() => {
    if (!activeGroupId) return;
    setExpandedGroups((current) => {
      if (current.has(activeGroupId)) return current;
      const next = new Set(current);
      next.add(activeGroupId);
      return next;
    });
  }, [activeGroupId]);

  useEffect(() => {
    if (!menuOpen) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setMenuOpen(false);
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [menuOpen]);

  return (
    <div className={cx(styles.chrome, menuOpen && styles.chromeMenuOpen, className)}>
      <a
        className={styles.skipLink}
        href="#pageContent"
        onClick={(event) => {
          event.preventDefault();
          document.getElementById('pageContent')?.focus();
        }}
      >
        Skip to main content
      </a>

      <header className={styles.topBar}>
        <Link to="/" className={styles.brand} aria-label={branding.appTitle}>
          <BrandLogo size="md" alt="" />
        </Link>
        <IconButton
          name="menu"
          aria-label={menuOpen ? t('shell.closeMenu') : t('shell.toggleMenu')}
          aria-expanded={menuOpen}
          aria-controls="app-navigation"
          fill="text"
          variant="secondary"
          size="lg"
          className={styles.menuButton}
          onClick={() => setMenuOpen((open) => !open)}
        />
        <div className={styles.search}>
          <Icon name="search" size="sm" />
          <input className={styles.searchInput} placeholder={t('shell.search')} aria-label={t('shell.search')} />
          <kbd className={styles.kbd}>⌘K</kbd>
        </div>
        <div className={styles.spacer} />
        {topBarActions}
        <AccountMenu />
      </header>

      <div className={styles.appBar}>
        {backTarget ? (
          <Link
            to={backTarget.to}
            search={backTarget.tab ? { tab: backTarget.tab } : undefined}
            className={styles.backNav}
            aria-label={t('shell.back')}
          >
            <Icon name="arrow-left" size="xl" />
          </Link>
        ) : (
          <nav className={styles.crumbs} aria-label={t('shell.breadcrumb')}>
            {crumbs.length > 0 ? (
              crumbs.map((label, index) => (
                <span key={`${label}-${index}`} className={styles.crumbGroup}>
                  {index > 0 ? <span className={styles.crumbSep}>›</span> : null}
                  <b className={index === crumbs.length - 1 ? styles.crumbCurrent : styles.crumbAncestor}>{label}</b>
                </span>
              ))
            ) : (
              <b className={styles.crumbCurrent}>{humanisePath(pathname)}</b>
            )}
            {hasTabBar && activeTabLabel ? (
              <>
                <span className={styles.crumbSep}>›</span>
                <span className={styles.crumbTab}>{activeTabLabel}</span>
              </>
            ) : null}
          </nav>
        )}
        <div className={styles.spacer} />
      </div>

      <nav className={styles.tabBar} aria-label={t('shell.pageTabs')}>
        {tabs.map((tab) => (
          <Link
            key={tab.id}
            to={pathname}
            search={{ tab: tab.id }}
            className={cx(styles.tab, tab.id === activeTab && styles.tabActive)}
            aria-current={tab.id === activeTab ? 'page' : undefined}
          >
            {t(`tab.${tab.id}`, { defaultValue: tab.text })}
          </Link>
        ))}
      </nav>

      <div className={styles.body}>
        {menuOpen ? (
          <div className={styles.scrim} onClick={() => setMenuOpen(false)} aria-hidden="true" />
        ) : null}

        <nav
          id="app-navigation"
          className={cx(styles.megaNav, menuOpen && styles.megaNavOpen)}
          aria-label={t('shell.mainNav')}
          inert={!menuOpen}
        >
          <div className={styles.megaHead}>
            <IconButton
              name="x"
              aria-label="Close navigation"
              fill="text"
              variant="secondary"
              size="sm"
              onClick={() => setMenuOpen(false)}
            />
          </div>
          <div className={styles.megaScroll}>
            {nav.map((section) => (
              <div key={section.id} className={styles.navSection}>
                <div className={styles.navSectionLabel}>{t(`nav.${section.id}`, { defaultValue: section.label })}</div>
                {section.items.map((item) => {
                  const itemActive = isNavItemActive(item, pathname);
                  const isGroup = Boolean(item.children?.length);

                  if (!isGroup) {
                    return (
                      <Link
                        key={item.id}
                        to={stripTrailingSlash(item.url)}
                        onClick={() => setMenuOpen(false)}
                        className={cx(styles.navItem, itemActive && styles.navItemActive)}
                        aria-current={itemActive ? 'page' : undefined}
                      >
                        {item.icon ? (
                          <span className={styles.navIcon}>
                            <Icon name={item.icon} size="md" />
                          </span>
                        ) : null}
                        <span className={styles.navLabel}>{t(`nav.${item.id}`, { defaultValue: item.text })}</span>
                      </Link>
                    );
                  }

                  const collapsed = !expandedGroups.has(item.id);

                  return (
                    <div key={item.id} className={styles.navGroup}>
                      <button
                        type="button"
                        className={cx(styles.navGroupHead, itemActive && styles.navItemActive)}
                        aria-expanded={!collapsed}
                        aria-controls={`nav-group-${item.id}`}
                        onClick={() => toggleGroup(item.id)}
                      >
                        {item.icon ? (
                          <span className={styles.navIcon}>
                            <Icon name={item.icon} size="md" />
                          </span>
                        ) : null}
                        <span className={styles.navLabel}>{t(`nav.${item.id}`, { defaultValue: item.text })}</span>
                        <Icon
                          name="chevron-down"
                          size="sm"
                          className={cx(styles.navCaret, collapsed && styles.navCaretCollapsed)}
                        />
                      </button>
                      {collapsed ? null : (
                        <div id={`nav-group-${item.id}`} className={styles.navGroupBody}>
                          {item.children?.map((child) => {
                            const target = splitUrl(child.url);
                            const childActive = target.path === pathname && target.tab === (activeTab ?? null);
                            return (
                              <Link
                                key={child.id}
                                to={target.path}
                                search={target.tab ? { tab: target.tab } : undefined}
                                onClick={() => setMenuOpen(false)}
                                className={cx(styles.navChild, childActive && styles.navChildActive)}
                                aria-current={childActive ? 'page' : undefined}
                              >
                                {t(`nav.${child.id}`, { defaultValue: child.text })}
                              </Link>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            ))}
          </div>
        </nav>

        <main id="pageContent" tabIndex={-1} className={styles.content}>
          {children}
        </main>
      </div>
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  chrome: css({
    display: 'flex',
    flexDirection: 'column',
    height: '100vh',
    overflow: 'hidden',
    background: theme.colors.background.canvas,
    color: theme.colors.text.primary,
  }),
  chromeMenuOpen: css({}),
  skipLink: css({
    position: 'fixed',
    top: -1000,
    left: theme.spacing(1),
    '&:focus': { top: theme.spacing(1), zIndex: theme.zIndex.portal },
  }),
  topBar: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    height: TOPBAR_HEIGHT,
    flexShrink: 0,
    padding: theme.spacing(0, 1.5),
    background: theme.colors.background.primary,
    borderBottom: `1px solid ${theme.colors.border.weak}`,
    zIndex: 3,
  }),
  menuButton: css({
    width: 40,
    height: 40,
    padding: 0,
    flexShrink: 0,
    '& svg': { width: 26, height: 26 },
  }),
  brand: css({
    display: 'inline-flex',
    alignItems: 'center',
    flexShrink: 0,
    textDecoration: 'none',
    borderRadius: theme.shape.radius.sm,
    '& img': { width: 46, height: 'auto' },
    '&:focus-visible': getFocusStyles(theme),
  }),
  search: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    height: 32,
    width: 'min(520px, 46vw)',
    padding: theme.spacing(0, 1.25),
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    color: theme.colors.text.disabled,
    '&:focus-within': { borderColor: theme.colors.primary.border },
  }),
  searchInput: css({
    flex: 1,
    minWidth: 0,
    background: 'transparent',
    border: 0,
    outline: 'none',
    color: theme.colors.text.primary,
    fontFamily: 'inherit',
    fontSize: theme.typography.sm.fontSize,
    '&::placeholder': { color: theme.colors.text.disabled },
  }),
  kbd: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: 10,
    border: `1px solid ${theme.colors.border.medium}`,
    borderRadius: 3,
    padding: theme.spacing(0, 0.5),
  }),
  spacer: css({ flex: 1 }),
  appBar: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(2),
    height: APPBAR_HEIGHT,
    flexShrink: 0,
    padding: theme.spacing(0, 1.5),
    background: theme.colors.background.primary,
    borderBottom: `1px solid ${theme.colors.border.weak}`,
    zIndex: 2,
  }),
  backNav: css({
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 32,
    height: 32,
    marginLeft: -theme.spacing(0.5),
    borderRadius: theme.shape.radius.default,
    color: theme.colors.text.secondary,
    textDecoration: 'none',
    '&:hover': { background: theme.colors.action.hover, color: theme.colors.text.primary },
    '&:focus-visible': getFocusStyles(theme),
  }),
  crumbs: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.75),
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.secondary,
    whiteSpace: 'nowrap',
  }),
  crumbGroup: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(0.75) }),
  crumbSep: css({ color: theme.colors.text.disabled }),
  crumbAncestor: css({ color: theme.colors.text.secondary, fontWeight: theme.typography.fontWeightRegular }),
  crumbCurrent: css({ color: theme.colors.text.primary, fontWeight: theme.typography.fontWeightMedium }),
  crumbTab: css({ color: theme.colors.text.secondary }),
  tabBar: css({
    display: 'flex',
    alignItems: 'stretch',
    gap: 2,
    height: TABBAR_HEIGHT,
    flexShrink: 0,
    padding: theme.spacing(0, 1.5),
    background: theme.colors.background.canvas,
    borderBottom: `1px solid ${theme.colors.border.weak}`,
    overflowX: 'auto',
    zIndex: 1,
  }),
  tab: css({
    display: 'inline-flex',
    alignItems: 'center',
    padding: theme.spacing(0, 1.5),
    border: 0,
    borderBottom: '2px solid transparent',
    background: 'transparent',
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
    textDecoration: 'none',
    whiteSpace: 'nowrap',
    '&:hover': { color: theme.colors.text.primary },
  }),
  tabActive: css({
    color: theme.colors.text.primary,
    borderBottomColor: theme.colors.primary.text,
  }),
  body: css({ display: 'flex', flex: 1, minHeight: 0 }),
  scrim: css({
    position: 'fixed',
    inset: `${TOPBAR_HEIGHT + APPBAR_HEIGHT + TABBAR_HEIGHT}px 0 0 0`,
    background: 'rgba(0, 0, 0, 0.45)',
    zIndex: theme.zIndex.sidemenu - 1,
  }),
  megaNav: css({
    position: 'fixed',
    top: TOPBAR_HEIGHT + APPBAR_HEIGHT + TABBAR_HEIGHT,
    left: 0,
    bottom: 0,
    width: 0,
    maxWidth: '100vw',
    height: 'auto',
    overflow: 'hidden',
    display: 'flex',
    flexDirection: 'column',
    background: theme.isDark ? darken(theme.colors.background.canvas, 0.45) : theme.colors.background.primary,
    zIndex: theme.zIndex.sidemenu,
    opacity: 0,
    pointerEvents: 'none',
    transition: 'width .2s ease-in-out, opacity .2s ease-in-out',
  }),
  megaNavOpen: css({
    width: 'var(--vyz-nav-width, 100%)',
    opacity: 1,
    pointerEvents: 'auto',
    [theme.breakpoints.down('sm')]: { width: '100%' },
  }),
  megaHead: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'flex-end',
    height: 40,
    flexShrink: 0,
    padding: theme.spacing(0, 1.5),
    borderBottom: `1px solid ${theme.colors.border.weak}`,
  }),
  megaScroll: css({ flex: 1, overflowY: 'auto', overflowX: 'hidden', padding: theme.spacing(0.5, 1, 2) }),
  navSection: css({ padding: theme.spacing(0.5, 0) }),
  navSectionLabel: css({
    fontSize: 12,
    textTransform: 'uppercase',
    letterSpacing: '0.08em',
    color: theme.colors.text.disabled,
    padding: theme.spacing(0, 1.25),
    margin: `${theme.spacing(1)} 0 ${theme.spacing(0.5)}`,
  }),
  navItem: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    width: '100%',
    height: 32,
    padding: theme.spacing(0, 1.25),
    marginBottom: 2,
    borderRadius: theme.shape.radius.default,
    color: theme.colors.text.secondary,
    textDecoration: 'none',
    fontSize: theme.typography.base.fontSize,
    '&:hover': { background: theme.colors.action.hover, color: theme.colors.text.primary },
    '&:focus-visible': getFocusStyles(theme),
  }),
  navItemActive: css({
    background: theme.colors.action.selected,
    color: theme.colors.text.primary,
    fontWeight: theme.typography.fontWeightMedium,
    '& span:first-of-type': { color: theme.colors.primary.text },
  }),
  navIcon: css({
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 24,
    flexShrink: 0,
    color: theme.colors.text.disabled,
  }),
  navLabel: css({ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }),
  navGroup: css({ marginBottom: 2 }),
  navGroupHead: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    width: '100%',
    height: 34,
    padding: theme.spacing(0, 1.25),
    border: 0,
    borderRadius: theme.shape.radius.default,
    background: 'transparent',
    color: theme.colors.text.primary,
    fontFamily: 'inherit',
    fontSize: theme.typography.base.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    textAlign: 'left',
    cursor: 'pointer',
    '&:hover': { background: theme.colors.action.hover },
    '&:focus-visible': getFocusStyles(theme),
  }),
  navCaret: css({
    marginLeft: 'auto',
    color: theme.colors.text.disabled,
    transition: 'transform .12s ease',
  }),
  navCaretCollapsed: css({ transform: 'rotate(-90deg)' }),
  navGroupBody: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
    paddingLeft: theme.spacing(2.5),
    paddingTop: theme.spacing(0.5),
  }),
  navChild: css({
    display: 'flex',
    alignItems: 'center',
    height: 32,
    padding: theme.spacing(0, 1.25),
    borderRadius: theme.shape.radius.default,
    color: theme.colors.text.secondary,
    textDecoration: 'none',
    fontSize: theme.typography.sm.fontSize,
    '&:hover': { background: theme.colors.action.hover, color: theme.colors.text.primary },
  }),
  navChildActive: css({ color: theme.colors.text.primary }),
  content: css({
    flex: 1,
    overflowY: 'auto',
    minWidth: 0,
    padding: 'calc(24px * var(--vyz-density-scale, 1)) calc(32px * var(--vyz-density-scale, 1))',
    '&:focus': { outline: 'none' },
    [theme.breakpoints.down('md')]: {
      padding: 'calc(16px * var(--vyz-density-scale, 1)) calc(12px * var(--vyz-density-scale, 1))',
    },
  }),
});
