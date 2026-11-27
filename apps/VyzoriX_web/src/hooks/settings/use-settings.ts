import {
  useGetMeSettings,
  usePatchMeSettings,
  useGetMeThresholds,
  usePatchMeThresholds,
  useGetMeNotifications,
  usePatchMeNotifications,
  usePostMeNotificationsWebhookTest,
  usePostMeNotificationsWebhookRotate,
} from '@/generated-rq/settings/user-settings';
import {
  useGetAuthMeNotificationsSlackChannels,
  useGetAuthMeNotificationsHistory,
  usePostAuthMeNotificationsHistoryRead,
  usePostAuthMeNotificationsPushTest,
  getGetAuthMeNotificationsHistoryQueryKey,
} from '@/generated-rq/notifications/notifications';

export function useSettings() {
  return useGetMeSettings();
}

export function useUpdateSettings() {
  return usePatchMeSettings();
}

export function useThresholds() {
  return useGetMeThresholds();
}

export function useUpdateThresholds() {
  return usePatchMeThresholds();
}

export function useNotifications() {
  return useGetMeNotifications();
}

export function useUpdateNotifications() {
  return usePatchMeNotifications();
}

export function useTestWebhook() {
  return usePostMeNotificationsWebhookTest();
}

export function useRotateWebhookSecret() {
  return usePostMeNotificationsWebhookRotate();
}

export function useSlackChannels(options?: Parameters<typeof useGetAuthMeNotificationsSlackChannels>[0]) {
  return useGetAuthMeNotificationsSlackChannels(options);
}

export function useNotificationHistory(
  params?: Parameters<typeof useGetAuthMeNotificationsHistory>[0],
  options?: Parameters<typeof useGetAuthMeNotificationsHistory>[1],
) {
  return useGetAuthMeNotificationsHistory(params, options);
}

export function useMarkNotificationsRead() {
  return usePostAuthMeNotificationsHistoryRead();
}

export function useSendTestPush() {
  return usePostAuthMeNotificationsPushTest();
}

export { getGetAuthMeNotificationsHistoryQueryKey };

export type {
  NotificationSettings,
  NotificationHistoryEntry,
  NotificationHistoryResult,
  SlackChannelsResult,
} from '@vyzorix/api-client';
