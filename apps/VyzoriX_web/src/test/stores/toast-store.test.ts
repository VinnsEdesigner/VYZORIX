import { describe, it, expect, beforeEach } from 'vitest';
import { TOAST_TIMEOUT, useToastStore } from '@/stores/toast-store';

describe('toast store', () => {
  beforeEach(() => {
    useToastStore.setState({ toasts: [] });
  });

  it('adds a toast and returns its id', () => {
    const id = useToastStore.getState().add({ title: 'Saved', severity: 'success' });
    expect(id).toBeTruthy();
    expect(useToastStore.getState().toasts).toHaveLength(1);
    expect(useToastStore.getState().toasts[0]?.title).toBe('Saved');
  });

  it('keeps insertion order, so the newest sits at the bottom of the stack', () => {
    const s = useToastStore.getState();
    s.add({ title: 'first', severity: 'info' });
    s.add({ title: 'second', severity: 'info' });
    expect(useToastStore.getState().toasts.map((t) => t.title)).toEqual(['first', 'second']);
  });

  it('removes by id and leaves the rest', () => {
    const s = useToastStore.getState();
    const a = s.add({ title: 'a', severity: 'success' });
    s.add({ title: 'b', severity: 'success' });
    useToastStore.getState().remove(a);
    expect(useToastStore.getState().toasts.map((t) => t.title)).toEqual(['b']);
  });

  it('ignores an unknown id', () => {
    useToastStore.getState().add({ title: 'a', severity: 'success' });
    useToastStore.getState().remove('nope');
    expect(useToastStore.getState().toasts).toHaveLength(1);
  });

  it('gives each toast a distinct id', () => {
    const s = useToastStore.getState();
    const a = s.add({ title: 'a', severity: 'success' });
    const b = s.add({ title: 'a', severity: 'success' });
    expect(a).not.toBe(b);
  });
});

describe('toast timeouts', () => {
  it('gives an error longer on screen than a success', () => {
    expect(TOAST_TIMEOUT.error).toBeGreaterThan(TOAST_TIMEOUT.success);
  });

  it('treats info like a success', () => {
    expect(TOAST_TIMEOUT.info).toBe(TOAST_TIMEOUT.success);
  });
});
