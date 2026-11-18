import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { waitFor, act, cleanup } from '@testing-library/react';
import { renderHookWithQueryClient } from '../helpers/render-hook';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { useAuthStore } from '@/stores/auth-store';
import { useWebSocketStore } from '@/stores/websocket-store';
import {
  useWebSocketConnection,
  useDeviceTelemetry,
  useDashboardEvents,
  useCommandDispatch,
} from '@/hooks/realtime';

setupIntegrationTest();

type Listener = (event: Event) => void;

class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  static OPEN = 1;
  static CLOSED = 3;

  readonly url: string;
  readonly protocol: string;
  readyState = FakeWebSocket.OPEN;
  onopen: ((ev: Event) => void) | null = null;
  onclose: ((ev: CloseEvent) => void) | null = null;
  onerror: ((ev: Event) => void) | null = null;
  onmessage: ((ev: MessageEvent) => void) | null = null;
  sent: string[] = [];
  private listeners: { type: string; fn: Listener }[] = [];

  constructor(url: string, protocols?: string | string[]) {
    this.url = url;
    this.protocol = Array.isArray(protocols) ? protocols[0] ?? '' : protocols ?? '';
    FakeWebSocket.instances.push(this);
    queueMicrotask(() => {
      this.readyState = FakeWebSocket.OPEN;
      this.dispatchEvent(new Event('open'));
    });
  }

  addEventListener(type: string, listener: Listener) {
    this.listeners.push({ type, fn: listener });
  }
  removeEventListener(type: string, listener: Listener) {
    this.listeners = this.listeners.filter((l) => !(l.type === type && l.fn === listener));
  }
  dispatchEvent(event: Event) {
    const type = event.type;
    const handler = (this as unknown as Record<string, ((e: Event) => void) | null>)[`on${type}`];
    handler?.(event);
    for (const l of this.listeners) {
      if (l.type === type) l.fn(event);
    }
    return true;
  }
  send(data: string) {
    this.sent.push(data);
    const msg = JSON.parse(data) as { type: string };
    if (msg.type === 'connection_init') {
      this.receiveMessage({ type: 'connection_ack' });
    }
  }
  close(code = 1000, reason = '') {
    this.readyState = FakeWebSocket.CLOSED;
    const event = new CloseEvent('close', { code, reason, wasClean: true });
    this.onclose?.(event);
    this.dispatchEvent(event);
  }

  receiveMessage(message: unknown) {
    this.dispatchEvent(new MessageEvent('message', { data: JSON.stringify(message) }));
  }
}

function waitForState(predicate: () => boolean, timeout = 2000): Promise<void> {
  return new Promise((resolve, reject) => {
    const start = Date.now();
    const check = () => {
      if (predicate()) return resolve();
      if (Date.now() - start > timeout) {
        return reject(new Error('Timed out waiting for state'));
      }
      setTimeout(check, 5);
    };
    check();
  });
}

function waitForSubscribeMessage(
  socket: FakeWebSocket,
  timeout = 2000,
): Promise<{ id: string }> {
  return new Promise((resolve, reject) => {
    const start = Date.now();
    const check = () => {
      const sub = socket.sent
        .map((raw) => JSON.parse(raw) as { type: string; id?: string })
        .find((m) => m.type === 'subscribe' && m.id);
      if (sub?.id) return resolve({ id: sub.id });
      if (Date.now() - start > timeout) {
        return reject(new Error(`Timed out waiting for subscribe message; sent=${JSON.stringify(socket.sent)}`));
      }
      setTimeout(check, 5);
    };
    check();
  });
}

beforeEach(() => {
  FakeWebSocket.instances = [];
  vi.stubGlobal('WebSocket', FakeWebSocket);
  useWebSocketStore.getState().disconnect();
  useAuthStore.getState().setOrganization('org-1');
});

afterEach(async () => {

  cleanup();
  useWebSocketStore.getState().disconnect();
  for (const socket of FakeWebSocket.instances) {
    if (socket.readyState === FakeWebSocket.OPEN) {
      socket.close(1000, 'teardown');
    }
  }
  await new Promise((r) => setTimeout(r, 0));
  vi.unstubAllGlobals();
  useAuthStore.getState().setOrganization(null);
});

