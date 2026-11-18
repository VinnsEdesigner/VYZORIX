import { useEffect, useRef, useState, useCallback, useMemo } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import {
  validateTelemetry,
  telemetryFromRaw,
  type WSTelemetry,
  type WSEvent,
  type WSEventType,
  type WSCommandType,
} from '@vyzorix/api-client';
import { postDeviceImeiCommand } from '@/generated-rq/commands/device-commands';
import {
  getGetTelemetryLatestDeviceIdQueryKey,
  getGetTelemetryStatsDeviceIdQueryKey,
} from '@/generated-rq/telemetry/device-telemetry';
import { getGetDashboardStatsQueryKey } from '@/generated-rq/dashboard/dashboard-stats';
import {
  invalidateGetCommandDispatchIdStatus,
  invalidateGetDeviceImeiCommandsPending,
} from '@/generated-rq/commands/device-commands';
import { invalidateGetDashboardEventsRecent } from '@/generated-rq/devices/device-management';
import {
  OnTelemetryReceivedDocument as TELEMETRY_RECEIVED_SUBSCRIPTION,
  OnOrganizationEventDocument as ORGANIZATION_EVENT_SUBSCRIPTION,
  OnCommandStatusChangedDocument as COMMAND_STATUS_SUBSCRIPTION,
} from '@vyzorix/api-client/generated-graphql';
import { useWebSocketStore } from '@/stores';
import { useCurrentOrganizationId } from '@/hooks/_shared/use-current-context';

const MAX_TELEMETRY_HISTORY = 100;
const MAX_EVENTS = 200;

interface SubscriptionDocument {
  loc?: { source: { body: string } };
}

function toQueryString(doc: unknown): string {
  return (doc as SubscriptionDocument)?.loc?.source.body ?? '';
}

interface UseDeviceTelemetryOptions {
  imei?: string;
}

interface UseDeviceTelemetryResult {
  telemetry: WSTelemetry | null;
  telemetryHistory: WSTelemetry[];
  isLoading: boolean;
  error: Error | null;
}

interface TelemetryReceivedPayload {
  id: string;
  deviceId: string;
  receivedAt: string;
  riskScore: number;
  bufferLevel: number;
  thermalTemp: number;
  payload?: string;
}

export function useDeviceTelemetry(
  options: UseDeviceTelemetryOptions = {},
): UseDeviceTelemetryResult {
  const { imei } = options;
  const queryClient = useQueryClient();
  const subscribe = useWebSocketStore((s) => s.subscribe);
  const organizationId = useCurrentOrganizationId();

  const historyRef = useRef<WSTelemetry[]>([]);
  const [telemetry, setTelemetry] = useState<WSTelemetry | null>(null);
  const [history, setHistory] = useState<WSTelemetry[]>([]);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    if (!organizationId || !imei) return;

    const unsubscribe = subscribe<TelemetryReceivedPayload>(
      {
        query: toQueryString(TELEMETRY_RECEIVED_SUBSCRIPTION),
        variables: { deviceId: imei },
      },
      {
        next: (data) => {
          if (!data) return;

          queryClient.invalidateQueries({
            queryKey: getGetTelemetryLatestDeviceIdQueryKey(data.deviceId),
          });
          queryClient.invalidateQueries({
            queryKey: getGetTelemetryStatsDeviceIdQueryKey(data.deviceId),
          });

          const frame = telemetryFromRaw({
            deviceId: data.deviceId,
            timestamp: data.receivedAt ? new Date(data.receivedAt).getTime() : Date.now(),
            riskScore: data.riskScore,
            bufferLevel: data.bufferLevel,
            thermalTemp: data.thermalTemp,
          });
          if (!validateTelemetry(frame)) return;

          historyRef.current = [...historyRef.current, frame].slice(-MAX_TELEMETRY_HISTORY);
          setTelemetry(frame);
          setHistory(historyRef.current);
          setError(null);
        },
        error: (err) => {
          setError(err instanceof Error ? err : new Error(String(err)));
        },
      },
    );
    return () => {
      unsubscribe();
      historyRef.current = [];
      setTelemetry(null);
      setHistory([]);
      setError(null);
    };
  }, [imei, organizationId, subscribe, queryClient]);

  return {
    telemetry,
    telemetryHistory: history,
    isLoading: telemetry === null && error === null,
    error,
  };
}

interface UseDashboardEventsOptions {
  eventTypes?: WSEventType[];
  deviceImei?: string;
}

interface UseDashboardEventsResult {
  events: WSEvent[];
  unreadCount: number;
  markAsRead: (eventId: string) => void;
  clearEvents: () => void;
}

interface OrganizationEventPayload {
  type: string;
  timestamp: string;
  deviceId?: string;
  data?: unknown;
}

let eventIdCounter = 0;

