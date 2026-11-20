import { cx } from '@emotion/css';
import { useVirtualizer } from '@tanstack/react-virtual';
import { useCallback, useEffect } from 'react';
import { Icon } from '../Icon';
import { useStyles2 } from '../../theme/ThemeContext';
import { isNewGroup } from './filter';
import { MENU_MAX_HEIGHT, SHEET_ROW_GAP, SHEET_ROW_HEIGHT, getComboboxStyles } from './styles';
import { itemToString, type ComboboxOption } from './types';

const OVERSCAN = 4;

export const ROW_METRICS = {
  popover: { row: 36, withDescription: 52, gap: 0 },
  sheet: {
    row: SHEET_ROW_HEIGHT,
    withDescription: SHEET_ROW_HEIGHT + 16,
    gap: SHEET_ROW_GAP,
  },
} as const;

export type ComboboxDensity = keyof typeof ROW_METRICS;

export interface ComboboxListProps<T extends string | number> {
  options: ComboboxOption<T>[];
  highlightedIndex: number | null;
  selectedValue: T | undefined;
  loading?: boolean;
  error?: boolean;
  noOptionsMessage?: string;
  showFocusRing: boolean;
  scrollRef: React.RefObject<HTMLDivElement | null>;
  getItemProps: (options: { item: ComboboxOption<T>; index: number }) => Record<string, unknown>;

  density?: ComboboxDensity;
}

export function ComboboxList<T extends string | number>({
  options,
  highlightedIndex,
  selectedValue,
  loading,
  error,
  noOptionsMessage = 'No options found.',
  showFocusRing,
  scrollRef,
  getItemProps,
  density = 'popover',
}: ComboboxListProps<T>) {
  const styles = useStyles2(getComboboxStyles, density);
  const metrics = ROW_METRICS[density];

  const estimateSize = useCallback(
    (index: number) => {
      const option = options[index];
      if (!option) return metrics.row;
      const grouped = isNewGroup(option, options[index - 1]);
      const base = option.description ? metrics.withDescription : metrics.row;

      return base + metrics.gap + (grouped && option.group ? metrics.row : 0);
    },
    [options, metrics],
  );

  const virtualizer = useVirtualizer({
    count: options.length,
    getScrollElement: () => scrollRef.current,
    estimateSize,
    overscan: OVERSCAN,

    initialRect: {
      width: 0,
      height: Math.min(options.length * metrics.row, MENU_MAX_HEIGHT),
    },
  });

  useEffect(() => {
    if (highlightedIndex === null || highlightedIndex < 0) return;
    virtualizer.scrollToIndex(highlightedIndex, { align: 'auto' });
  }, [highlightedIndex, virtualizer]);

  if (loading) {
    return (
      <div className={styles.message}>
        <Icon name="spinner" size="sm" /> Loading options…
      </div>
    );
  }

  if (error) {
    return (
      <div className={styles.message}>
        <Icon name="alert-triangle" size="sm" /> An error occurred while loading options.
      </div>
    );
  }

  if (options.length === 0) {
    return <div className={styles.message}>{noOptionsMessage}</div>;
  }

  return (
    <div style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
      {virtualizer.getVirtualItems().map((row) => {
        const option = options[row.index];
        if (!option) return null;

        const startingNewGroup = isNewGroup(option, options[row.index - 1]);
        const isHighlighted = highlightedIndex === row.index;
        const isSelected = option.value === selectedValue;

        return (
          <div
            key={row.key}
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              width: '100%',
              transform: `translateY(${row.start}px)`,
            }}
          >
            {startingNewGroup && option.group ? (
              <div className={styles.groupHeader} role="presentation">
                {option.group}
              </div>
            ) : null}

            <div
              {...getItemProps({ item: option, index: row.index })}
              className={cx(
                styles.option,
                isHighlighted && styles.optionFocused,
                isHighlighted && showFocusRing && styles.optionFocusRing,
                isSelected && styles.optionSelected,
                option.infoOption && styles.optionInfo,
              )}
            >
              {option.icon ? <Icon name={option.icon} size="sm" /> : null}
              <span className={styles.optionBody}>
                <span className={styles.optionLabel}>{itemToString(option)}</span>
                {option.description ? (
                  <span className={styles.optionDescription}>{option.description}</span>
                ) : null}
              </span>
              {isSelected ? <Icon name="check" size="sm" /> : null}
            </div>
          </div>
        );
      })}
    </div>
  );
}

export { MENU_MAX_HEIGHT };
