import { createVyzorStore } from '@/lib/state';

/** Which device's deploy surface the view is showing. */
export interface DeploySelection {
  imei: string | null;
  deviceName: string | null;
}

/**
 * Client-only UI state for the read-only Deploy view (US-010). Server data —
 * config, credentials, secret names — lives in TanStack Query hooks, not here.
 */
export interface DeployStoreState {
  /** The device under inspection; `null` until one is picked. */
  selection: DeploySelection;
  /** The app whose secrets panel is open, if any. */
  expandedApp: string | null;
  /** The secret key whose value panel is open (names are shown by default). */
  revealedSecretKey: string | null;
  /**
   * A freshly minted credential token. Shown exactly once — it is never
   * persisted and cleared as soon as the operator dismisses it.
   */
  pendingCredentialToken: string | null;
  selectDevice: (imei: string, deviceName?: string) => void;
  clearSelection: () => void;
  toggleApp: (app: string) => void;
  revealSecret: (key: string | null) => void;
  setPendingCredentialToken: (token: string | null) => void;
  clear: () => void;
}

const EMPTY_SELECTION: DeploySelection = { imei: null, deviceName: null };

export const useDeployStore = createVyzorStore<DeployStoreState>('DeployStore', (set) => ({
  selection: { ...EMPTY_SELECTION },
  expandedApp: null,
  revealedSecretKey: null,
  pendingCredentialToken: null,

  selectDevice: (imei, deviceName) =>
    set({ selection: { imei, deviceName: deviceName ?? null } }),

  clearSelection: () => set({ selection: { ...EMPTY_SELECTION } }),

  toggleApp: (app) =>
    set((state) => ({ expandedApp: state.expandedApp === app ? null : app })),

  revealSecret: (key) => set({ revealedSecretKey: key }),

  setPendingCredentialToken: (token) => set({ pendingCredentialToken: token }),

  clear: () =>
    set({
      selection: { ...EMPTY_SELECTION },
      expandedApp: null,
      revealedSecretKey: null,
      pendingCredentialToken: null,
    }),
}));