export function useDashboardEvents(
  options: UseDashboardEventsOptions = {},
): UseDashboardEventsResult {
  const { eventTypes, deviceImei } = options;
  const queryClient = useQueryClient();
  const subscribe = useWebSocketStore((s) => s.subscribe);
  const organizationId = useCurrentOrganizationId();

  const stableEventTypes = useMemo(
    () => (eventTypes ? [...eventTypes] : null),

    [eventTypes?.join('|')],
  );

  const eventsRef = useRef<WSEvent[]>([]);
  const [events, setEvents] = useState<WSEvent[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);

  useEffect(() => {
    if (!organizationId) return;

    const filterTypes = stableEventTypes ? new Set<WSEventType>(stableEventTypes) : null;

    const unsubscribe = subscribe<OrganizationEventPayload>(
      {
        query: toQueryString(ORGANIZATION_EVENT_SUBSCRIPTION),
        variables: { orgId: organizationId },
      },
      {
        next: (data) => {
          if (!data) return;

          queryClient.invalidateQueries({ queryKey: getGetDashboardStatsQueryKey() });
          void invalidateGetDashboardEventsRecent(queryClient);

          const eventType = data.type as WSEventType;
          if (filterTypes && !filterTypes.has(eventType)) return;
          if (deviceImei && data.deviceId && data.deviceId !== deviceImei) return;

          const event: WSEvent = {
            id: `evt-${Date.now()}-${eventIdCounter++}`,
            type: eventType,
            deviceId: data.deviceId ?? '',
            timestamp: data.timestamp ? new Date(data.timestamp) : new Date(),
            data: data.data as Record<string, unknown> | undefined,
          };

          eventsRef.current = [event, ...eventsRef.current].slice(0, MAX_EVENTS);
          setEvents(eventsRef.current);
          setUnreadCount((count) => count + 1);
        },
      },
    );
    return () => {
      unsubscribe();
      eventsRef.current = [];
      setEvents([]);
      setUnreadCount(0);
    };
  }, [organizationId, subscribe, queryClient, stableEventTypes, deviceImei]);

  const markAsRead = useCallback(() => {
    setUnreadCount(0);
  }, []);

  const clearEvents = useCallback(() => {
    eventsRef.current = [];
    setEvents([]);
    setUnreadCount(0);
  }, []);

  return { events, unreadCount, markAsRead, clearEvents };
}

interface UseCommandDispatchOptions {
  imei: string;
}

interface UseCommandDispatchResult {
  sendCommand: (
    command: WSCommandType,
    parameters?: Record<string, unknown>,
  ) => Promise<string>;
  pendingCommands: { dispatchId: string; command: WSCommandType; createdAt: Date }[];
  commandStatus: Map<string, 'pending' | 'delivered' | 'failed'>;
}

interface CommandStatusPayload {
  dispatchId: string;
  commandId: string;
  deviceId: string;
  command: string;
  status: string;
  createdAt: string;
}

export function useCommandDispatch(
  options: UseCommandDispatchOptions,
): UseCommandDispatchResult {
  const { imei } = options;
  const queryClient = useQueryClient();
  const subscribe = useWebSocketStore((s) => s.subscribe);
  const organizationId = useCurrentOrganizationId();

  const [pending, setPending] = useState<
    { dispatchId: string; command: WSCommandType; createdAt: Date }[]
  >([]);
  const [commandStatus, setCommandStatus] = useState<
    Map<string, 'pending' | 'delivered' | 'failed'>
  >(new Map());

  useEffect(() => {
    if (!organizationId || !imei) return;

    const unsubscribe = subscribe<CommandStatusPayload>(
      {
        query: toQueryString(COMMAND_STATUS_SUBSCRIPTION),
        variables: { deviceId: imei },
      },
      {
        next: (data) => {
          if (!data?.dispatchId) return;

          const status = mapCommandStatus(data.status);
          if (!status) return;

          setCommandStatus((prev) => {
            const next = new Map(prev);
            next.set(data.dispatchId, status);
            return next;
          });

          void invalidateGetCommandDispatchIdStatus(queryClient, data.dispatchId);
          void invalidateGetDeviceImeiCommandsPending(queryClient, data.deviceId);

          if (status === 'delivered' || status === 'failed') {
            setPending((prev) => prev.filter((p) => p.dispatchId !== data.dispatchId));
          }
        },
      },
    );
    return () => {
      unsubscribe();
    };
  }, [imei, organizationId, subscribe, queryClient]);

  const sendCommand = useCallback(
    async (
      command: WSCommandType,
      parameters?: Record<string, unknown>,
    ): Promise<string> => {
      if (!imei) throw new Error('Cannot dispatch command: no device IMEI');

      const sent = await postDeviceImeiCommand(imei, {
        command,
        args: parameters,
      });

      const dispatchId = sent.dispatchId ?? '';
      const createdAt = new Date(sent.serverTime ?? Date.now());

      setPending((prev) => [
        ...prev,
        { dispatchId, command, createdAt },
      ]);
      setCommandStatus((prev) => {
        const next = new Map(prev);
        next.set(dispatchId, 'pending');
        return next;
      });

      return dispatchId;
    },
    [imei, organizationId],
  );

  return { sendCommand, pendingCommands: pending, commandStatus };
}

function mapCommandStatus(
  status: string,
): 'pending' | 'delivered' | 'failed' | null {
  switch (status) {
    case 'pending':
      return 'pending';
    case 'delivered':
      return 'delivered';
    case 'completed':
      return 'delivered';
    case 'failed':
    case 'cancelled':
      return 'failed';
    default:
      return null;
  }
}
