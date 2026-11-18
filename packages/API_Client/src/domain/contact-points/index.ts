export type {
  ContactPoint,
  ContactPointListResult,
  ContactPointRequest,
  ContactPointTestResult,
} from '../../generated/vyzorixUpdateServerAPI.schemas';

export const CONTACT_POINT_CHANNELS = ['email', 'webhook', 'slack'] as const;
export type ContactPointChannel = (typeof CONTACT_POINT_CHANNELS)[number];

export function isContactPointChannel(v: string): v is ContactPointChannel {
  return CONTACT_POINT_CHANNELS.includes(v as ContactPointChannel);
}
