import ws from 'k6/ws';
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';
import { b64encode } from 'k6/encoding';

// ── WebSocket-Last ──────────────────────────────────────────────────────────
// Testet die WS-Verbindungs-Kapazität des api-gateway: viele Rider halten

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8081';
const WS_URL   = BASE_URL.replace(/^http/, 'ws');
const RIDER    = __ENV.RIDER_USER || 'rider@drova.local';
const RIDER_PW = __ENV.RIDER_PW   || 'Test1234!';
const WS_VUS   = Number(__ENV.WS_VUS || 50);
const HOLD_MS  = Number(__ENV.WS_HOLD_MS || 15000);   // Verbindung so lange halten

const wsConnect = new Rate('ws_connect_success');
const wsMsgRate = new Rate('ws_message_received');

export const options = {
  scenarios: {
    ws_hold: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '20s', target: WS_VUS },   // hochfahren
        { duration: '1m',  target: WS_VUS },    // halten
        { duration: '10s', target: 0 },         // abbauen
      ],
    },
  },
  thresholds: {
    ws_connect_success: ['rate>0.95'],   // >95% der Verbindungen müssen aufgehen
  },
};

// Token pro VU cachen (ein Nutzer loggt sich einmal ein)
let sessionToken = null;
function login() {
  const res = http.post(`${BASE_URL}/v1/auth/token`, null, {
    headers: { Authorization: 'Basic ' + b64encode(`${RIDER}:${RIDER_PW}`) },
  });
  return res.status === 200 ? res.json('token') : null;
}
function token() {
  if (!sessionToken) sessionToken = login();
  return sessionToken;
}

// Einmal-Ticket holen (single-use, 30s TTL) — hält den JWT aus der WS-URL raus
function getTicket(tk) {
  const res = http.get(`${BASE_URL}/ws/ticket`, {
    headers: { Authorization: `Bearer ${tk}` },
  });
  return res.status === 200 ? res.json('ticket') : null;
}

export default function () {
  const tk = token();
  if (!tk) { wsConnect.add(false); return; }

  const ticket = getTicket(tk);
  if (!ticket) { wsConnect.add(false); return; }

  const url = `${WS_URL}/ws/riders?ticket=${ticket}`;
  const res = ws.connect(url, {}, function (socket) {
    socket.on('open', () => {
      wsConnect.add(true);
      // Verbindung halten wie ein echter Rider, der auf Updates wartet
      socket.setTimeout(() => socket.close(), HOLD_MS);
    });
    socket.on('message', () => wsMsgRate.add(true));   // Fahrt-Events empfangen
    socket.on('error', () => wsConnect.add(false));
  });

  // 101 = erfolgreicher WS-Upgrade
  check(res, { 'ws upgrade 101': (r) => r && r.status === 101 });
  sleep(1);
}
