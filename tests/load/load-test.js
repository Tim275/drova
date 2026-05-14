import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';
import { b64encode } from 'k6/encoding';

const errorRate = new Rate('errors');

// In CI (k3d on shared runner) latency is ~10s — use K6_DURATION_THRESHOLD to override.
// Default 500ms is the production target.
const durationThreshold = __ENV.K6_DURATION_THRESHOLD || 'p(95)<500';

export const options = {
  stages: [
    { duration: '10s', target: 10 },
    { duration: '20s', target: 20 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_duration: [durationThreshold],
    http_req_failed:   ['rate<0.01'],
    errors:            ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8081';
const TOKEN    = __ENV.E2E_TOKEN || '';

export default function () {
  const headers = TOKEN
    ? { Authorization: `Bearer ${TOKEN}` }
    : {};

  // Health endpoint — no auth
  const health = http.get(`${BASE_URL}/health`);
  check(health, { 'health 200': (r) => r.status === 200 });
  errorRate.add(health.status !== 200);

  // Login endpoint — Basic Auth
  const login = http.post(`${BASE_URL}/v1/auth/token`, null, {
    headers: { Authorization: 'Basic ' + b64encode('rider@drova.local:Test1234!') },
  });
  check(login, { 'login 200': (r) => r.status === 200 });
  errorRate.add(login.status !== 200);

  // Trip history — Bearer Auth
  if (TOKEN) {
    const history = http.get(`${BASE_URL}/trips/history`, { headers });
    check(history, { 'history 200': (r) => r.status === 200 });
    errorRate.add(history.status !== 200);
  }

  sleep(1);
}