describe('useWebSocketConnection (§8.1)', () => {
  it('auto-connects when an organization is selected', async () => {
    const { result } = renderHookWithQueryClient(() => useWebSocketConnection());
    await waitFor(() => expect(result.current.isConnected).toBe(true));
    expect(result.current.lastConnectedAt).toBeInstanceOf(Date);
    expect(result.current.connectionError).toBeNull();
  });

  it('exposes the spec return shape (isConnected/isReconnecting/connect/disconnect/lastConnectedAt/connectionError)', async () => {
    const { result } = renderHookWithQueryClient(() => useWebSocketConnection());
    await waitFor(() => expect(result.current.isConnected).toBe(true));
    expect(typeof result.current.connect).toBe('function');
    expect(typeof result.current.disconnect).toBe('function');
    expect(result.current.isReconnecting).toBe(false);
  });

  it('exposes a null connectionError on a clean connection', async () => {
    const { result } = renderHookWithQueryClient(() => useWebSocketConnection());
    await waitFor(() => expect(result.current.isConnected).toBe(true));
    expect(result.current.connectionError).toBeNull();

  });
});

describe('useDeviceTelemetry (§8.2)', () => {
  it('isLoading is true before the first frame arrives', () => {
    const { result } = renderHookWithQueryClient(() => useDeviceTelemetry({ imei: 'dev-1' }));
    expect(result.current.telemetry).toBeNull();
    expect(result.current.telemetryHistory).toHaveLength(0);
    expect(result.current.isLoading).toBe(true);
    expect(result.current.error).toBeNull();
  });

  it('receives a telemetry frame and exposes it as the latest + history', async () => {
    const { result } = renderHookWithQueryClient(() => useDeviceTelemetry({ imei: 'dev-1' }));
    await waitForState(() => useWebSocketStore.getState().isConnected);

    const socket = FakeWebSocket.instances[0]!;
    const sub = await waitForSubscribeMessage(socket);
    act(() => {
      socket.receiveMessage({
        id: sub.id,
        type: 'next',
        payload: {
          data: {
            id: 'tel-1',
            deviceId: 'dev-1',
            receivedAt: new Date().toISOString(),
            riskScore: 42,
            bufferLevel: 30,
            thermalTemp: 55,
          },
        },
      });
    });

    await waitFor(() => expect(result.current.telemetry).not.toBeNull());
    expect(result.current.telemetry?.deviceId).toBe('dev-1');
    expect(result.current.telemetry?.riskScore).toBe(42);
    expect(result.current.telemetryHistory).toHaveLength(1);
    expect(result.current.isLoading).toBe(false);
  });

  it('accumulates frames into the history buffer (capped at 100)', async () => {
    const { result } = renderHookWithQueryClient(() => useDeviceTelemetry({ imei: 'dev-1' }));
    await waitForState(() => useWebSocketStore.getState().isConnected);

    const socket = FakeWebSocket.instances[0]!;
    const sub = await waitForSubscribeMessage(socket);

    for (let i = 0; i < 105; i++) {
      act(() => {
        socket.receiveMessage({
          id: sub.id,
          type: 'next',
          payload: {
            data: {
              id: `tel-${i}`,
              deviceId: 'dev-1',
              receivedAt: new Date().toISOString(),
              riskScore: i % 101,
              bufferLevel: 10,
              thermalTemp: 20,
            },
          },
        });
      });
    }

    await waitFor(() => expect(result.current.telemetryHistory).toHaveLength(100));

    expect(result.current.telemetry?.riskScore).toBe(3);
  });

  it('drops frames that fail validation (riskScore out of range)', async () => {
    const { result } = renderHookWithQueryClient(() => useDeviceTelemetry({ imei: 'dev-1' }));
    await waitForState(() => useWebSocketStore.getState().isConnected);

    const socket = FakeWebSocket.instances[0]!;
    const sub = await waitForSubscribeMessage(socket);
    act(() => {
      socket.receiveMessage({
        id: sub.id,
        type: 'next',
        payload: {
          data: {
            id: 'bad',
            deviceId: 'dev-1',
            receivedAt: new Date().toISOString(),
            riskScore: 150,
            bufferLevel: 30,
            thermalTemp: 55,
          },
        },
      });
    });

    expect(result.current.telemetry).toBeNull();
    expect(result.current.telemetryHistory).toHaveLength(0);
  });
});

