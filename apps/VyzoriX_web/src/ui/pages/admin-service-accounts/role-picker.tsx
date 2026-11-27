import { useMemo } from 'react';
import { css } from '@emotion/css';
import { Checkbox, Combobox, Spinner, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import type { FixedRoleGrant } from '@/hooks/service-accounts';
import { useServiceAccountRoleCatalog } from '@/hooks/service-accounts';

export type FixedRoleScope = 'organization' | 'all';

export interface RolePickerProps {
  basicRole: string;
  onBasicRoleChange: (role: string) => void;
  roles: FixedRoleGrant[];
  onRolesChange: (roles: FixedRoleGrant[]) => void;
}

export function RolePicker({ basicRole, onBasicRoleChange, roles, onRolesChange }: RolePickerProps) {
  const styles = useStyles2(getStyles);
  const catalog = useServiceAccountRoleCatalog();

  const groups = useMemo(() => {
    const map = new Map<string, Array<{ id: string; access: string }>>();
    for (const role of catalog.data?.roles ?? []) {
      if (!role.id || !role.group) continue;
      const list = map.get(role.group) ?? [];
      list.push({ id: role.id, access: role.access ?? '' });
      map.set(role.group, list);
    }
    return [...map.entries()].sort(([a], [b]) => a.localeCompare(b));
  }, [catalog.data]);

  if (catalog.isLoading) {
    return (
      <div className={styles.loading}>
        <Spinner /> Loading roles…
      </div>
    );
  }

  const has = (role: string, scope: FixedRoleScope) =>
    roles.some((grant) => grant.role === role && grant.scope === scope);

  const toggle = (role: string, scope: FixedRoleScope, checked: boolean) => {
    const without = roles.filter((grant) => !(grant.role === role && grant.scope === scope));
    onRolesChange(checked ? [...without, { role, scope }] : without);
  };

  return (
    <div className={styles.root}>
      <div className={styles.basicRow}>
        <label className={styles.basicLabel} htmlFor="sa-basic-role">
          Basic role
        </label>
        <div className={styles.basicSelect}>
          <Combobox
            id="sa-basic-role"
            aria-label="Basic role"
            value={basicRole || null}
            options={[
              { value: '', label: 'No basic role' },
              ...(catalog.data?.org_roles ?? []).map((role) => ({
                value: role,
                label: role.charAt(0).toUpperCase() + role.slice(1),
              })),
            ]}
            onChange={(next) => onBasicRoleChange(next ?? '')}
            isClearable={false}
          />
        </div>
      </div>

      <div className={styles.groups}>
        {groups.map(([group, entries]) => (
          <fieldset key={group} className={styles.group}>
            <legend className={styles.groupTitle}>{group}</legend>
            {entries.map((entry) => (
              <div key={entry.id} className={styles.roleRow}>
                <span className={styles.roleName}>
                  {entry.id.split(':')[1]?.replace(/^\w/, (c) => c.toUpperCase())} · {entry.access}
                </span>
                <div className={styles.scopeRow}>
                  <Checkbox
                    label="Organization"
                    checked={has(entry.id, 'organization')}
                    onChange={(event) => toggle(entry.id, 'organization', event.currentTarget.checked)}
                  />
                  <Checkbox
                    label="All"
                    checked={has(entry.id, 'all')}
                    onChange={(event) => toggle(entry.id, 'all', event.currentTarget.checked)}
                  />
                </div>
              </div>
            ))}
          </fieldset>
        ))}
      </div>
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  root: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  loading: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
  }),
  basicRow: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(2) }),
  basicLabel: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize, minWidth: 90 }),
  basicSelect: css({ maxWidth: 220 }),
  groups: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1),
    maxHeight: 280,
    overflowY: 'auto',
    paddingRight: theme.spacing(1),
  }),
  group: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`,
    margin: 0,
  }),
  groupTitle: css({
    padding: `0 ${theme.spacing(0.5)}`,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  roleRow: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    padding: `${theme.spacing(0.5)} 0`,
  }),
  roleName: css({ fontSize: theme.typography.sm.fontSize, color: theme.colors.text.primary }),
  scopeRow: css({ display: 'flex', gap: theme.spacing(1.5) }),
});
