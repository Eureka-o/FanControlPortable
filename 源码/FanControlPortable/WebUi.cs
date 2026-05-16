namespace FanControlPortable;

public static class WebUi
{
    public const string TrayHtml = """
<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
html,body{margin:0;background:#10161f;color:#edf2f7;font-family:"Microsoft YaHei UI","Segoe UI",Arial,sans-serif}
.tray{padding:14px;display:grid;gap:12px}.status{height:70px;border:1px solid #2f3a49;border-radius:12px;background:#151b23;padding:12px}.status b{display:block;font-size:18px}.status span{display:block;margin-top:4px;color:#a9b4c3;font-size:12px}.row{display:grid;grid-template-columns:repeat(3,1fr);gap:8px}.btn{height:36px;border:1px solid #2f3a49;border-radius:10px;background:#151b23;color:#edf2f7;font-weight:850}.btn:hover{border-color:#58a6ff;background:#202b39}.danger:hover{border-color:#f87171;background:#f87171;color:#1f0707}
</style>
</head>
<body>
<div class="tray">
  <div class="status"><b id="modeText">FanControl</b><span id="speedText">等待状态</span></div>
  <div class="row">
    <button class="btn" onclick="cmd('setMode',{mode:'auto'})">自动</button>
    <button class="btn" onclick="cmd('setSpeed',{speed:45,forceMode:true})">45%</button>
    <button class="btn danger" onclick="cmd('setMode',{mode:'off'})">关闭</button>
  </div>
</div>
<script>
const webview=window.chrome&&window.chrome.webview?window.chrome.webview:null;
function cmd(command,payload){if(webview)webview.postMessage(Object.assign({command},payload))}
if(webview){webview.addEventListener('message',event=>{const s=event.data?.settings||{},f=event.data?.fan||{};document.getElementById('modeText').textContent=s.mode||'monitor';document.getElementById('speedText').textContent=`当前 ${f.current??'--'}% · 目标 ${f.target??'--'}%`});webview.postMessage({command:'ready'})}
</script>
</body>
</html>
""";

