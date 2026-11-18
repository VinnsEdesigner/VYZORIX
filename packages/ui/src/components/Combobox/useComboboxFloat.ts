import { autoPlacement, autoUpdate, size, useFloating } from '@floating-ui/react';
import { useEffect, useRef, useState, type CSSProperties } from 'react';

const VIEWPORT_PADDING = 16;

const MIN_HEIGHT = 120;

export interface ComboboxFloat {
  inputRef: React.RefObject<HTMLInputElement | null>;
  floatingRef: React.RefObject<HTMLDivElement | null>;
  scrollRef: React.RefObject<HTMLDivElement | null>;
  menuStyles: CSSProperties;
  maxHeight: number;
}

export function useComboboxFloat(isOpen: boolean): ComboboxFloat {
  const inputRef = useRef<HTMLInputElement>(null);
  const floatingRef = useRef<HTMLDivElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const [maxHeight, setMaxHeight] = useState<number>(MIN_HEIGHT);

  const { floatingStyles, refs, update } = useFloating({
    strategy: 'fixed',
    placement: 'bottom-start',
    open: isOpen,
    middleware: [
      autoPlacement({
        allowedPlacements: ['bottom-start', 'bottom-end', 'top-start', 'top-end'],
        crossAxis: true,
      }),
      size({
        apply({ availableHeight }) {
          const next = Math.max(availableHeight - VIEWPORT_PADDING, MIN_HEIGHT);
          setMaxHeight((current) => (current === next ? current : next));
        },
      }),
    ],
  });

  useEffect(() => {
    refs.setReference(inputRef.current);
    refs.setFloating(floatingRef.current);
  }, [refs, isOpen]);

  useEffect(() => {
    if (!isOpen || !inputRef.current || !floatingRef.current) {
      return;
    }
    return autoUpdate(inputRef.current, floatingRef.current, update);
  }, [isOpen, update]);

  return {
    inputRef,
    floatingRef,
    scrollRef,
    menuStyles: { ...floatingStyles, minWidth: inputRef.current?.offsetWidth },
    maxHeight,
  };
}
