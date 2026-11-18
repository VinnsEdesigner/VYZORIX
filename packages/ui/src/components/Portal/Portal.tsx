import { useEffect, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

export interface PortalProps {
  children: ReactNode;

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

export function Portal({ children, root }: PortalProps) {
  const [target, setTarget] = useState<HTMLElement | null>(root ?? null);

  useEffect(() => {
    if (!root) setTarget(getRoot());
  }, [root]);

  if (!target) return null;
  return createPortal(children, target);
}