    public const string MainHtml = """
<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>FanControlPortable</title>
<style>
:root{
  --bg:#0d1117;--panel:#151b23;--panel2:#1b2430;--line:#2f3a49;--text:#edf2f7;--muted:#a9b4c3;
  --blue:#58a6ff;--green:#36d399;--amber:#f6b94b;--red:#f87171;--cyan:#22d3ee;--shadow:0 22px 50px rgba(0,0,0,.28);
}
*{box-sizing:border-box}
html,body{height:100%;margin:0;background:var(--bg);color:var(--text);font-family:"Microsoft YaHei UI","Segoe UI",Arial,sans-serif;letter-spacing:0}
body{overflow:hidden;user-select:none} input,select{user-select:text} button,input,select{font:inherit}
.app{height:100vh;display:flex;flex-direction:column}
.top{min-height:64px;display:flex;align-items:center;justify-content:space-between;gap:12px;padding:12px 16px;border-bottom:1px solid var(--line);background:#111821}
.brand{display:flex;align-items:center;gap:12px;min-width:0}.logo{width:40px;height:40px;border-radius:10px;background:linear-gradient(135deg,var(--blue),var(--cyan));display:grid;place-items:center;font-weight:950;color:#fff}
.brand b{display:block;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.brand span{display:block;color:var(--muted);font-size:12px;margin-top:2px}
.top-actions{display:flex;gap:8px;align-items:center;flex-wrap:wrap;justify-content:flex-end}.ipbox{height:38px;display:grid;grid-template-columns:42px minmax(190px,270px);border:1px solid var(--line);border-radius:10px;overflow:hidden;background:var(--panel)}
.ipbox label{display:grid;place-items:center;color:var(--muted);background:var(--panel2);font-size:12px;font-weight:900}.input,.select{height:38px;border:1px solid var(--line);border-radius:10px;background:var(--panel);color:var(--text);padding:0 11px;outline:0}
.ipbox .input{border:0;border-radius:0}.btn{height:38px;border:1px solid var(--line);border-radius:10px;background:var(--panel);color:var(--text);padding:0 13px;font-weight:850;font-size:13px;display:inline-flex;align-items:center;justify-content:center;gap:7px;white-space:nowrap;cursor:pointer}
.btn:hover{border-color:var(--blue);background:#202b39}.btn.primary,.btn.active{background:var(--blue);border-color:var(--blue);color:#06101f}.btn.good{background:var(--green);border-color:var(--green);color:#06140f}.btn.danger:hover,.btn.danger.active{background:var(--red);border-color:var(--red);color:#1f0707}.btn:disabled{opacity:.48;cursor:default}
.nav{display:flex;gap:8px;padding:10px 16px;border-bottom:1px solid var(--line);background:#10161f}.nav button{min-width:86px}
.scroll{flex:1;overflow:auto}.wrap{width:min(1220px,100%);margin:0 auto;padding:20px;display:grid;gap:18px}.page{display:none}.page.active{display:grid;gap:18px}
.grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px}.grid.two{grid-template-columns:repeat(2,minmax(0,1fr))}.grid.three{grid-template-columns:repeat(3,minmax(0,1fr))}
.card{border:1px solid var(--line);border-radius:12px;background:var(--panel);box-shadow:var(--shadow);overflow:hidden}.head{padding:16px 18px;border-bottom:1px solid var(--line);display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}.head b{font-size:15px}.head span{display:block;color:var(--muted);font-size:12px;margin-top:2px}
.body{padding:18px;display:grid;gap:14px}.stat{border:1px solid var(--line);border-radius:12px;background:var(--panel2);padding:15px;min-width:0}.stat span{color:var(--muted);font-size:12px;font-weight:900}.stat b{display:block;margin-top:8px;font-size:25px;line-height:29px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.stat small{display:block;margin-top:4px;color:var(--muted);font-size:12px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.goodText{color:var(--green)}.warnText{color:var(--amber)}.badText{color:var(--red)}.blueText{color:var(--blue)}
.chip{height:28px;border:1px solid var(--line);border-radius:999px;padding:0 10px;display:inline-flex;align-items:center;gap:7px;color:var(--muted);font-size:12px;font-weight:900}.dot{width:7px;height:7px;border-radius:50%;background:currentColor}.chip.good{color:var(--green)}.chip.bad{color:var(--red)}.chip.warn{color:var(--amber)}
.mode-row,.quick-row,.preset-row{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.source-row{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px}
.speed{border:1px solid var(--line);border-radius:12px;background:var(--panel2);padding:15px;display:grid;gap:12px}.speed.locked{border-color:rgba(246,185,75,.55)}.speed-top{display:flex;justify-content:space-between;align-items:center}.speed-value{width:92px;border:0;background:transparent;color:var(--blue);font-size:26px;font-weight:950;text-align:right;outline:0}.range{width:100%;accent-color:var(--blue)}
.mini{display:grid;gap:10px}.mini div{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:10px}.mini span:first-child{color:var(--muted);font-size:12px;font-weight:900}.mini span:last-child{font-weight:900;text-align:right;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.box{border:1px solid var(--line);border-radius:12px;background:var(--panel2);padding:13px;color:var(--muted);font-size:12px;line-height:1.75;white-space:pre-wrap;word-break:break-word}.box.warn{border-color:rgba(246,185,75,.5);color:#ffd28a;background:rgba(246,185,75,.08)}
.fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.field{display:grid;gap:8px}.field label{color:var(--muted);font-size:12px;font-weight:900}.sensor-current{color:var(--muted);font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.toggle{min-height:62px;display:flex;justify-content:space-between;align-items:center;gap:12px;border:1px solid var(--line);border-radius:12px;background:var(--panel2);padding:14px}.toggle b{display:block}.toggle small{display:block;color:var(--muted);font-size:12px;margin-top:2px}.toggle input{display:none}.switch{width:44px;height:26px;background:#586272;border-radius:999px;position:relative}.switch:after{content:"";position:absolute;top:4px;left:4px;width:18px;height:18px;border-radius:50%;background:#fff;transition:.18s}.toggle input:checked+.switch{background:var(--blue)}.toggle input:checked+.switch:after{transform:translateX(18px)}
.curve-input{width:100%;height:40px;border:1px solid var(--line);border-radius:10px;background:var(--panel2);color:var(--text);padding:0 11px;outline:0}.hidden{display:none!important}
@media(max-width:900px){.grid,.grid.two,.grid.three,.fields{grid-template-columns:1fr 1fr}.top{align-items:stretch;flex-direction:column}.top-actions{justify-content:stretch}.ipbox{grid-template-columns:42px minmax(0,1fr);width:100%}}
@media(max-width:640px){.wrap{padding:14px}.grid,.grid.two,.grid.three,.fields,.mode-row,.quick-row,.preset-row,.source-row{grid-template-columns:1fr}.top-actions{display:grid;grid-template-columns:1fr 1fr}.ipbox{grid-column:1/-1}}
</style>
</head>
<body>
<div class="app">
  <header class="top">
    <div class="brand"><div class="logo">F</div><div><b id="appName">FanControlPortable</b><span id="appMeta">--</span></div></div>
    <div class="top-actions">
      <div class="ipbox"><label for="deviceIp">IP</label><input id="deviceIp" class="input" list="recentIpList" spellcheck="false" placeholder="192.168.1.20 或 IP:端口"></div>
      <datalist id="recentIpList"></datalist>
      <button class="btn primary" onclick="saveIp()">保存应用</button>
      <button class="btn" onclick="testIp()">测试连接</button>
      <button class="btn" onclick="checkUpdate()">检查更新</button>
    </div>
  </header>
  <nav class="nav">
    <button id="nav-dashboard" class="btn active" onclick="setPage('dashboard')">控制台</button>
    <button id="nav-settings" class="btn" onclick="setPage('settings')">设置</button>
  </nav>
  <main class="scroll"><div class="wrap">
    <section id="page-dashboard" class="page active">
      <section id="setupCard" class="card hidden">
        <div class="head"><div><b>首次设置</b><span>按顺序完成一次，以后不再自动显示</span></div><button class="btn" onclick="completeSetup()">不再提示</button></div>
        <div class="body grid">
          <div class="stat"><span>1. 连接散热器</span><b>填 IP</b><small>支持 IP 或 IP:端口</small></div>
          <div class="stat"><span>2. 测试连接</span><b>看状态</b><small>失败会给出原因</small></div>
          <div class="stat"><span>3. 选温度源</span><b>CPU/GPU</b><small>推荐 CPU Package / GPU Core</small></div>
          <div class="stat"><span>4. 选模式</span><b>自动</b><small>按曲线下发转速</small></div>
        </div>
      </section>

      <section class="card">
        <div class="head">
          <div><b>实时状态</b><span id="summaryText">等待数据</span></div>
          <div><span id="onlineChip" class="chip"><span class="dot"></span><span id="onlineText">设备状态</span></span> <span id="modeChip" class="chip">只监控</span> <span id="sourceChip" class="chip">CPU</span></div>
        </div>
        <div class="body grid">
          <article class="stat"><span>CPU 温度</span><b id="cpuVal">--</b><small id="cpuDetail">--</small></article>
          <article class="stat"><span>GPU 温度</span><b id="gpuVal">--</b><small id="gpuDetail">--</small></article>
          <article class="stat"><span>风扇速度</span><b id="fanVal">--</b><small id="fanDetail">--</small></article>
          <article class="stat"><span>上次下发</span><b id="lastCommandVal">--</b><small id="lastCommandDetail">目标 --</small></article>
        </div>
      </section>

      <section class="grid two">
        <section class="card">
          <div class="head"><div><b>运行控制</b><span>常用动作放在这里</span></div></div>
          <div class="body">
            <div class="mode-row">
              <button id="mode-monitor" class="btn" onclick="setMode('monitor')">只监控</button>
              <button id="mode-manual" class="btn" onclick="setMode('manual')">手动</button>
              <button id="mode-auto" class="btn" onclick="setMode('auto')">自动</button>
              <button id="mode-off" class="btn danger" onclick="setMode('off')">关闭</button>
            </div>
            <div id="speedCard" class="speed">
              <div class="speed-top"><span>手动转速</span><input id="speedText" class="speed-value" type="number" min="0" max="100" onchange="setSpeed(this.value,true)"></div>
              <input id="speed" class="range" type="range" min="0" max="100" oninput="beginSpeedDrag();syncSpeed(this.value)" onchange="endSpeedDrag(this.value,true)">
              <div id="speedLockText" class="box">--</div>
            </div>
            <div class="quick-row">
              <button id="speedPreset20" class="btn" onclick="setSpeed(20,true)">20%</button>
              <button id="speedPreset45" class="btn" onclick="setSpeed(45,true)">45%</button>
              <button id="speedPreset75" class="btn" onclick="setSpeed(75,true)">75%</button>
              <button id="speedPreset100" class="btn" onclick="setSpeed(100,true)">100%</button>
            </div>
          </div>
        </section>

        <section class="card">
          <div class="head"><div><b>设备回传</b><span>连接失败时看这里</span></div></div>
          <div class="body">
            <div class="mini">
              <div><span>连接状态</span><span id="deviceOnline">--</span></div>
              <div><span>当前转速</span><span id="deviceCurrent">--</span></div>
              <div><span>目标转速</span><span id="deviceTarget">--</span></div>
              <div><span>设备目标</span><span id="deviceBackTarget">--</span></div>
              <div><span>设备温度</span><span id="deviceTemp">--</span></div>
              <div><span>设备模式</span><span id="deviceMode">--</span></div>
            </div>
            <div id="messageText" class="box">准备就绪</div>
          </div>
        </section>
      </section>

      <section class="card">
        <div class="head"><div><b>曲线预设</b><span>先选预设，再按需要微调</span></div><button class="btn primary" onclick="saveCurve()">保存曲线</button></div>
        <div class="body">
          <div class="preset-row">
            <button class="btn" onclick="applyCurvePreset('quiet')">安静</button>
            <button class="btn" onclick="applyCurvePreset('balanced')">均衡</button>
            <button class="btn" onclick="applyCurvePreset('cooling')">强冷</button>
            <button class="btn" onclick="applyCurvePreset('game')">游戏</button>
          </div>
          <input id="curveInput" class="curve-input" spellcheck="false" placeholder="30:20,42:30,60:60,82:100">
          <div id="curveText" class="box">--</div>
        </div>
      </section>
    </section>

    <section id="page-settings" class="page">
      <section class="card">
        <div class="head"><div><b>温度来源</b><span>自动模式使用的温度基准</span></div></div>
        <div class="body">
          <div class="source-row">
            <button id="source-cpu" class="btn" onclick="setSource('cpu')">CPU</button>
            <button id="source-gpu" class="btn" onclick="setSource('gpu')">GPU</button>
            <button id="source-max" class="btn" onclick="setSource('max')">最高温度</button>
          </div>
          <div class="fields">
            <div class="field"><label for="cpuSensorSelect">CPU 温度源</label><select id="cpuSensorSelect" class="select" onchange="setTemperatureSensor('cpu',this.value)"></select><div id="cpuSensorCurrent" class="sensor-current">当前：--</div></div>
            <div class="field"><label for="gpuSensorSelect">GPU 温度源</label><select id="gpuSensorSelect" class="select" onchange="setTemperatureSensor('gpu',this.value)"></select><div id="gpuSensorCurrent" class="sensor-current">当前：--</div></div>
          </div>
        </div>
      </section>

      <section class="card">
        <div class="head"><div><b>更新</b><span id="updateSubtitle">当前版本 --</span></div><button class="btn primary" onclick="checkUpdate()">检查更新</button></div>
        <div class="body">
          <div id="updateText" class="box">推荐下载 Lite 包。Standard 包多 PawnIO 驱动安装器；Compat 包自带 .NET、WebView2/PawnIO 安装器。</div>
          <button id="openUpdateBtn" class="btn" onclick="openUpdate()">打开下载</button>
        </div>
      </section>

      <section class="card">
        <div class="head"><div><b>连接记录</b><span>最近成功保存/测试过的地址</span></div></div>
        <div class="body"><div id="recentIpButtons" class="quick-row"></div></div>
      </section>

      <section class="card">
        <div class="head"><div><b>后台行为</b><span>最小干扰，保持控制稳定</span></div></div>
        <div class="body">
          <label class="toggle"><span><b>关闭时挂到托盘</b><small>不打断后台控制</small></span><input id="closeTray" type="checkbox" onchange="send({command:'toggleCloseToTray',value:this.checked})"><span class="switch"></span></label>
          <label class="toggle"><span><b>启动后最小化</b><small>适合开机自启</small></span><input id="startMin" type="checkbox" onchange="send({command:'toggleStartMinimized',value:this.checked})"><span class="switch"></span></label>
          <label class="toggle"><span><b>开机自启</b><small>使用当前用户 Run 注册表</small></span><input id="startWin" type="checkbox" onchange="send({command:'toggleStartWithWindows',value:this.checked})"><span class="switch"></span></label>
          <label class="toggle"><span><b>后台释放界面引擎</b><small>下次打开窗口会重新加载界面</small></span><input id="releaseWebView" type="checkbox" onchange="send({command:'toggleReleaseWebViewInBackground',value:this.checked})"><span class="switch"></span></label>
          <label class="toggle"><span><b>后台裁剪工作集</b><small>降低任务管理器显示占用</small></span><input id="trimWorkingSet" type="checkbox" onchange="send({command:'toggleTrimWorkingSetInBackground',value:this.checked})"><span class="switch"></span></label>
        </div>
      </section>

      <section class="card">
        <div class="head"><div><b>控制条行为</b><span>避免误触改变模式</span></div></div>
        <div class="body grid two">
          <button id="speedBehaviorManualOnly" class="btn" onclick="setSpeedControlBehavior('manualOnly')">仅手动可调</button>
          <button id="speedBehaviorSwitch" class="btn" onclick="setSpeedControlBehavior('switchToManual')">拖动切手动</button>
        </div>
      </section>

      <section class="card">
        <div class="head"><div><b>配置与诊断</b><span>缓存文件夹是 WebView2 运行缓存，可保留</span></div></div>
        <div class="body">
          <div class="quick-row">
            <button class="btn" onclick="send({command:'saveConfig'})">导出配置</button>
            <button class="btn" onclick="send({command:'loadConfig'})">导入配置</button>
            <button class="btn" onclick="send({command:'showWindow'})">显示窗口</button>
            <button class="btn danger" onclick="send({command:'exit'})">退出</button>
          </div>
          <div id="diagnosticText" class="box">诊断信息显示在这里。</div>
        </div>
      </section>
    </section>
  </div></main>
</div>
<script>
const DEFAULT_CURVE='30:20,36:22,42:26,48:34,54:44,60:56,66:70,74:86,82:100';
const PRESETS={
  quiet:'35:20,48:24,60:34,72:52,82:75,90:100',
  balanced:'30:20,42:26,54:44,66:70,78:88,86:100',
  cooling:'30:35,42:45,54:60,66:78,76:92,84:100',
  game:'36:30,48:40,60:58,70:76,80:92,88:100'
};
const webview=window.chrome&&window.chrome.webview?window.chrome.webview:null;
const $=id=>document.getElementById(id);
let state=null,speedDragging=false,currentPage='dashboard';
const labels={mode:{monitor:'只监控',manual:'手动',auto:'自动',off:'关闭'},source:{cpu:'CPU 基准',gpu:'GPU 基准',max:'最高温度'}};
function send(payload){if(webview)webview.postMessage(payload)}
function clamp(v,min,max){return Math.max(min,Math.min(max,v))}
function pct(v){return v==null?'--':`${Math.round(Number(v))}%`}
function temp(v){return v==null?'--':`${Math.round(Number(v))}°C`}
function tone(v){if(v==null)return '';if(v>=82)return 'badText';if(v>=72)return 'warnText';return 'goodText'}
function setText(id,v){const el=$(id);if(el)el.textContent=v}
function normalizeMode(v){return ['manual','auto','off'].includes(v)?v:'monitor'}
function normalizeSource(v){return ['gpu','max'].includes(v)?v:'cpu'}
function normalizeBehavior(v){return v==='switchToManual'?'switchToManual':'manualOnly'}
function setPage(page){currentPage=page==='settings'?'settings':'dashboard';document.querySelectorAll('.page').forEach(el=>el.classList.toggle('active',el.id==='page-'+currentPage));['dashboard','settings'].forEach(n=>$('nav-'+n)?.classList.toggle('active',n===currentPage))}
function saveIp(){send({command:'saveIp',ip:$('deviceIp').value.trim()})}
function testIp(){send({command:'test',ip:$('deviceIp').value.trim()})}
function checkUpdate(){send({command:'checkUpdate'})}
function openUpdate(){send({command:'openUpdate'})}
function completeSetup(){send({command:'completeSetup'})}
function setMode(mode){renderMode(mode);send({command:'setMode',mode})}
function setSource(source){renderSource(source);send({command:'setSource',source})}
function setTemperatureSensor(sensorKind,sensorId){send({command:'setTemperatureSensor',sensorKind,sensorId:sensorId||'auto'})}
function setSpeedControlBehavior(behavior){renderBehavior(behavior);send({command:'setSpeedControlBehavior',behavior})}
function syncSpeed(v){const speed=clamp(parseInt(v,10)||0,0,100);$('speed').value=speed;$('speedText').value=speed;[20,45,75,100].forEach(x=>$('speedPreset'+x)?.classList.toggle('active',speed===x))}
function isSpeedLocked(){const s=state?.settings||{};return normalizeBehavior(s.speedControlBehavior)==='manualOnly'&&normalizeMode(s.mode)!=='manual'}
function setSpeed(v,forceMode){if(isSpeedLocked())return;const speed=clamp(parseInt(v,10)||0,0,100);syncSpeed(speed);send({command:'setSpeed',speed,forceMode})}
function beginSpeedDrag(){speedDragging=true}
function endSpeedDrag(v,forceMode){speedDragging=false;setSpeed(v,forceMode)}
function saveCurve(){send({command:'setCurve',curve:$('curveInput').value.trim()||DEFAULT_CURVE})}
function applyCurvePreset(name){$('curveInput').value=PRESETS[name]||DEFAULT_CURVE;saveCurve()}
function useRecentIp(ip){$('deviceIp').value=ip;testIp()}
function renderMode(mode){mode=normalizeMode(mode);['monitor','manual','auto','off'].forEach(n=>$('mode-'+n)?.classList.toggle('active',n===mode))}
function renderSource(source){source=normalizeSource(source);['cpu','gpu','max'].forEach(n=>$('source-'+n)?.classList.toggle('active',n===source))}
function renderBehavior(behavior){behavior=normalizeBehavior(behavior);$('speedBehaviorManualOnly')?.classList.toggle('active',behavior==='manualOnly');$('speedBehaviorSwitch')?.classList.toggle('active',behavior==='switchToManual')}
function renderSpeedState(mode,behavior){const locked=normalizeBehavior(behavior)==='manualOnly'&&normalizeMode(mode)!=='manual';$('speedCard')?.classList.toggle('locked',locked);['speed','speedText','speedPreset20','speedPreset45','speedPreset75','speedPreset100'].forEach(id=>{const el=$(id);if(el)el.disabled=locked});setText('speedLockText',locked?'当前模式锁定手动转速，切到手动后可调。':'可调整手动转速。')}
function renderSensor(kind,sensors,selected,currentName,currentTemp){const select=$(kind+'SensorSelect');if(!select)return;sensors=Array.isArray(sensors)?sensors:[];if(document.activeElement!==select){select.innerHTML='';if(!sensors.length){const o=document.createElement('option');o.textContent='未检测到传感器';o.value='auto';select.appendChild(o)}sensors.forEach(sensor=>{const o=document.createElement('option');o.value=sensor.id||'auto';o.textContent=`${sensor.name||sensor.id} · ${temp(sensor.value)}`;select.appendChild(o)});select.value=selected&&selected!=='auto'?selected:(sensors[0]?.id||'auto')}setText(kind+'SensorCurrent',`当前：${currentName||'--'} ${temp(currentTemp)}`)}
function renderRecentIps(list){const data=$('recentIpList'),host=$('recentIpButtons');data.innerHTML='';host.innerHTML='';(Array.isArray(list)?list:[]).forEach(ip=>{const opt=document.createElement('option');opt.value=ip;data.appendChild(opt);const btn=document.createElement('button');btn.className='btn';btn.textContent=ip;btn.onclick=()=>useRecentIp(ip);host.appendChild(btn)})}
function renderUpdate(update,app){setText('updateSubtitle',`当前 ${app?.version||'--'} · ${app?.edition||'--'} · 推荐 ${app?.packageFile||'--'}`);if(!update){setText('updateText','推荐下载 Lite 包。Standard 包多 PawnIO 驱动安装器；Compat 包自带 .NET、WebView2/PawnIO 安装器。');return}setText('updateText',`${update.message}\n当前：${update.currentVersion||'--'}\n最新：${update.latestVersion||'--'}\n包：${update.assetName||app?.packageFile||'--'}\n\n${update.notes||''}`)}
function render(){
  if(!state)return;
  const s=state.settings||{},h=state.hardware||{},f=state.fan||{},app=state.app||{},update=state.update;
  const mode=normalizeMode(s.mode),source=normalizeSource(s.source),message=f.message||'准备就绪';
  setText('appName',app.name||'FanControlPortable');setText('appMeta',`${app.version||''} · ${app.edition||''}`);
  $('setupCard')?.classList.toggle('hidden',!!s.firstRunSetupComplete);
  if(document.activeElement!==$('deviceIp'))$('deviceIp').value=s.ip||'';
  renderRecentIps(s.recentIps);
  if(!speedDragging&&document.activeElement!==$('speed')&&document.activeElement!==$('speedText'))syncSpeed(s.manualSpeed??45);
  ['closeTray','startMin','startWin','releaseWebView','trimWorkingSet'].forEach(id=>{const el=$(id);if(el){const key={closeTray:'closeToTray',startMin:'startMinimized',startWin:'startWithWindows',releaseWebView:'releaseWebViewInBackground',trimWorkingSet:'trimWorkingSetInBackground'}[id];el.checked=!!s[key]}});
  $('onlineChip').className='chip '+(f.online?'good':'bad');setText('onlineText',f.online?'在线':'离线');setText('modeChip',labels.mode[mode]);setText('sourceChip',labels.source[source]);
  setText('summaryText',`${message} · 目标 ${pct(f.target)} · 控制温度 ${temp(f.controlTemp)}`);
  setText('cpuVal',temp(h.cpuTemp));$('cpuVal').className=tone(h.cpuTemp);setText('gpuVal',temp(h.gpuTemp));$('gpuVal').className=tone(h.gpuTemp);setText('fanVal',pct(f.current));$('fanVal').className=f.online?'goodText':'badText';setText('lastCommandVal',f.lastCommandAt||'--');
  setText('cpuDetail',h.cpuSensor||'--');setText('gpuDetail',h.gpuSensor||'--');setText('fanDetail',`目标 ${pct(f.target)} / ${labels.mode[mode]}`);setText('lastCommandDetail',`目标 ${pct(f.target)}`);
  setText('deviceOnline',f.online?'在线':'离线');setText('deviceCurrent',pct(f.current));setText('deviceTarget',pct(f.target));setText('deviceBackTarget',pct(f.deviceTarget));setText('deviceTemp',temp(f.deviceTemp));setText('deviceMode',f.deviceMode||'--');
  setText('messageText',message);setText('diagnosticText',h.diagnostic||'温度采集正常。');
  if(document.activeElement!==$('curveInput'))$('curveInput').value=s.curve||DEFAULT_CURVE;setText('curveText',$('curveInput').value);
  renderMode(mode);renderSource(source);renderBehavior(s.speedControlBehavior);renderSpeedState(mode,s.speedControlBehavior);
  renderSensor('cpu',h.cpuSensors,s.cpuSensorId,h.cpuSensor,h.cpuTemp);renderSensor('gpu',h.gpuSensors,s.gpuSensorId,h.gpuSensor,h.gpuTemp);renderUpdate(update,app);
}
if(webview){webview.addEventListener('message',event=>{if(event.data?.type==='state'){state=event.data;render()}if(event.data?.type==='notice'){setText('messageText',event.data.message||'准备就绪')}if(event.data?.type==='navigate'){setPage(event.data.page)}});send({command:'ready'})}
else{state={settings:{ip:'192.168.137.2',recentIps:['192.168.137.2'],mode:'auto',source:'cpu',manualSpeed:45,curve:DEFAULT_CURVE,speedControlBehavior:'manualOnly',closeToTray:true,releaseWebViewInBackground:true,trimWorkingSetInBackground:true},app:{name:'FanControlPortable Lite',version:'1.1.0',edition:'Lite',packageFile:'FanControlPortable-lite.zip'},hardware:{cpuTemp:61,cpuSensor:'CPU Package',gpuTemp:66,gpuSensor:'GPU Core',diagnostic:'预览模式',cpuSensors:[{id:'cpu',name:'CPU Package',value:61}],gpuSensors:[{id:'gpu',name:'GPU Core',value:66}]},fan:{online:true,current:45,target:56,controlTemp:61,lastCommandAt:'12:30:00',message:'状态正常'}};render()}
</script>
</body>
</html>
""";
}
