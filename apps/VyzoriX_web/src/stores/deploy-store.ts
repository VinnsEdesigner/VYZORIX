import { createVyzorStore } from '@/lib/state';

export interface DeploySelection {
  imei: string | null;
  deviceName: string | null;
}

export interface DeployStoreState {

  selection: DeploySelection;

  expandedApp: string | null;

  revealedSecretKey: string | null;

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
