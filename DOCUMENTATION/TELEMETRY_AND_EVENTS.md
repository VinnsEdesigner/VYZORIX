# Telemetry and device events

**Status:** shipped. Server: `apps/api/internal/domain/telemetry/`, the device telemetry
handler, `apps/api/internal/ws/telemetry_filter.go`, and the device-events handlers.
Device: `cloud-connector/.../telemetry/`.
**Related:** `DEVICE_ONBOARDING.md` (how the device got here),
`ALERTING.md` (threshold rules that read telemetry),
`WEBSOCKET_SYSTEM.md` (the transport telemetry arrives on).

This document explains how the server knows a device is healthy: the periodic telemetry
the device reports, and the discrete events it records.

---

## 1. Telemetry, in one paragraph

Telemetry is the device telling the server how it is. The device samples a few readings
on a cadence and sends them; the server stores them and drives its threshold alerts from
them. A device that never reports is **invisible** to the user — the dashboard shows
a device that is online with no telemetry at all, which reads as broken rather than as
"not reporting" — so the cadence and the frame are both deliberately simple and always
sent.

---

## 2. The cadence

The interval comes from the **config** (`vvv hb 60s`), not a constant, so the server can
slow reporting across devices without shipping a new APK. The device applies a new value
on the next tick. A value below the device's minimum is clamped: the server owns the
value, but a typo in a config must not turn every device into a request flood.

Reporting is also triggered on **every socket connect** — the same trigger as the update
check — so a device that has been offline reports as soon as it can reach the server. It
is not a poll loop.

---

## 3. The frame

The device sends a telemetry frame mirroring the server's `TelemetryFrame`:

| Field | Type | Meaning |
|:--|:--|:--|
| `type` | string | the frame type |
| `deviceId` | string | the reporting device |
| `timestamp` | any | when the reading was taken |
| `uptime` | int64 | device uptime, **seconds** |
| `thermalTemp` | float | battery temperature, °C |
| `riskScore` | int | reserved |
| `audioMode`, `bufferLevel`, `speakerOn` | int/int/bool | present for schema compatibility |

Two notes on the fields, both of which have caused confusion:

- **`uptime` is in seconds, not milliseconds.** The server's column and frame are
  seconds, so a millisecond reading would make every device look as if it had been up for
  decades.
- **`audioMode`, `bufferLevel` and `speakerOn` are reported as zeros.** This device has
  no audio engine — the design is a headless Debian agent, not an audio router, and
  nothing produces those readings. The server's schema carries the fields, so they are
  sent rather than omitted; a dashboard that shows `0` can tell "no audio engine on this
  device" from "device not reporting", whereas a fabricated number would hide which.

`thermalTemp` is a **battery** temperature, not a CPU one — Android exposes no
unprivileged CPU sensor — and it is the reading that matters most on hardware that
throttles under sustained load. It is reported without a permission. The device samples
it from `ACTION_BATTERY_CHANGED` and documents it as a battery temperature rather than
presenting it as something it is not.

---

## 4. What the server does with it

- **Stores** each frame, keyed by device, with the payload and the indexed readings.
- **Feeds threshold alerting.** Organization and device settings carry thermal warn/crit
  thresholds; the alerting engine evaluates telemetry against them. See
  `ALERTING.md`.
- **Streams it to the dashboard** over the live channel, filtered by
  `apps/api/internal/ws/telemetry_filter.go` so a subscriber receives the devices it is
  entitled to see.
- **Exposes history** for the diagnostics views.

The storage entity indexes `riskScore`, `bufferLevel`, `thermalTemp` and `uptime`, so a
dashboard query does not have to parse the payload.

---

## 5. Device events

Telemetry is periodic; events are discrete. A **device event** is a thing that happened:
a connection, a command, a state change, an error. Events are stored and are queryable
per device, by type, and by id:

| Endpoint | What it returns |
|:--|:--|
| `GET /dashboard/device/:imei/events` | events for one device |
| `GET /dashboard/events/recent` | the most recent events across devices |
| `GET /dashboard/events/types/:type` | events of one type |
| `GET /dashboard/events/:id` | one event |

All are gated on `events.read` scoped to devices. Events and telemetry are different
things with different lifetimes: telemetry is a time series you chart, events are a
ledger you search.

---

## 6. The honest limits

- **A device that is offline is invisible.** There is no server-side polling; telemetry
  is push-only, and the "no data" state is exactly the "device is not reporting" state.
  The 2-minute TTL on download progress is the same idea applied to a different signal.
- **The audio fields are structurally zero.** They exist for schema compatibility, and a
  future audio feature would fill them, not fake them.
- **`riskScore` is reserved.** The field is carried; nothing computes it yet.
