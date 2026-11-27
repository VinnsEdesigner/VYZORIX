import { useQueryClient } from '@tanstack/react-query';
import {
  useGetAuthMeNotificationsPushPublicKey,
  usePostAuthMeNotificationsPushSubscribe,
  usePostAuthMeNotificationsPushUnsubscribe,
} from '@/generated-rq/notifications/notifications';
import { getGetMeNotificationsQueryKey } from '@/generated-rq/settings/user-settings';
import {
  currentPushSubscription,
  dropPushSubscription,
  pushSupported,
  subscribeToPush,
} from '@/lib/push/web-push';

export function usePushPublicKey() {
  return useGetAuthMeNotificationsPushPublicKey({
    query: { enabled: pushSupported(), staleTime: 5 * 60_000 },
  });
}

export function useSubscribeToPush() {
  const publicKeyQuery = usePushPublicKey();
  const subscribe = usePostAuthMeNotificationsPushSubscribe();
  const queryClient = useQueryClient();

  const subscribeAsync = async () => {
    const { enabled, publicKey } = publicKeyQuery.data ?? {};
    if (!enabled || !publicKey) {
      throw new Error('push_not_configured');
    }

    const subscription = await subscribeToPush(publicKey);

    try {
      await subscribe.mutateAsync({ data: subscription.toJSON() });
    } catch (error) {
      await dropPushSubscription(subscription).catch(() => undefined);
      throw error;
    }

    void queryClient.invalidateQueries({ queryKey: getGetMeNotificationsQueryKey() });
  };

  return {
    subscribe: subscribeAsync,
    isSubscribing: subscribe.isPending,
    isError: subscribe.isError,
    error: subscribe.error,
  };
}

export function useUnsubscribeFromPush() {
  const unsubscribe = usePostAuthMeNotificationsPushUnsubscribe();
  const queryClient = useQueryClient();

  const unsubscribeAsync = async () => {
    const subscription = await currentPushSubscription();
    if (!subscription) return;

    await unsubscribe.mutateAsync({ data: { endpoint: subscription.endpoint } });
    await dropPushSubscription(subscription);

    void queryClient.invalidateQueries({ queryKey: getGetMeNotificationsQueryKey() });
  };

  return {
    unsubscribe: unsubscribeAsync,
    isUnsubscribing: unsubscribe.isPending,
    isError: unsubscribe.isError,
    error: unsubscribe.error,
  };
}