describe('useDashboardEvents (§8.3)', () => {
  it('starts empty with zero unread', () => {
    const { result } = renderHookWithQueryClient(() => useDashboardEvents());
    expect(result.current.events).toHaveLength(0);
    expect(result.current.unreadCount).toBe(0);
  });

  it('collects events and increments unreadCount', async () => {
    const { result } = renderHookWithQueryClient(() => useDashboardEvents());
    await waitForState(() => useWebSocketStore.getState().isConnected);

    const socket = FakeWebSocket.instances[0]!;
    const sub = await waitForSubscribeMessage(socket);

    act(() => {
      socket.receiveMessage({
        id: sub.id,
        type: 'next',
        payload: {
          data: {
            type: 'DEVICE_CONNECTED',
            timestamp: new Date().toISOString(),
            deviceId: 'dev-1',
          },
        },
      });
    });

    await waitFor(() => expect(result.current.events).toHaveLength(1));
    expect(result.current.events[0]?.type).toBe('DEVICE_CONNECTED');
    expect(result.current.unreadCount).toBe(1);
  });

  it('markAsRead resets unreadCount but keeps events', async () => {
    const { result } = renderHookWithQueryClient(() => useDashboardEvents());
    await waitForState(() => useWebSocketStore.getState().isConnected);

    const socket = FakeWebSocket.instances[0]!;
    const sub = await waitForSubscribeMessage(socket);
    act(() => {
      socket.receiveMessage({
        id: sub.id,
        type: 'next',
        payload: { data: { type: 'DEVICE_DISCONNECTED', timestamp: new Date().toISOString(), deviceId: 'dev-1' } },
      });
    });
    await waitFor(() => expect(result.current.unreadCount).toBe(1));

    act(() => result.current.markAsRead('evt-0'));
    expect(result.current.unreadCount).toBe(0);
    expect(result.current.events).toHaveLength(1);
  });

  it('clearEvents empties the event list and resets unread', async () => {
    const { result } = renderHookWithQueryClient(() => useDashboardEvents());
    await waitForState(() => useWebSocketStore.getState().isConnected);

    const socket = FakeWebSocket.instances[0]!;
    const sub = await waitForSubscribeMessage(socket);
    act(() => {
      socket.receiveMessage({
        id: sub.id,
        type: 'next',
        payload: { data: { type: 'THRESHOLD_BREACH', timestamp: new Date().toISOString(), deviceId: 'dev-1' } },
      });
    });
    await waitFor(() => expect(result.current.events).toHaveLength(1));

    act(() => result.current.clearEvents());
    expect(result.current.events).toHaveLength(0);
    expect(result.current.unreadCount).toBe(0);
  });

  it('filters by eventTypes', async () => {
    const { result } = renderHookWithQueryClient(() =>
      useDashboardEvents({ eventTypes: ['DEVICE_CONNECTED'] }),
    );
    await waitForState(() => useWebSocketStore.getState().isConnected);

    const socket = FakeWebSocket.instances[0]!;
    const sub = await waitForSubscribeMessage(socket);

    act(() => {
      socket.receiveMessage({
        id: sub.id,
        type: 'next',
        payload: { data: { type: 'DEVICE_DISCONNECTED', timestamp: new Date().toISOString(), deviceId: 'dev-1' } },
      });
    });
    await waitForState(() => useWebSocketStore.getState().activeSubscriptions >= 1);
    expect(result.current.events).toHaveLength(0);

    act(() => {
      socket.receiveMessage({
        id: sub.id,
        type: 'next',
        payload: { data: { type: 'DEVICE_CONNECTED', timestamp: new Date().toISOString(), deviceId: 'dev-1' } },
      });
    });
    await waitFor(() => expect(result.current.events).toHaveLength(1));
    expect(result.current.events[0]?.type).toBe('DEVICE_CONNECTED');
  });
});

