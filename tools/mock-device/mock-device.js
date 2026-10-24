const http = require('http');

const host = process.env.MOCK_FAN_HOST || '127.0.0.1';
const port = Number(process.env.MOCK_FAN_PORT || 18080);
const maxBodyBytes = 4 * 1024 * 1024;

const state = {
  currentRpm: 1400,
  targetRpm: 1400,
  speed: 35,
  targetSpeed: 35,
  wifiTargetSpeed: 35,
  wifiControl: true,
  controlMode: 'wifi',
  mode: '自动模式(实时转速)',
  workMode: '自动模式(实时转速)',
  power: true,
  temperature: 43,
  gpuTemperature: 41,
  powerWatts: 9.5,
  gpuPowerWatts: 4.2,
  lighting: {
    enabled: true,
    mode: 'blackshark_breathing',
    brightness: 72,
    speed: 'medium',
    colors: [{ r: 0, g: 220, b: 255 }],
  },
  gearLight: { enabled: true, gear: 2, brightness: 72 },
  screen: {
    page: 'parameters',
    pages: ['image', 'parameters'],
    width: 428,
    height: 142,
    image: null,
  },
  updatedAt: new Date().toISOString(),
  postCount: 0,
  lastPayload: null,
};

function finiteNumber(value) {
  const next = Number(value);
  return Number.isFinite(next) ? next : null;
}

function clampRpm(value) {
  const next = finiteNumber(value);
  return next === null ? null : Math.max(0, Math.min(4000, Math.round(next)));
}

function clampPercent(value) {
  const next = finiteNumber(value);
  return next === null ? null : Math.max(0, Math.min(100, Math.round(next)));
}

function rpmToPercent(rpm) {
  return Math.round((rpm / 4000) * 100);
}

function percentToRpm(percent) {
  return Math.round((percent / 100) * 4000);
}

function updateSimulatedState() {
  if (state.currentRpm < state.targetRpm) state.currentRpm += Math.min(180, state.targetRpm - state.currentRpm);
  if (state.currentRpm > state.targetRpm) state.currentRpm -= Math.min(180, state.currentRpm - state.targetRpm);
  state.speed = rpmToPercent(state.currentRpm);
  state.targetSpeed = rpmToPercent(state.targetRpm);
  state.wifiTargetSpeed = state.targetSpeed;

  const wobble = Math.sin(Date.now() / 8000) * 1.2;
  state.temperature = Math.round((38 + state.currentRpm * 0.012 + wobble) * 10) / 10;
  state.gpuTemperature = Math.round((36 + state.currentRpm * 0.009 + wobble * 0.6) * 10) / 10;
  state.powerWatts = Math.round((4.5 + state.currentRpm * 0.006) * 10) / 10;
  state.gpuPowerWatts = Math.round((2.5 + state.currentRpm * 0.003) * 10) / 10;
  state.updatedAt = new Date().toISOString();
}

function json(res, status, body) {
  const payload = JSON.stringify(body, null, 2);
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'GET,POST,PUT,OPTIONS',
    'Access-Control-Allow-Headers': 'Content-Type',
    'Cache-Control': 'no-store',
  });
  res.end(payload);
}

function readBody(req, limit = maxBodyBytes) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    let size = 0;
    req.on('data', (chunk) => {
      size += chunk.length;
      if (size > limit) {
        reject(new Error(`request body too large (limit ${limit} bytes)`));
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => resolve(Buffer.concat(chunks)));
    req.on('error', reject);
  });
}

async function readJSON(req) {
  const body = await readBody(req, 256 * 1024);
  if (!body.length) return {};
  return JSON.parse(body.toString('utf8'));
}

function updateSpeed(payload) {
  const rpmValue = payload.rpm ?? payload.targetRpm ?? payload.currentRpm;
  const percentValue = payload.percent ?? payload.targetPercent;
  let rpm = rpmValue !== undefined ? clampRpm(rpmValue) : null;
  if (rpm === null && percentValue !== undefined) {
    const percent = clampPercent(percentValue);
    rpm = percent === null ? null : percentToRpm(percent);
  }
  if (rpm === null && payload.speed !== undefined) {
    const speed = finiteNumber(payload.speed);
    rpm = speed === null ? null : (speed > 100 ? clampRpm(speed) : percentToRpm(clampPercent(speed)));
  }
  if (rpm === null) return null;
  state.targetRpm = rpm;
  state.wifiControl = true;
  state.controlMode = 'wifi';
  state.mode = payload.workMode || payload.mode || state.mode;
  state.workMode = state.mode;
  state.postCount += 1;
  state.lastPayload = payload;
  state.updatedAt = new Date().toISOString();
  return rpm;
}

