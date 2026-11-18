import { describe, it, expect } from 'vitest';
import { deviceConfigFromWire, deployDriftFromWire } from '@vyzorix/api-client';
import type { CurrentResponse } from '@vyzorix/api-client';

describe('deployDriftFromWire', () => {
  it('maps every state the console knows', () => {
    for (const state of ['in_sync', 'unknown', 'behind', 'ahead', 'app_changed'] as const) {
      expect(deployDriftFromWire({ state }).state).toBe(state);
    }
  });

  it('defaults an unrecognised state to unknown, never to healthy', () => {

    expect(deployDriftFromWire({ state: 'something_new' }).state).toBe('unknown');
    expect(deployDriftFromWire(undefined).state).toBe('unknown');
    expect(deployDriftFromWire({}).state).toBe('unknown');
  });

  it('converts the reported-at epoch millis to a Date, and null to null', () => {
    expect(deployDriftFromWire({ reportedAt: 1_767_225_600_000 }).reportedAt).toEqual(
      new Date(1_767_225_600_000),
    );
    expect(deployDriftFromWire({}).reportedAt).toBeNull();
  });

  it('keeps a null reported version distinct from zero', () => {

    expect(deployDriftFromWire({}).reportedVersion).toBeNull();
    expect(deployDriftFromWire({ reportedVersion: 0 }).reportedVersion).toBe(0);
  });

  it('carries the detail sentence through', () => {
    const detail = 'app vfd: expected 3f2a…, running 9c11…';
    expect(deployDriftFromWire({ state: 'app_changed', detail }).detail).toBe(detail);
  });
});

describe('deviceConfigFromWire', () => {
  it('includes the drift verdict', () => {
    const wire: CurrentResponse = {
      deviceId: 'd_1',
      version: 7,
      kind: 'current',
      document: "' vyzorix.v",
      drift: { state: 'behind', detail: 'expected version 7, device holds 6' },
    };

    const snapshot = deviceConfigFromWire(wire);

    expect(snapshot.drift.state).toBe('behind');
    expect(snapshot.drift.detail).toContain('device holds 6');
  });

  it('reports unknown when the response carries no drift at all', () => {

    const snapshot = deviceConfigFromWire({ deviceId: 'd_1', version: 7 });

    expect(snapshot.drift.state).toBe('unknown');
  });
});
