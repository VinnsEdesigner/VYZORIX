import { type IconName } from '../Icon';

export interface ComboboxOption<T extends string | number = string> {
  value: T;
  label?: string;
  description?: string;
  group?: string;
  icon?: IconName;
  infoOption?: boolean;
}

export function itemToString<T extends string | number>(item?: ComboboxOption<T> | null): string {
  if (item == null) {
    return '';
  }
  return item.label ?? String(item.value);
}