function mergeLighting(payload) {
  if (typeof payload.enabled === 'boolean') state.lighting.enabled = payload.enabled;
  if (typeof payload.mode === 'string' && payload.mode.trim()) state.lighting.mode = payload.mode.trim();
  if (typeof payload.speed === 'string' && payload.speed.trim()) state.lighting.speed = payload.speed.trim();
  if (Array.isArray(payload.colors)) state.lighting.colors = payload.colors;
  const brightness = clampPercent(payload.brightness);
  if (brightness !== null) state.lighting.brightness = brightness;
  if (typeof payload.gearLight === 'boolean') state.gearLight.enabled = payload.gearLight;
  state.gearLight.brightness = state.lighting.brightness;
}

function stateResponse() {
  updateSimulatedState();
  return {
    device: 'blackshark-brb02-wifi-mock',
    manufacturer: '黑鲨（Black Shark）',
    product: '黑鲨（Black Shark）风神 Pro',
    model: 'BRB02',
    transport: 'wifi',
    connected: true,
    speed: state.speed,
    fanSpeed: state.speed,
    targetSpeed: state.targetSpeed,
    wifiTargetSpeed: state.wifiTargetSpeed,
    currentRpm: state.currentRpm,
    targetRpm: state.targetRpm,
    currentSpeed: state.currentRpm,
    targetSpeedRpm: state.targetRpm,
    maxRpm: 4000,
    wifiControl: state.wifiControl,
    controlMode: state.controlMode,
    mode: state.mode,
    workMode: state.workMode,
    power: state.power,
    temperature: state.temperature,
    gpuTemperature: state.gpuTemperature,
    powerWatts: state.powerWatts,
    gpuPowerWatts: state.gpuPowerWatts,
    lighting: state.lighting,
    brightness: state.lighting.brightness,
    gearLight: state.gearLight,
    screen: state.screen,
    updatedAt: state.updatedAt,
  };
}

