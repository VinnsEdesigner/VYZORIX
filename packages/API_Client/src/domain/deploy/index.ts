// Deploy domain — the declarative, signed deployment surface
// (`documents/VYZORIX_DEPLOY_CONTRACT.md`).
//
// Two layers, same as every other bounded context:
//   1. the generated wire types (as the server emits them) are re-exported
//      verbatim, and
//   2. a small set of hook-facing domain types (camelCase, `Date` for epoch
//      millis) plus the mappers that produce them.
//
// The wire carries epoch **milliseconds** (the Go handlers call `.UnixMilli()`),
// so every timestamp mapper goes through `toDate`.
import type {
  AppMarking,
  CredentialListResponse,
  CredentialResponse,
  CurrentResponse,
  DeployResponse,
  DeviceSummary,
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

/** Epoch milliseconds -> `Date`; `null`/`undefined` pass through as `null`. */
export function toDate(epochMillis: number | null | undefined): Date | null {
  return epochMillis === null || epochMillis === undefined ? null : new Date(epochMillis);
}

// ---- Hook-facing domain types (camelCase, Date-based) ----

/** An app the deploy declares; `custom` marks one absent from the catalog. */
export interface DeployApp {
  name: string;
  custom: boolean;
}

/** A device-scoped deploy credential. The token is only in `MintedDeployCredential`. */
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

/** A credential as returned by `POST` — the only response that carries the token. */
export interface MintedDeployCredential extends DeployCredential {
  token: string;
}

/** One secret name an app expects. Never a value. */
export interface DeploySecretName {
  key: string;
  generation: number;
  updatedAt: Date | null;
}

/** The secret names an app expects, and their generation. */
export interface DeployAppSecrets {
  app: string;
  secrets: DeploySecretName[];
}

/** The config a device currently holds. */
export interface DeviceConfigSnapshot {
  deviceId: string;
  version: number;
  kind: string;
  createdAt: Date | null;
  document: string;
}

/** The operator's identity on the deploy surface. */
export interface DeployIdentity {
  orgId: string | null;
  orgName: string | null;
  operatorName: string | null;
  email: string | null;
  role: string | null;
  device: DeviceSummary | null;
}

/** The outcome of a compile/sign. `dryRun` is true when nothing was stored. */
export interface DeployResult {
  version: number;
  artifacts: number;
  authorizedBy: string;
  dryRun: boolean;
  apps: DeployApp[];
  document: string | null;
}

// ---- Wire -> domain mappers ----

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
  };
}

export function deployIdentityFromWire(wire: IdentityResponse): DeployIdentity {
  return {
    orgId: wire.orgId ?? null,
    orgName: wire.orgName ?? null,
    operatorName: wire.operatorName ?? null,
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

// ---- Presentation helpers ----

/** The `expose` modes an app may declare (contract §4.1). */
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
