import { useEffect, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

export interface PortalProps {
  children: ReactNode;
  /** Portal target. Defaults to a lazily created `#vyzorix-portal` on body. */
  root?: HTMLElement | null;
}

let sharedRoot: HTMLElement | null = null;

function getRoot(): HTMLElement | null {
  if (typeof document === 'undefined') return null;
  if (sharedRoot) return sharedRoot;
  const existing = document.getElementById('vyzorix-portal');
  if (existing) {
    sharedRoot = existing;
    return sharedRoot;
  }
  const el = document.createElement('div');
  el.id = 'vyzorix-portal';
  document.body.appendChild(el);
  sharedRoot = el;
  return sharedRoot;
}

/**
 * Renders children into a node outside the React tree's DOM position.
 *
 * Overlays must not be clipped by an ancestor's `overflow: hidden` or trapped in
 * its stacking context, so they render through here. Returns null during SSR,
 * where there is no document.
 */
export function Portal({ children, root }: PortalProps) {
  const [target, setTarget] = useState<HTMLElement | null>(root ?? null);

  useEffect(() => {
    if (!root) setTarget(getRoot());
  }, [root]);

  if (!target) return null;
  return createPortal(children, target);
}