describe('useCommandDispatch (§8.4)', () => {
  it('sendCommand dispatches via REST and tracks pending status', async () => {
    const { result } = renderHookWithQueryClient(() =>
      useCommandDispatch({ imei: 'dev-1' }),
    );
    await waitForState(() => useWebSocketStore.getState().isConnected);

    let dispatchId: string | undefined;
    await act(async () => {
      dispatchId = await result.current.sendCommand('FORCE_SPEAKER', { active: true });
    });

    expect(dispatchId).toBeDefined();
    expect(result.current.pendingCommands).toHaveLength(1);
    expect(result.current.pendingCommands[0]?.command).toBe('FORCE_SPEAKER');
    expect(result.current.commandStatus.get(dispatchId!)).toBe('pending');
  });

  it('updates commandStatus to delivered when a WS status update arrives', async () => {
    const { result } = renderHookWithQueryClient(() =>
      useCommandDispatch({ imei: 'dev-1' }),
    );
    await waitForState(() => useWebSocketStore.getState().isConnected);

    let dispatchId: string | undefined;
    await act(async () => {
      dispatchId = await result.current.sendCommand('RESET_AUDIO_HAL');
    });
    expect(result.current.commandStatus.get(dispatchId!)).toBe('pending');

    const socket = FakeWebSocket.instances[0]!;
    const statusSub = socket.sent
      .map((raw) => JSON.parse(raw) as { type: string; id?: string })
      .find((m) => m.type === 'subscribe' && m.id);
    expect(statusSub?.id).toBeDefined();

    act(() => {
      socket.receiveMessage({
        id: statusSub!.id,
        type: 'next',
        payload: {
          data: {
            dispatchId,
            commandId: 'cmd-1',
            deviceId: 'dev-1',
            command: 'RESET_AUDIO_HAL',
            status: 'delivered',
            createdAt: new Date().toISOString(),
          },
        },
      });
    });

    await waitFor(() => expect(result.current.commandStatus.get(dispatchId!)).toBe('delivered'));

    expect(result.current.pendingCommands).toHaveLength(0);
  });

  it('maps failed/cancelled status to failed', async () => {
    const { result } = renderHookWithQueryClient(() =>
      useCommandDispatch({ imei: 'dev-1' }),
    );
    await waitForState(() => useWebSocketStore.getState().isConnected);

    let dispatchId: string | undefined;
    await act(async () => {
      dispatchId = await result.current.sendCommand('TOGGLE_CAPTURE');
    });

    const socket = FakeWebSocket.instances[0]!;
    const statusSub = socket.sent
      .map((raw) => JSON.parse(raw) as { type: string; id?: string })
      .find((m) => m.type === 'subscribe' && m.id)!;

    act(() => {
      socket.receiveMessage({
        id: statusSub.id,
        type: 'next',
        payload: {
          data: {
            dispatchId,
            commandId: 'cmd-1',
            deviceId: 'dev-1',
            command: 'TOGGLE_CAPTURE',
            status: 'failed',
            createdAt: new Date().toISOString(),
          },
        },
      });
    });

    await waitFor(() => expect(result.current.commandStatus.get(dispatchId!)).toBe('failed'));
  });

  it('exposes the spec return shape (sendCommand/pendingCommands/commandStatus Map)', async () => {
    const { result } = renderHookWithQueryClient(() =>
      useCommandDispatch({ imei: 'dev-1' }),
    );
    await waitForState(() => useWebSocketStore.getState().isConnected);
    expect(typeof result.current.sendCommand).toBe('function');
    expect(Array.isArray(result.current.pendingCommands)).toBe(true);
    expect(result.current.commandStatus).toBeInstanceOf(Map);
  });
});
