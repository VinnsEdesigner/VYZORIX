import type {
  DeviceListItem,
  DeviceListResult,
  DeviceDetailResult,
  DeviceCountResult,
  DeviceTagsResult,
  DeviceTagAddedResult,
  DeviceTagRemovedResult,
  DeviceFCMTokenRequest,
  SetDeviceTagsRequest,
  DeviceTransferRequest,
  DeviceTransferResult,
  DeviceSettingsResult,
  UpdateDeviceSettingsRequest,
  ThresholdsResult,
  ConnectionStatusResult,
  ConnectionListResult,
  ConnectionMetricsResult,
  DeviceDisconnectResult,
} from '../../generated/vyzorixUpdateServerAPI.schemas';

export type {
  DeviceListItem,
  DeviceListResult,
  DeviceDetailResult,
  DeviceCountResult,
  DeviceTagsResult,
  DeviceTagAddedResult,
  DeviceTagRemovedResult,
  DeviceFCMTokenRequest,
  SetDeviceTagsRequest,
  DeviceTransferRequest,
  DeviceTransferResult,
  DeviceSettingsResult,
  UpdateDeviceSettingsRequest,
  ThresholdsResult,
  ConnectionStatusResult,
  ConnectionListResult,
  ConnectionMetricsResult,
  DeviceDisconnectResult,
};

export type DeviceStatus = 'online' | 'offline' | 'deregistered';

export function validateIMEI(imei: string): boolean {
  if (!/^\d{15}$/.test(imei)) return false;
  return validateIMEIChecksum(imei);
}

export function validateIMEIChecksum(imei: string): boolean {
  if (!/^\d{15}$/.test(imei)) return false;
  let sum = 0;
  let isEven = false;
  for (let i = imei.length - 1; i >= 0; i--) {
    const ch = imei[i];
    if (ch === undefined) return false;
    let digit = parseInt(ch, 10);
    if (isEven) {
      digit *= 2;
      if (digit > 9) digit -= 9;
    }
    sum += digit;
    isEven = !isEven;
  }
  return sum % 10 === 0;
}

export function validateFCMToken(token: string): string | null {
  if (!token) return 'FCM token is required';
  if (token.length < 100 || token.length > 4000) return 'FCM token has invalid length';
  return null;
}
