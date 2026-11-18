import { http, HttpResponse } from 'msw';

const HealthEndpoint = '/health';

export function createConnectivityHandlers() {
  return [
    http.head(HealthEndpoint, () => new HttpResponse(null, { status: 200 })),
  ];
}
