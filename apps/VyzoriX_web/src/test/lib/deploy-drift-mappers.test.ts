import { describe, it, expect } from 'vitest';
import { deviceConfigFromWire, deployDriftFromWire } from '@vyzorix/api-client';
import type { CurrentResponse } from '@vyzorix/api-client';

/**
 * The drift the device reports against the config the server expects.
 *
 * The mapper is where a server string becomes a verdict the UI renders, so the
 * fallback matters as much as the mapping: a state the console does not recognise
 * must read as `unknown`, never as healthy. A device that has never reported and a
 * device running the right thing must not look the same.
 */
describe('deployDriftFromWire', () => {
  it('maps every state the console knows', () => {
    for (const state of ['in_sync', 'unknown', 'behind', 'ahead', 'app_changed'] as const) {
      expect(deployDriftFromWire({ state }).state).toBe(state);
    }
  });

  it('defaults an unrecognised state to unknown, never to healthy', () => {
    // A server that grows a sixth state must not be rendered as in-sync by a console
    // that does not understand it.
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
    // null is "does not report"; 0 is "reports holding no config". The UI says
    // different things for each, so the mapper must not collapse them.
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
    // An older server, or a response that predates the field: unknown, not healthy.
    const snapshot = deviceConfigFromWire({ deviceId: 'd_1', version: 7 });

    expect(snapshot.drift.state).toBe('unknown');
  });
});
