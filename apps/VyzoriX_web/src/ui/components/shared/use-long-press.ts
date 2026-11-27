import { useCallback, useEffect, useRef, useState } from 'react';

export const LONG_PRESS_MS = 500;
const MOVE_TOLERANCE_PX = 10;

export interface LongPressHandlers {
  onPointerDown: (event: React.PointerEvent) => void;
  onPointerUp: (event: React.PointerEvent) => void;
  onPointerMove: (event: React.PointerEvent) => void;
  onPointerCancel: (event: React.PointerEvent) => void;
  onContextMenu: (event: React.MouseEvent) => void;
}

export function useLongPress(onLongPress: () => void, enabled = true): LongPressHandlers {
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const origin = useRef<{ x: number; y: number } | null>(null);
  const fired = useRef(false);

  const clear = useCallback(() => {
    if (timer.current !== null) {
      clearTimeout(timer.current);
      timer.current = null;
    }
    origin.current = null;
  }, []);

  useEffect(() => clear, [clear]);

  const onPointerDown = useCallback(
    (event: React.PointerEvent) => {
      if (!enabled) return;

      if (event.button !== 0 && event.pointerType === 'mouse') return;

      fired.current = false;
      origin.current = { x: event.clientX, y: event.clientY };
      timer.current = setTimeout(() => {
        fired.current = true;
        onLongPress();
      }, LONG_PRESS_MS);
    },
    [enabled, onLongPress],
  );

  const onPointerMove = useCallback(
    (event: React.PointerEvent) => {
      if (!origin.current) return;
      const dx = event.clientX - origin.current.x;
      const dy = event.clientY - origin.current.y;

      if (Math.abs(dx) > MOVE_TOLERANCE_PX || Math.abs(dy) > MOVE_TOLERANCE_PX) clear();
    },
    [clear],
  );

  return {
    onPointerDown,
    onPointerUp: clear,
    onPointerMove,
    onPointerCancel: clear,
    onContextMenu: (event) => {

      if (fired.current) event.preventDefault();
    },
  };
}

export interface SelectionState {
  selected: ReadonlySet<string>;
  isSelected: (id: string) => boolean;
  toggle: (id: string) => void;
  selectAll: (ids: string[]) => void;
  clear: () => void;
  count: number;
  allSelected: (ids: string[]) => boolean;
}

export function useSelection(): SelectionState {
  const [selected, setSelected] = useState<Set<string>>(() => new Set());

  const toggle = useCallback((id: string) => {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);

      return next;
    });
  }, []);

  const selectAll = useCallback((ids: string[]) => {
    setSelected(new Set(ids));
  }, []);

  const clear = useCallback(() => setSelected(new Set()), []);

  return {
    selected,
    isSelected: (id) => selected.has(id),
    toggle,
    selectAll,
    clear,
    count: selected.size,
    allSelected: (ids) => ids.length > 0 && ids.every((id) => selected.has(id)),
  };
}
