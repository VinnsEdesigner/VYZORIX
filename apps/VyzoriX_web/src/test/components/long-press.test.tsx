import { describe, it, expect, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useLongPress, LONG_PRESS_MS } from '@/ui/components/shared/use-long-press';

function pointer(x = 0, y = 0, pointerType = 'touch') {
  return { button: 0, pointerType, clientX: x, clientY: y } as unknown as React.PointerEvent;
}

describe('useLongPress', () => {
  it('fires after the hold threshold', () => {
    vi.useFakeTimers();
    const onHold = vi.fn();
    const { result } = renderHook(() => useLongPress(onHold));

    act(() => result.current.onPointerDown(pointer()));
    act(() => { vi.advanceTimersByTime(LONG_PRESS_MS - 1); });
    expect(onHold).not.toHaveBeenCalled();

    act(() => { vi.advanceTimersByTime(2); });
    expect(onHold).toHaveBeenCalledTimes(1);
    vi.useRealTimers();
  });

  it('does not fire when released early, which is what an ordinary tap is', () => {
    vi.useFakeTimers();
    const onHold = vi.fn();
    const { result } = renderHook(() => useLongPress(onHold));

    act(() => result.current.onPointerDown(pointer()));
    act(() => result.current.onPointerUp(pointer()));
    act(() => { vi.advanceTimersByTime(LONG_PRESS_MS * 3); });

    expect(onHold).not.toHaveBeenCalled();
    vi.useRealTimers();
  });

  it('does not fire when the finger moves, so a scroll never selects', () => {
    vi.useFakeTimers();
    const onHold = vi.fn();
    const { result } = renderHook(() => useLongPress(onHold));

    act(() => result.current.onPointerDown(pointer()));
    act(() => result.current.onPointerMove(pointer(0, 60)));
    act(() => { vi.advanceTimersByTime(LONG_PRESS_MS * 2); });

    expect(onHold).not.toHaveBeenCalled();
    vi.useRealTimers();
  });

  it('tolerates a small jitter, because a real finger is never perfectly still', () => {
    vi.useFakeTimers();
    const onHold = vi.fn();
    const { result } = renderHook(() => useLongPress(onHold));

    act(() => result.current.onPointerDown(pointer()));
    act(() => result.current.onPointerMove(pointer(3, 4)));
    act(() => { vi.advanceTimersByTime(LONG_PRESS_MS + 1); });

    expect(onHold).toHaveBeenCalledTimes(1);
    vi.useRealTimers();
  });

  it('ignores a right-click, which has its own meaning', () => {
    vi.useFakeTimers();
    const onHold = vi.fn();
    const { result } = renderHook(() => useLongPress(onHold));

    act(() => result.current.onPointerDown({ button: 2, pointerType: 'mouse', clientX: 0, clientY: 0 } as unknown as React.PointerEvent));
    act(() => { vi.advanceTimersByTime(LONG_PRESS_MS * 2); });

    expect(onHold).not.toHaveBeenCalled();
    vi.useRealTimers();
  });

  it('is inert when disabled', () => {
    vi.useFakeTimers();
    const onHold = vi.fn();
    const { result } = renderHook(() => useLongPress(onHold, false));

    act(() => result.current.onPointerDown(pointer()));
    act(() => { vi.advanceTimersByTime(LONG_PRESS_MS * 2); });

    expect(onHold).not.toHaveBeenCalled();
    vi.useRealTimers();
  });
});
