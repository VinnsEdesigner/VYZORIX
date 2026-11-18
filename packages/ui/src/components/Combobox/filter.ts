import { itemToString, type ComboboxOption } from './types';

export function filterOptions<T extends string | number>(
  options: ComboboxOption<T>[],
  needle: string,
): ComboboxOption<T>[] {
  const query = needle.trim().toLowerCase();
  if (query === '') {
    return options;
  }

  return options.filter((option) => {
    const label = itemToString(option).toLowerCase();
    const description = (option.description ?? '').toLowerCase();
    return label.includes(query) || description.includes(query);
  });
}

export function isNewGroup<T extends string | number>(
  option: ComboboxOption<T>,
  previous?: ComboboxOption<T>,
): boolean {
  if (!option.group) {
    return Boolean(previous?.group);
  }
  if (!previous) {
    return true;
  }
  return previous.group !== option.group;
}
