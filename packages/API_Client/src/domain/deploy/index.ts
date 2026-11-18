import type {
  AppMarking,
  CredentialListResponse,
  CredentialResponse,
  CurrentResponse,
  DeployResponse,
  DeviceSummary,
  DriftResponse,
  IdentityResponse,
  MintCredentialResponse,
  SecretListResponse,
  SecretNameResponse,
} from '../../generated/vyzorixUpdateServerAPI.schemas';

export type {
  AppMarking,
  CiphertextResponse,
  CredentialListResponse,
  CredentialResponse,
  CurrentResponse,
  DeployResponse,
  DeviceSummary,
  GetDeploySchemaVersion200,
  GetDeployWhoamiParams,
  IdentityResponse,
  MintCredentialRequest,
  MintCredentialResponse,
  PostDeployParams,
  SecretListResponse,
  SecretNameResponse,
  SetSecretRequest,
  SetSecretResponse,
} from '../../generated/vyzorixUpdateServerAPI.schemas';

export function toDate(epochMillis: number | null | undefined): Date | null {
  return epochMillis === null || epochMillis === undefined ? null : new Date(epochMillis);
}

export interface DeployApp {
  name: string;
  custom: boolean;
}

export interface DeployCredential {
  id: string;
  name: string;
  prefix: string;
  deviceId: string | null;
  createdBy: string | null;
  createdAt: Date | null;
  lastUsedAt: Date | null;
  revokedAt: Date | null;
}

export interface MintedDeployCredential extends DeployCredential {
  token: string;
}

export interface DeploySecretName {
  key: string;
  generation: number;
  updatedAt: Date | null;
}

export interface DeployAppSecrets {
  app: string;
  secrets: DeploySecretName[];
}

export interface DeviceConfigSnapshot {
  deviceId: string;
  version: number;
  kind: string;
  createdAt: Date | null;
  document: string;

  drift: DeployDrift;
}

export type DeployDriftState = 'in_sync' | 'unknown' | 'behind' | 'ahead' | 'app_changed';

export interface DeployDrift {
  state: DeployDriftState;

  detail: string;

  reportedVersion: number | null;

  reportedAt: Date | null;
}

export interface DeployIdentity {
  orgId: string | null;
  orgName: string | null;
  userName: string | null;
  email: string | null;
  role: string | null;
  device: DeviceSummary | null;
}

export interface DeployResult {
  version: number;
  artifacts: number;
  authorizedBy: string;
  dryRun: boolean;
  apps: DeployApp[];
  document: string | null;
}

export function deployAppFromWire(wire: AppMarking): DeployApp {
  return { name: wire.name ?? '', custom: wire.custom ?? false };
}

export function deployResultFromWire(wire: DeployResponse): DeployResult {
  return {
    version: wire.version ?? 0,
    artifacts: wire.artifacts ?? 0,
    authorizedBy: wire.authorizedBy ?? '',
    dryRun: wire.dryRun ?? false,
    apps: (wire.apps ?? []).map(deployAppFromWire),
    document: wire.document ?? null,
  };
}

export function deviceConfigFromWire(wire: CurrentResponse): DeviceConfigSnapshot {
  return {
    deviceId: wire.deviceId ?? '',
    version: wire.version ?? 0,
    kind: wire.kind ?? '',
    createdAt: toDate(wire.createdAt),
    document: wire.document ?? '',
    drift: deployDriftFromWire(wire.drift),
  };
}

const DRIFT_STATES: readonly DeployDriftState[] = [
  'in_sync',
  'unknown',
  'behind',
  'ahead',
  'app_changed',
];

export function deployDriftFromWire(wire: DriftResponse | undefined): DeployDrift {
  const state = DRIFT_STATES.find((s) => s === wire?.state) ?? 'unknown';

  return {
    state,
    detail: wire?.detail ?? '',
    reportedVersion: wire?.reportedVersion ?? null,
    reportedAt: toDate(wire?.reportedAt),
  };
}

export function deployIdentityFromWire(wire: IdentityResponse): DeployIdentity {
  return {
    orgId: wire.orgId ?? null,
    orgName: wire.orgName ?? null,
    userName: wire.userName ?? null,
    email: wire.email ?? null,
    role: wire.role ?? null,
    device: wire.device ?? null,
  };
}

export function deployCredentialFromWire(wire: CredentialResponse): DeployCredential {
  return {
    id: wire.id ?? '',
    name: wire.name ?? '',
    prefix: wire.prefix ?? '',
    deviceId: wire.deviceId ?? null,
    createdBy: wire.createdBy ?? null,
    createdAt: toDate(wire.createdAt),
    lastUsedAt: toDate(wire.lastUsedAt),
    revokedAt: toDate(wire.revokedAt),
  };
}

export function mintedCredentialFromWire(wire: MintCredentialResponse): MintedDeployCredential {
  return { ...deployCredentialFromWire(wire), token: wire.token ?? '' };
}

export function deployCredentialsFromWire(wire: CredentialListResponse): DeployCredential[] {
  return (wire.credentials ?? []).map(deployCredentialFromWire);
}

export function deploySecretNameFromWire(wire: SecretNameResponse): DeploySecretName {
  return {
    key: wire.key ?? '',
    generation: wire.generation ?? 0,
    updatedAt: toDate(wire.updatedAt),
  };
}

export function deployAppSecretsFromWire(wire: SecretListResponse): DeployAppSecrets {
  return {
    app: wire.app ?? '',
    secrets: (wire.secrets ?? []).map(deploySecretNameFromWire),
  };
}

export type DeployExposeMode = 'loopback' | 'lan' | 'cloud';

export function getDeployExposeLabel(mode: DeployExposeMode): string {
  const labels: Record<DeployExposeMode, string> = {
    loopback: 'Loopback',
    lan: 'LAN',
    cloud: 'Cloud',
  };
  return labels[mode];
}

export function isCredentialActive(credential: DeployCredential): boolean {
  return credential.revokedAt === null;
}
