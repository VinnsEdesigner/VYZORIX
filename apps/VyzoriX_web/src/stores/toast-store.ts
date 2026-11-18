import { createVyzorStore } from '@/lib/state';

export type ToastSeverity = 'success' | 'warning' | 'error' | 'info';

export interface Toast {
  id: string;
  title: string;
  text?: string;
  severity: ToastSeverity;
}

export interface ToastStoreState {
  toasts: Toast[];
  add: (toast: Omit<Toast, 'id'>) => string;
  remove: (id: string) => void;
}

function makeId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID();
  }
  return `toast-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

export const useToastStore = createVyzorStore<ToastStoreState>('ToastStore', (set) => ({
  toasts: [],
  add: (toast) => {
    const id = makeId();
    set((state) => ({ toasts: [...state.toasts, { ...toast, id }] }));
    return id;
  },
  remove: (id) => set((state) => ({ toasts: state.toasts.filter((t) => t.id !== id) })),
}));

export function notify(toast: Omit<Toast, 'id'>): string {
  return useToastStore.getState().add(toast);
}

export const notifySuccess = (title: string, text?: string) => notify({ title, text, severity: 'success' });
export const notifyWarning = (title: string, text?: string) => notify({ title, text, severity: 'warning' });
export const notifyError = (title: string, text?: string) => notify({ title, text, severity: 'error' });
export const notifyInfo = (title: string, text?: string) => notify({ title, text, severity: 'info' });

export const TOAST_TIMEOUT: Record<ToastSeverity, number> = {
  success: 3000,
  info: 3000,
  warning: 5000,
  error: 7000,
};

export function useToast() {
  return {
    success: notifySuccess,
    warning: notifyWarning,
    error: notifyError,
    info: notifyInfo,
    remove: (id: string) => useToastStore.getState().remove(id),
  };
}
