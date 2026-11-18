import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { http, HttpResponse } from 'msw';
import { useConnectivityStore } from '@/stores/connectivity-store';
import { getConnectivityMonitor } from '@vyzorix/api-client';

const { server } = setupIntegrationTest();

beforeEach(() => {

  Object.defineProperty(navigator, 'onLine', { configurable: true, value: true });
  window.dispatchEvent(new Event('online'));
  useConnectivityStore.setState({
    isChecking: false,
    isFlushing: false,
  });
});

afterEach(() => {
  Object.defineProperty(navigator, 'onLine', { configurable: true, value: true });
  window.dispatchEvent(new Event('online'));
});

describe('useConnectivityStore', () => {
  it('initializes with monitor state', () => {
    const state = useConnectivityStore.getState();
    expect(state.isOnline).toBe(true);
    expect(state.queueSize).toBe(0);
    expect(state.isChecking).toBe(false);
  });

  it('syncs when the network goes offline', () => {
    Object.defineProperty(navigator, 'onLine', {
      configurable: true,
      value: false,
    });
    window.dispatchEvent(new Event('offline'));
    expect(useConnectivityStore.getState().isOnline).toBe(false);
  });

  it('syncs back when the network comes online', () => {
    Object.defineProperty(navigator, 'onLine', { configurable: true, value: false });
    window.dispatchEvent(new Event('offline'));
    expect(useConnectivityStore.getState().isOnline).toBe(false);

    Object.defineProperty(navigator, 'onLine', { configurable: true, value: true });
    window.dispatchEvent(new Event('online'));
    expect(useConnectivityStore.getState().isOnline).toBe(true);
  });

  it('checkConnectivity toggles isChecking and returns true when healthy', async () => {
    const promise = useConnectivityStore.getState().checkConnectivity();
    expect(useConnectivityStore.getState().isChecking).toBe(true);
    const result = await promise;
    expect(result).toBe(true);
    expect(useConnectivityStore.getState().isChecking).toBe(false);
    expect(useConnectivityStore.getState().isOnline).toBe(true);
  });

  it('checkConnectivity reports offline when the health probe fails', async () => {
    server.use(http.head('/health', () => HttpResponse.json({}, { status: 503 })));
    const result = await useConnectivityStore.getState().checkConnectivity();
    expect(result).toBe(false);
    expect(useConnectivityStore.getState().isOnline).toBe(false);
  });

  it('checkConnectivity resets isChecking even when the probe errors', async () => {
    server.use(
      http.head('/health', () => HttpResponse.error()),
    );
    const result = await useConnectivityStore.getState().checkConnectivity();

    expect(result).toBe(navigator.onLine);
    expect(useConnectivityStore.getState().isChecking).toBe(false);
  });

  it('flushQueue toggles isFlushing', async () => {
    const promise = useConnectivityStore.getState().flushQueue();
    expect(useConnectivityStore.getState().isFlushing).toBe(true);
    await promise;
    expect(useConnectivityStore.getState().isFlushing).toBe(false);
  });

  it('clearQueue empties the monitor queue', () => {
    const monitor = getConnectivityMonitor();
    expect(monitor.getQueueSize()).toBe(0);
    useConnectivityStore.getState().clearQueue();
    expect(monitor.getQueueSize()).toBe(0);
  });
});
