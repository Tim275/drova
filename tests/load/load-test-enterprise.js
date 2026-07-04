import http from 'k6/http';
import { check } from 'k6';
import { Rate, Trend } from 'k6/metrics';
import { b64encode } from 'k6/encoding';

// ── Enterprise-Lastmodell ──────────────────────────────────────────────────
// 1.000.000 Nutzer/Tag ÷ 86.400 s ≈ 12 Nutzer/s (Schnitt)
//   × ~7 Requests/Reise           ≈ 80 req/s (Schnitt)
//   × 4 (Abend-Peak Ridesharing)  ≈ 320 req/s (Peak)
//
// Modelliert wird die ANKUNFTSRATE (req/s), nicht eine fixe VU-Zahl — das ist
// der Unterschied zwischen Smoke und echtem Last-Test. ramping-arrival-rate
// hält die Rate konstant und zieht VUs bei Bedarf aus dem Pool nach.
// Ziel per TARGET_RPS dialbar (Default 300 = ~1 Mio Nutzer/Tag im Peak).
//
// ENABLE_BOOKING=1 schaltet den Schreib-Pfad zu (Preview → CreateTrip). Bewusst
// AUS by default: erzeugt echte Trips in der DB und braucht MAPBOX_TOKEN server-
// seitig — nur gegen eine Dev-/Wegwerf-DB laufen lassen.

const TARGET_RPS = Number(__ENV.TARGET_RPS || 300);
const BASE_URL   = __ENV.BASE_URL   || 'http://localhost:8081';
const RIDER      = __ENV.RIDER_USER || 'rider@drova.local';
const RIDER_PW   = __ENV.RIDER_PW   || 'Test1234!';
const BOOKING    = (__ENV.ENABLE_BOOKING || '') === '1';

const errorRate    = new Rate('errors');
const loginTrend   = new Trend('login_duration', true);
const historyTrend = new Trend('history_duration', true);
const nearbyTrend  = new Trend('nearby_duration', true);
const previewTrend = new Trend('preview_duration', true);
const createTrend  = new Trend('create_duration', true);

export const options = {
  scenarios: {
    enterprise_peak: {
      executor: 'ramping-arrival-rate',
      startRate: 0,
      timeUnit: '1s',
      preAllocatedVUs: 100,
      maxVUs: 2000,
      stages: [
        { target: TARGET_RPS,                    duration: '1m' }, // Ramp-up
        { target: TARGET_RPS,                    duration: '3m' }, // Peak halten
        { target: Math.round(TARGET_RPS * 1.5),  duration: '1m' }, // Stress-Spike
        { target: TARGET_RPS,                    duration: '2m' }, // erholt es sich?
        { target: 0,                             duration: '1m' }, // Abfall
      ],
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed:   ['rate<0.01'],
    errors:            ['rate<0.01'],
    login_duration:    ['p(95)<800'],   // Auth darf etwas langsamer sein
    history_duration:  ['p(95)<400'],
    nearby_duration:   ['p(95)<400'],
    preview_duration:  ['p(95)<1500'],  // ruft Mapbox → höhere Grenze
    create_duration:   ['p(95)<800'],
  },
};

function login() {
  const res = http.post(`${BASE_URL}/v1/auth/token`, null, {
    headers: { Authorization: 'Basic ' + b64encode(`${RIDER}:${RIDER_PW}`) },
    tags: { endpoint: 'login' },
  });
  loginTrend.add(res.timings.duration);
  const ok = check(res, { 'login 200': (r) => r.status === 200 });
  errorRate.add(!ok);
  return ok ? res.json('token') : null;
}

// Session-Token + userID pro VU: überleben Iterationen innerhalb desselben VU
// (realistisch — ein echter Nutzer loggt sich einmal ein, nicht bei jedem Klick).
let sessionToken = null;
let sessionUserID = null;
function token() {
  if (!sessionToken) sessionToken = login();
  return sessionToken;
}
function userID(tk) {
  if (sessionUserID) return sessionUserID;
  const res = http.get(`${BASE_URL}/v1/users/me`, {
    headers: { Authorization: `Bearer ${tk}` }, tags: { endpoint: 'me' },
  });
  if (res.status === 200) sessionUserID = String(res.json('id'));
  return sessionUserID;
}

// Schreib-Pfad: Preview (Fahrpreise + Mapbox-Route) → CreateTrip. Shapes exakt
// wie tests/e2e/smoke_test.go. Nur aktiv mit ENABLE_BOOKING=1.
function bookingJourney(tk) {
  const uid = userID(tk);
  if (!uid) return;

  const previewBody = JSON.stringify({
    userID: uid,
    pickup:      { latitude: 51.2277, longitude: 6.7735 },
    destination: { latitude: 51.2180, longitude: 6.7940 },
    pickupAddress:  'Königsallee, Düsseldorf',
    dropoffAddress: 'Hauptbahnhof, Düsseldorf',
  });
  const pv = http.post(`${BASE_URL}/trip/preview`, previewBody, {
    headers: { Authorization: `Bearer ${tk}`, 'Content-Type': 'application/json' },
    tags: { endpoint: 'preview' },
  });
  previewTrend.add(pv.timings.duration);
  const pvOk = check(pv, { 'preview 201': (r) => r.status === 201 });
  errorRate.add(!pvOk);
  if (!pvOk) return;

  const fares = pv.json('data.rideFares');
  if (!fares || fares.length === 0) return;
  const fareID = fares[0].id;

  const startBody = JSON.stringify({ rideFareID: fareID, userID: uid });
  const cr = http.post(`${BASE_URL}/trip/start`, startBody, {
    headers: { Authorization: `Bearer ${tk}`, 'Content-Type': 'application/json' },
    tags: { endpoint: 'create' },
  });
  createTrend.add(cr.timings.duration);
  errorRate.add(!check(cr, { 'create 201': (r) => r.status === 201 }));
}

export default function () {
  // Realistischer Ridesharing-Mix — lese-lastig wie echter Traffic.
  const dice = Math.random();

  // Schreib-Last: ~5% der Iterationen buchen wirklich (nur mit ENABLE_BOOKING=1).
  if (BOOKING && dice < 0.05) {
    const tk = token();
    if (tk) bookingJourney(tk);
    return;
  }

  if (dice < 0.40) {
    // 40% App-Health / Polling (ohne Auth)
    const res = http.get(`${BASE_URL}/health`, { tags: { endpoint: 'health' } });
    errorRate.add(!check(res, { 'health 200': (r) => r.status === 200 }));

  } else if (dice < 0.65) {
    // 25% Login (Auth-Last)
    login();

  } else if (dice < 0.85) {
    // 20% Fahrt-Historie (autorisierter Read → Postgres)
    const tk = token();
    if (tk) {
      const res = http.get(`${BASE_URL}/trips/history`, {
        headers: { Authorization: `Bearer ${tk}` }, tags: { endpoint: 'history' },
      });
      historyTrend.add(res.timings.duration);
      errorRate.add(!check(res, { 'history 200': (r) => r.status === 200 }));
    }

  } else {
    // 15% Fahrer in der Nähe (autorisierter Geo-Read → Redis GEOSEARCH)
    const tk = token();
    if (tk) {
      const res = http.get(`${BASE_URL}/drivers/nearby?lat=51.22&lng=6.77&radius_km=15`, {
        headers: { Authorization: `Bearer ${tk}` }, tags: { endpoint: 'nearby' },
      });
      nearbyTrend.add(res.timings.duration);
      errorRate.add(!check(res, { 'nearby 200': (r) => r.status === 200 }));
    }
  }
}
