import { Button, Menu, type MenuItem } from '@vyzorix/ui';

export interface RowAction {
  label: string;
  onClick: () => void;
  icon?: MenuItem['icon'];
  destructive?: boolean;
  disabled?: boolean;
}

export function RowActionsMenu({ label, actions }: { label: string; actions: RowAction[] }) {
  return (
    <Menu items={actions}>
      <Button variant="secondary" size="sm" fill="text" icon="more-horizontal" aria-label={`Actions for ${label}`} />
    </Menu>
  );
}
