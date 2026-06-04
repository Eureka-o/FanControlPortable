const http = require('http');

const host = process.env.MOCK_FAN_HOST || '127.0.0.1';
const port = Number(process.env.MOCK_FAN_PORT || 18080);

const state = {
  speed: 35,
  targetSpeed: 35,
  wifiControl: true,
  controlMode: 'wifi',
  power: true,
  temperature: 42,
  updatedAt: new Date().toISOString(),
  postCount: 0,
  lastPayload: null,
};

function clampSpeed(value) {
  const next = Number(value);
  if (!Number.isFinite(next)) return null;
  return Math.max(0, Math.min(100, Math.round(next)));
}

function updateSimulatedState() {
  if (state.speed < state.targetSpeed) state.speed += Math.min(3, state.targetSpeed - state.speed);
  if (state.speed > state.targetSpeed) state.speed -= Math.min(3, state.speed - state.targetSpeed);
  const wobble = Math.sin(Date.now() / 8000) * 1.8;
  state.temperature = Math.round((39 + state.speed * 0.08 + wobble) * 10) / 10;
  state.updatedAt = new Date().toISOString();
}

function json(res, status, body) {
  const payload = JSON.stringify(body, null, 2);
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'GET,POST,OPTIONS',
    'Access-Control-Allow-Headers': 'Content-Type',
    'Cache-Control': 'no-store',
  });
  res.end(payload);
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    req.on('data', (chunk) => {
      chunks.push(chunk);
      if (Buffer.concat(chunks).length > 64 * 1024) {
        req.destroy(new Error('request body too large'));
      }
    });
    req.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
    req.on('error', reject);
  });
}

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url || '/', `http://${req.headers.host || `${host}:${port}`}`);

  if (req.method === 'OPTIONS') {
    json(res, 204, {});
    return;
  }

  if (req.method === 'GET' && url.pathname === '/api/data') {
    updateSimulatedState();
    console.log(`[mock-device] GET /api/data -> speed=${state.speed}% target=${state.targetSpeed}% temp=${state.temperature}`);
    json(res, 200, {
      speed: state.speed,
      fanSpeed: state.speed,
      wifiTargetSpeed: state.targetSpeed,
      targetSpeed: state.targetSpeed,
      wifiControl: state.wifiControl,
      controlMode: state.controlMode,
      mode: state.controlMode,
      power: state.power,
      temperature: state.temperature,
      updatedAt: state.updatedAt,
    });
    return;
  }

  if (req.method === 'POST' && url.pathname === '/api/speed') {
    try {
      const raw = await readBody(req);
      const payload = raw.trim() ? JSON.parse(raw) : {};
      const speed = clampSpeed(payload.speed);
      if (speed === null) {
        json(res, 400, { status: 'error', message: 'speed must be a number from 0 to 100' });
        return;
      }
      state.targetSpeed = speed;
      state.wifiControl = true;
      state.controlMode = 'wifi';
      state.postCount += 1;
      state.lastPayload = payload;
      state.updatedAt = new Date().toISOString();
      console.log(`[mock-device] POST /api/speed <- speed=${speed}% count=${state.postCount}`);
      json(res, 200, {
        status: 'success',
        controlMode: 'wifi',
        speed: state.speed,
        targetSpeed: state.targetSpeed,
        postCount: state.postCount,
      });
    } catch (error) {
      json(res, 400, { status: 'error', message: error.message || String(error) });
    }
    return;
  }

  if (req.method === 'GET' && url.pathname === '/api/logs') {
    json(res, 200, state);
    return;
  }

  json(res, 404, { status: 'error', message: `unknown route ${req.method} ${url.pathname}` });
});

server.listen(port, host, () => {
  console.log(`[mock-device] listening on http://${host}:${port}`);
  console.log('[mock-device] configure FanControlPortable device address as 127.0.0.1:18080');
});

function shutdown() {
  console.log('[mock-device] shutting down');
  server.close(() => process.exit(0));
}

process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);