async function handle(req, res) {
  const url = new URL(req.url || '/', `http://${req.headers.host || `${host}:${port}`}`);
  const write = req.method === 'POST' || req.method === 'PUT';

  if (req.method === 'OPTIONS') {
    json(res, 204, {});
    return;
  }
  if (req.method === 'GET' && ['/api/data', '/api/status', '/api/state'].includes(url.pathname)) {
    json(res, 200, stateResponse());
    return;
  }
  if (write && ['/api/speed', '/api/rpm'].includes(url.pathname)) {
    try {
      const rpm = updateSpeed(await readJSON(req));
      if (rpm === null) return json(res, 400, { status: 'error', message: 'rpm/speed must be a number from 0 to 4000' });
      json(res, 200, { status: 'success', controlMode: 'wifi', currentRpm: state.currentRpm, targetRpm: rpm, speed: state.speed, targetSpeed: state.targetSpeed, postCount: state.postCount });
    } catch (error) { json(res, 400, { status: 'error', message: error.message || String(error) }); }
    return;
  }
  if (req.method === 'GET' && ['/api/mode', '/api/work-mode'].includes(url.pathname)) {
    json(res, 200, { mode: state.mode, workMode: state.workMode, controlMode: state.controlMode });
    return;
  }
  if (write && ['/api/mode', '/api/work-mode'].includes(url.pathname)) {
    try {
      const payload = await readJSON(req);
      const mode = payload.workMode || payload.mode;
      if (typeof mode !== 'string' || !mode.trim()) return json(res, 400, { status: 'error', message: 'mode must be a non-empty string' });
      state.mode = mode.trim(); state.workMode = state.mode; state.updatedAt = new Date().toISOString();
      json(res, 200, { status: 'success', mode: state.mode, workMode: state.workMode });
    } catch (error) { json(res, 400, { status: 'error', message: error.message || String(error) }); }
    return;
  }
  if (req.method === 'GET' && ['/api/lighting', '/api/light'].includes(url.pathname)) {
    json(res, 200, { ...state.lighting, gearLight: state.gearLight });
    return;
  }
  if (write && ['/api/lighting', '/api/light'].includes(url.pathname)) {
    try { mergeLighting(await readJSON(req)); json(res, 200, { status: 'success', lighting: state.lighting, gearLight: state.gearLight }); }
    catch (error) { json(res, 400, { status: 'error', message: error.message || String(error) }); }
    return;
  }
  if (write && ['/api/rgb-off', '/api/lighting/off'].includes(url.pathname)) {
    state.lighting.enabled = false;
    state.gearLight.enabled = false;
    state.postCount += 1;
    state.updatedAt = new Date().toISOString();
    json(res, 200, { status: 'success', lighting: state.lighting, gearLight: state.gearLight });
    return;
  }
  if (req.method === 'GET' && ['/api/brightness', '/api/lighting/brightness'].includes(url.pathname)) {
    json(res, 200, { brightness: state.lighting.brightness }); return;
  }
  if (write && ['/api/brightness', '/api/lighting/brightness'].includes(url.pathname)) {
    try {
      const payload = await readJSON(req); const brightness = clampPercent(payload.brightness ?? payload.value);
      if (brightness === null) return json(res, 400, { status: 'error', message: 'brightness must be a number from 0 to 100' });
      state.lighting.brightness = brightness; state.gearLight.brightness = brightness;
      json(res, 200, { status: 'success', brightness });
    } catch (error) { json(res, 400, { status: 'error', message: error.message || String(error) }); }
    return;
  }
  if (req.method === 'GET' && ['/api/gear-light', '/api/lighting/gear-light'].includes(url.pathname)) {
    json(res, 200, state.gearLight); return;
  }
  if (write && ['/api/gear-light', '/api/lighting/gear-light'].includes(url.pathname)) {
    try {
      const payload = await readJSON(req); if (typeof payload.enabled === 'boolean') state.gearLight.enabled = payload.enabled;
      const gear = finiteNumber(payload.gear); if (gear !== null) state.gearLight.gear = Math.max(0, Math.min(5, Math.round(gear)));
      const brightness = clampPercent(payload.brightness); if (brightness !== null) state.gearLight.brightness = brightness;
      json(res, 200, { status: 'success', gearLight: state.gearLight });
    } catch (error) { json(res, 400, { status: 'error', message: error.message || String(error) }); }
    return;
  }
  if (req.method === 'GET' && ['/api/screen', '/api/screen/status'].includes(url.pathname)) {
    json(res, 200, state.screen); return;
  }
  if (write && ['/api/screen', '/api/screen/page'].includes(url.pathname)) {
    try {
      const payload = await readJSON(req); if (payload.page !== 'image' && payload.page !== 'parameters') return json(res, 400, { status: 'error', message: 'page must be image or parameters' });
      state.screen.page = payload.page; json(res, 200, { status: 'success', screen: state.screen });
    } catch (error) { json(res, 400, { status: 'error', message: error.message || String(error) }); }
    return;
  }
  if (req.method === 'POST' && ['/api/screen/parameters', '/api/screen/host-info'].includes(url.pathname)) {
    try {
      const payload = await readJSON(req);
      if (typeof payload.frameBase64 !== 'string' || !payload.frameBase64.trim()) {
        return json(res, 400, { status: 'error', message: 'frameBase64 must be a non-empty string' });
      }
      const frame = Buffer.from(payload.frameBase64, 'base64');
      state.screen.page = 'parameters';
      state.screen.lastParameters = { bytes: frame.length, updatedAt: new Date().toISOString() };
      json(res, 200, { status: 'success', parameters: state.screen.lastParameters });
    } catch (error) { json(res, 400, { status: 'error', message: error.message || String(error) }); }
    return;
  }
  if (req.method === 'GET' && ['/api/screen/image', '/api/image'].includes(url.pathname)) {
    json(res, 200, state.screen.image || { uploaded: false, page: 'image' }); return;
  }
  if (req.method === 'POST' && ['/api/screen/image', '/api/image'].includes(url.pathname)) {
    try {
      const body = await readBody(req); state.screen.image = { uploaded: true, bytes: body.length, contentType: req.headers['content-type'] || 'application/octet-stream', uploadedAt: new Date().toISOString() }; state.screen.page = 'image';
      json(res, 200, { status: 'success', progress: 100, image: state.screen.image });
    } catch (error) { json(res, 413, { status: 'error', message: error.message || String(error) }); }
    return;
  }
  if (req.method === 'GET' && url.pathname === '/api/logs') { json(res, 200, stateResponse()); return; }
  if (write && url.pathname === '/api/heartbeat') { json(res, 200, { status: 'ok', updatedAt: state.updatedAt }); return; }
  json(res, 404, { status: 'error', message: `unknown route ${req.method} ${url.pathname}` });
}

function selfTest() {
  if (clampRpm(-1) !== 0 || clampRpm(5000) !== 4000 || percentToRpm(50) !== 2000 || rpmToPercent(4000) !== 100) throw new Error('speed helpers failed');
  const previous = state.targetRpm; if (updateSpeed({ rpm: 3200 }) !== 3200 || state.targetRpm !== 3200) throw new Error('RPM update failed'); state.targetRpm = previous;
  console.log('[mock-device] self-test passed');
}

if (process.argv.includes('--self-test')) {
  selfTest();
} else {
  const server = http.createServer((req, res) => handle(req, res).catch((error) => json(res, 500, { status: 'error', message: error.message || String(error) })));
  server.listen(port, host, () => {
    console.log(`[mock-device] listening on http://${host}:${port}`);
    console.log('[mock-device] Black Shark BRB02 WiFi preview: RPM 0-4000, lighting, gear-light, screen/image');
    console.log('[mock-device] configure FanControl device address as 127.0.0.1:18080');
  });
  function shutdown() { console.log('[mock-device] shutting down'); server.close(() => process.exit(0)); }
  process.on('SIGINT', shutdown); process.on('SIGTERM', shutdown);
}
