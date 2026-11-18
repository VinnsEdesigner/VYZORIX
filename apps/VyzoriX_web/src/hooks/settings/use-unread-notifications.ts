import { useGetAuthMeNotificationsHistory } from '@/generated-rq/notifications/notifications';
import type { NotificationHistoryResult } from '@vyzorix/api-client';

const POLL_MS = 30_000;

export function useUnreadNotifications(): number {
  const query = useGetAuthMeNotificationsHistory(
    { limit: 1, offset: 0 },
    { query: { refetchInterval: POLL_MS, refetchOnWindowFocus: true, staleTime: POLL_MS } },
  );

  return (query.data as NotificationHistoryResult | undefined)?.unread ?? 0;
}
