import { Tag, type IconName } from '@vyzorix/ui';

export interface StatusBadgeProps {

  label: string;

  icon?: IconName;
}

export function StatusBadge({ label, icon }: StatusBadgeProps) {
  return <Tag name={label} icon={icon} />;
}
