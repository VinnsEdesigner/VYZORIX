import { cx } from '@emotion/css';
import { useCombobox } from 'downshift';
import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react';
import { Icon } from '../Icon';
import { Input } from '../Input';
import { OverlaySheet } from '../Overlay';
import { Portal } from '../Portal';
import { useOverlay } from '../Overlay/OverlayProvider';
import { useStyles2 } from '../../theme/ThemeContext';
import { ComboboxList } from './ComboboxList';
import { filterOptions } from './filter';
import { getComboboxStyles } from './styles';
import { itemToString, type ComboboxOption } from './types';
import { useComboboxFloat } from './useComboboxFloat';

function composeRefs(
  ...refs: Array<React.Ref<HTMLDivElement> | undefined>
): React.RefCallback<HTMLDivElement> {
  return (node) => {
    for (const ref of refs) {
      if (!ref) continue;
      if (typeof ref === 'function') {
        ref(node);
      } else {
        (ref as React.MutableRefObject<HTMLDivElement | null>).current = node;
      }
    }
  };
}

export interface ComboboxProps<T extends string | number> {
  options: ComboboxOption<T>[];
  value: T | null;
  onChange: (value: T | null) => void;
  isClearable?: boolean;
  searchable?: boolean;
  placeholder?: string;
  id?: string;
  disabled?: boolean;
  invalid?: boolean;

  'aria-label'?: string;
  'data-testid'?: string;
}

export function Combobox<T extends string | number>({
  options,
  value,
  onChange,
  isClearable,
  searchable = false,
  placeholder = 'Select…',
  id,
  disabled,
  invalid,
  'aria-label': ariaLabel,
  'data-testid': dataTestId,
}: ComboboxProps<T>) {
  const styles = useStyles2(getComboboxStyles);
  const reactId = useId();
  const inputId = id ?? `combobox-${reactId}`;

  const [showFocusRing, setShowFocusRing] = useState(false);

  const selectedItem = useMemo(
    () => options.find((option) => option.value === value) ?? null,
    [options, value],
  );

  const [isOpen, setIsOpen] = useState(false);
  const { presentation } = useOverlay();
  const isSheet = presentation === 'sheet';

  const { inputRef, floatingRef, scrollRef, menuStyles, maxHeight } = useComboboxFloat(isOpen && !isSheet);

  const sheetScrollRef = useRef<HTMLDivElement>(null);
  const activeScrollRef = isSheet ? sheetScrollRef : scrollRef;

  const [items, setItems] = useState(options);

  const {
    getInputProps,
    getMenuProps,
    getItemProps,
    highlightedIndex,
    setInputValue,
  } = useCombobox<ComboboxOption<T>>({
    items,
    itemToString,
    selectedItem,
    isItemDisabled: (item) => Boolean(item?.infoOption),
    onIsOpenChange: ({ isOpen: open }) => {
      setIsOpen(open);

      if (open) {
        setItems(options);
      }
    },
    onInputValueChange: ({ inputValue: next, type }) => {
      if (type === useCombobox.stateChangeTypes.InputChange) {
        setItems(filterOptions(options, next ?? ''));
      }
    },
    onSelectedItemChange: ({ selectedItem: next }) => {
      onChange(next ? next.value : null);
      setIsOpen(false);
    },
    onStateChange: ({ type }) => {
      setShowFocusRing(/KeyDown|InputChange/.test(String(type)));
    },

    stateReducer(state, { changes, type }) {
      const opening = state.isOpen === false && changes.isOpen === true;
      const closing = state.isOpen === true && changes.isOpen === false;

      if (opening && changes.inputValue === state.inputValue) {
        return { ...changes, inputValue: '' };
      }

      if (closing) {
        return {
          ...changes,
          inputValue: changes.selectedItem ? itemToString(changes.selectedItem) : '',
        };
      }

      if (type === useCombobox.stateChangeTypes.InputChange) {
        return { ...changes, highlightedIndex: 0 };
      }

      return changes;
    },
  });

  useEffect(() => {
    setItems(options);
  }, [options]);

  const handleClear = useCallback(() => {
    onChange(null);
    setInputValue('');
  }, [onChange]);

  const showClear = Boolean(isClearable) && value !== null && !disabled;

  const {
    ref: menuRef,
    suppressRefError: _suppress,
    ...menuProps
  } = getMenuProps({ suppressRefError: true }) as {
    ref?: React.Ref<HTMLDivElement>;
    suppressRefError?: boolean;
  } & Record<string, unknown>;

  return (
    <>
      <Input
        {...getInputProps({
          id: inputId,
          disabled,
          invalid,
          'aria-label': ariaLabel,
          'data-testid': dataTestId,
          ref: inputRef,

          readOnly: !searchable,
          className: searchable ? undefined : styles.noCaret,
          placeholder: isOpen ? placeholder : itemToString(selectedItem) || placeholder,
          onKeyDown: (event: React.KeyboardEvent<HTMLInputElement>) => {
            if (event.key === 'Escape' && isOpen) {
              event.stopPropagation();
            }
          },
        })}
        suffix={
          <span style={{ display: 'flex', alignItems: 'center', gap: 4, pointerEvents: 'auto' }}>
            {showClear ? (
              <Icon
                name="x"
                size="sm"
                className={styles.clear}
                role="button"
                tabIndex={0}
                aria-label="Clear value"
                onClick={handleClear}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault();
                    handleClear();
                  }
                }}
              />
            ) : null}
            <Icon name={isOpen ? 'search' : 'chevron-down'} size="sm" className={styles.chevron} />
          </span>
        }
      />

      {isSheet ? (
        <OverlaySheet
          open={isOpen}
          onClose={() => setIsOpen(false)}
          label={ariaLabel ?? placeholder}
          testId={dataTestId ? `${dataTestId}-sheet` : undefined}
        >
          <div {...menuProps} ref={menuRef} className={styles.sheetList}>
            <div ref={sheetScrollRef} className={styles.menuScroll}>
              <ComboboxList
                options={items}
                highlightedIndex={highlightedIndex}
                selectedValue={value ?? undefined}
                showFocusRing={showFocusRing}
                scrollRef={activeScrollRef}
                getItemProps={getItemProps}
                density="sheet"
              />
            </div>
          </div>
        </OverlaySheet>
      ) : (
        <Portal>
          <div
            {...menuProps}
            ref={composeRefs(menuRef, floatingRef)}
            className={cx(styles.menu, !isOpen && styles.menuClosed)}
            style={menuStyles}
          >
            {isOpen ? (
              <div ref={scrollRef} className={styles.menuScroll} style={{ maxHeight }}>
                <ComboboxList
                  options={items}
                  highlightedIndex={highlightedIndex}
                  selectedValue={value ?? undefined}
                  showFocusRing={showFocusRing}
                  scrollRef={activeScrollRef}
                  getItemProps={getItemProps}
                />
              </div>
            ) : null}
          </div>
        </Portal>
      )}
    </>
  );
}
