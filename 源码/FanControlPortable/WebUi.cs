namespace FanControlPortable;

public static class WebUi
{
    public const string MainHtml = """
<!doctype html>
<html lang="zh-CN" class="dark">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>风扇控制便携版</title>
<style>
:root{
  --bg:#f4f7fb;--fg:#0f172a;--panel:#ffffff;--surface:#f8fafc;--surface-alt:#eef2f7;
  --muted:#475569;--border:#d7dee9;--primary:#2563eb;--primary-soft:rgba(37,99,235,.12);
  --good:#047857;--warn:#b45309;--danger:#dc2626;--shadow:0 18px 44px rgba(15,23,42,.08);
}
.dark{
  --bg:#0b111d;--fg:#eaf0f8;--panel:#111a2b;--surface:#0f1726;--surface-alt:#1b2638;
  --muted:#a7b4c6;--border:#2e3b52;--primary:#60a5fa;--primary-soft:rgba(96,165,250,.16);
  --good:#34d399;--warn:#fbbf24;--danger:#f87171;--shadow:0 22px 48px rgba(0,0,0,.28);
}
*{box-sizing:border-box}
html,body{width:100%;height:100%;margin:0;background:var(--bg);color:var(--fg);font-family:"Microsoft YaHei UI","Segoe UI",Arial,sans-serif;letter-spacing:0;-webkit-font-smoothing:antialiased}
body{overflow:hidden;user-select:none}
input,select{user-select:text}
button,input,select{font:inherit}
button{cursor:pointer}
button:focus-visible,input:focus-visible,select:focus-visible{outline:2px solid var(--primary);outline-offset:2px}
.app{height:100vh;display:flex;flex-direction:column;background:var(--bg)}
.body-shell{flex:1;min-height:0;display:flex;flex-direction:column}
.app.nav-side .body-shell{display:grid;grid-template-columns:190px minmax(0,1fr)}
.topbar{min-height:62px;display:flex;align-items:center;justify-content:space-between;gap:14px;padding:12px 16px;border-bottom:1px solid var(--border);background:rgba(15,23,42,.03)}
.brand{display:flex;align-items:center;gap:12px;min-width:0}
.mark{width:40px;height:40px;border-radius:13px;display:grid;place-items:center;background:linear-gradient(145deg,var(--primary),#22d3ee);color:#fff;font-weight:950;box-shadow:0 12px 24px var(--primary-soft);flex:0 0 auto}
.brand-text{min-width:0}
.brand-text b{display:block;font-size:16px;line-height:20px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.brand-text span{display:block;margin-top:2px;font-size:12px;line-height:16px;color:var(--muted);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.top-actions{display:flex;align-items:center;justify-content:flex-end;gap:10px;flex-wrap:wrap}
.ip-field{height:38px;display:grid;grid-template-columns:48px minmax(180px,250px);border:1px solid var(--border);border-radius:12px;background:var(--panel);overflow:hidden}
.ip-field label{height:100%;display:grid;place-items:center;background:var(--surface-alt);border-right:1px solid var(--border);color:var(--muted);font-size:12px;font-weight:900}
.input{width:100%;height:100%;border:0;background:transparent;color:var(--fg);padding:0 12px;outline:0}
.btn{height:38px;border:1px solid var(--border);border-radius:12px;background:var(--panel);color:var(--fg);padding:0 14px;font-size:13px;font-weight:800;display:inline-flex;align-items:center;justify-content:center;gap:8px;white-space:nowrap;transition:background-color .16s,border-color .16s,color .16s,transform .12s,box-shadow .16s}
.btn:hover{background:var(--surface-alt);border-color:var(--primary)}
.btn:active{transform:translateY(1px)}
.btn.primary,.btn.active{background:var(--primary);border-color:var(--primary);color:#fff;box-shadow:0 10px 22px var(--primary-soft)}
.btn.danger.active,.btn.danger:hover{background:var(--danger);border-color:var(--danger);color:#fff}
.nav{display:flex;align-items:center;gap:8px;padding:10px 16px;border-bottom:1px solid var(--border);background:rgba(15,23,42,.02);overflow:auto;scrollbar-width:thin}
.nav-btn{height:38px;min-width:82px;border:1px solid var(--border);border-radius:12px;background:var(--panel);color:var(--muted);padding:0 14px;font-size:13px;font-weight:900;display:inline-flex;align-items:center;justify-content:center;white-space:nowrap;transition:background-color .16s,border-color .16s,color .16s}
.nav-btn:hover{background:var(--surface-alt);border-color:var(--primary);color:var(--fg)}
.nav-btn.active{background:var(--primary);border-color:var(--primary);color:#fff;box-shadow:0 10px 22px var(--primary-soft)}
.app.nav-side .nav{min-width:0;min-height:0;align-items:stretch;flex-direction:column;padding:14px 12px;border-right:1px solid var(--border);border-bottom:0;overflow:auto}
.app.nav-side .nav-btn{width:100%;justify-content:flex-start}
.scroll{flex:1;min-height:0;overflow:auto;scrollbar-width:thin;scrollbar-color:rgba(148,163,184,.55) transparent}
.app.nav-side .scroll{min-width:0}
.wrap{width:min(1220px,100%);margin:0 auto;padding:24px;display:grid;gap:24px}
.page{display:none;min-width:0}
.page.active{display:grid;gap:22px}
.hero,.card{border:1px solid var(--border);border-radius:20px;background:var(--panel);box-shadow:var(--shadow);overflow:hidden}
.hero{padding:24px}
.hero-top{display:flex;align-items:flex-start;justify-content:space-between;gap:14px;flex-wrap:wrap}
.hero h1{margin:0;font-size:26px;line-height:30px;font-weight:950;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.subtitle{margin-top:6px;color:var(--muted);font-size:13px;line-height:18px}
.chips{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
.chip{height:28px;padding:0 10px;border-radius:999px;border:1px solid var(--border);background:var(--surface-alt);color:var(--muted);display:inline-flex;align-items:center;gap:7px;font-size:12px;font-weight:900;white-space:nowrap}
.dot{width:7px;height:7px;border-radius:50%;background:currentColor}
.chip.good{color:var(--good);border-color:rgba(52,211,153,.36);background:rgba(52,211,153,.10)}
.chip.warn{color:var(--warn);border-color:rgba(251,191,36,.36);background:rgba(251,191,36,.10)}
.chip.danger{color:var(--danger);border-color:rgba(248,113,113,.36);background:rgba(248,113,113,.10)}
.stats{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:16px;margin-top:22px}
.stat{min-width:0;border:1px solid var(--border);border-radius:16px;background:var(--surface);padding:18px;overflow:hidden}
.stat span{display:block;color:var(--muted);font-size:12px;font-weight:900;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.stat b{display:block;margin-top:9px;font-size:28px;line-height:31px;font-weight:950;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.stat small{display:block;margin-top:6px;color:var(--muted);font-size:12px;line-height:16px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.value.good{color:var(--good)}.value.warn{color:var(--warn)}.value.danger{color:var(--danger)}.value.primary{color:var(--primary)}
.layout{display:grid;grid-template-columns:1fr;gap:22px}
.card-head{padding:20px 22px;border-bottom:1px solid var(--border);display:flex;align-items:center;justify-content:space-between;gap:14px;flex-wrap:wrap}
.card-title{min-width:0}
.card-title b{display:block;font-size:15px;line-height:20px;font-weight:950;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.card-title span{display:block;margin-top:2px;font-size:12px;color:var(--muted);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.card-body{padding:22px;display:grid;gap:18px}
.mode-row,.quick-row{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px}
.source-row{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}
.nav-choice{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}
.btn.wide{width:100%}
.speed-preset.active{background:var(--primary);border-color:var(--primary);color:#fff;box-shadow:0 10px 22px var(--primary-soft)}
.speed-card{border:1px solid var(--border);border-radius:16px;background:var(--surface);padding:18px;display:grid;gap:16px}
.speed-head{display:flex;align-items:center;justify-content:space-between;gap:12px}
.speed-head span{color:var(--muted);font-size:12px;font-weight:900}
.speed-value{width:92px;border:0;background:transparent;color:var(--primary);font-size:27px;font-weight:950;text-align:right;outline:0}
.range{width:100%;height:26px;accent-color:var(--primary)}
.mini-list{display:grid;gap:13px}
.mini-row{display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:center;gap:12px;min-height:32px}
.mini-row span:first-child{color:var(--muted);font-size:12px;font-weight:900}
.mini-row span:last-child{min-width:0;font-size:13px;font-weight:900;text-align:right;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.message-box{border:1px solid var(--border);border-radius:16px;background:var(--surface);padding:16px;color:var(--muted);font-size:12px;line-height:1.75;white-space:pre-wrap;word-break:break-word}
.curve-wrap{border:1px solid var(--border);border-radius:16px;background:linear-gradient(180deg,var(--surface-alt),var(--surface));overflow:hidden}
.curve{width:100%;height:310px;display:block;touch-action:none}
.curve-toolbar{display:flex;align-items:center;gap:9px;flex-wrap:wrap}
.curve-input{height:40px;width:100%;border:1px solid var(--border);border-radius:12px;background:var(--surface);color:var(--fg);padding:0 12px;outline:0;font-weight:700}
.point-list{display:grid;gap:8px}
.point-item{display:grid;grid-template-columns:minmax(0,1fr) auto auto;align-items:center;gap:8px;border:1px solid var(--border);border-radius:12px;background:var(--surface);padding:9px 10px}
.point-item b{font-size:13px;line-height:17px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.point-item small{display:block;margin-top:2px;color:var(--muted);font-size:11px;line-height:15px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.point-item.active{border-color:var(--primary);box-shadow:0 0 0 2px var(--primary-soft)}
.point-chip{height:26px;padding:0 9px;border-radius:999px;border:1px solid var(--border);background:var(--surface-alt);color:var(--muted);font-size:11px;font-weight:900;display:inline-flex;align-items:center;justify-content:center}
.help-card{border:1px solid var(--border);border-radius:16px;background:var(--surface);padding:18px;display:grid;gap:14px}
.help-head b{display:block;font-size:14px;line-height:18px;font-weight:950;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.help-head span{display:block;margin-top:2px;color:var(--muted);font-size:12px;line-height:16px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.sensor-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px}
.field{min-width:0;display:grid;gap:9px}
.field label{color:var(--muted);font-size:12px;font-weight:900}
.select{width:100%;height:40px;border:1px solid var(--border);border-radius:12px;background:var(--panel);color:var(--fg);padding:0 11px;outline:0;font-weight:800}
.sensor-current{min-height:16px;color:var(--muted);font-size:11px;line-height:16px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.help-list{display:grid;gap:12px}
.help-item{display:grid;grid-template-columns:26px minmax(0,1fr);gap:12px;align-items:flex-start}
.help-num{width:26px;height:26px;border-radius:9px;background:var(--surface-alt);border:1px solid var(--border);display:grid;place-items:center;color:var(--primary);font-size:12px;font-weight:950;flex:0 0 auto}
.help-item b{display:block;font-size:12px;line-height:16px;font-weight:900;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.help-item small{display:block;margin-top:2px;color:var(--muted);font-size:11px;line-height:15px;white-space:normal;word-break:break-word}
.toggle-row{min-height:66px;display:flex;align-items:center;justify-content:space-between;gap:16px;border:1px solid var(--border);border-radius:16px;background:var(--surface);padding:16px}
.toggle-row span:first-child{min-width:0}
.toggle-row b{display:block;font-size:13px;line-height:18px}
.toggle-row small{display:block;margin-top:2px;color:var(--muted);font-size:12px;line-height:16px;word-break:break-word}
.toggle-row input{display:none}
.switch{width:44px;height:26px;border-radius:999px;background:rgba(148,163,184,.38);position:relative;flex:0 0 auto;transition:background-color .18s}
.switch:after{content:"";position:absolute;top:4px;left:4px;width:18px;height:18px;border-radius:50%;background:#fff;transition:transform .18s}
.toggle-row input:checked+.switch{background:var(--primary)}
.toggle-row input:checked+.switch:after{transform:translateX(18px)}
.perf-note{border:1px solid rgba(251,191,36,.38);border-radius:14px;background:rgba(251,191,36,.10);color:var(--warn);padding:12px 14px;font-size:12px;line-height:20px}
.muted{color:var(--muted)}
@media (max-width:1080px){
  .topbar{align-items:stretch;flex-direction:column}
  .top-actions{justify-content:stretch}.ip-field{grid-template-columns:48px minmax(0,1fr);flex:1}
  .stats{grid-template-columns:repeat(2,minmax(0,1fr))}
  .app.nav-side .body-shell{display:flex;flex-direction:column}
  .app.nav-side .nav{flex-direction:row;align-items:center;padding:10px 16px;border-right:0;border-bottom:1px solid var(--border)}
  .app.nav-side .nav-btn{width:auto;justify-content:center}
}
@media (max-width:680px){
  .wrap{padding:16px;gap:18px}.page.active{gap:18px}.stats{grid-template-columns:1fr 1fr}.mode-row,.quick-row,.source-row,.nav-choice,.sensor-grid{grid-template-columns:1fr}
  .hero h1{font-size:22px}.top-actions{display:grid;grid-template-columns:1fr 1fr}.top-actions .btn{width:100%}.ip-field{width:100%;grid-column:1/-1}
  .nav{padding:9px 12px}.nav-btn{min-width:72px;padding:0 10px}
}
</style>
</head>
<body>
<div class="app">
  <header class="topbar">
    <div class="brand">
      <div class="mark">F</div>
      <div class="brand-text">
        <b>风扇控制便携版</b>
        <span>温度监控、曲线控制、远程散热器下发</span>
      </div>
    </div>
    <div class="top-actions">
      <div class="ip-field">
        <label for="deviceIp">IP</label>
        <input id="deviceIp" class="input" spellcheck="false" placeholder="例如 192.168.1.20 或 192.168.1.20:端口">
      </div>
      <button class="btn primary" onclick="saveIp()">保存并应用</button>
      <button class="btn" onclick="testIp()">测试连接</button>
    </div>
  </header>

  <div class="body-shell">
    <nav id="appNav" class="nav" aria-label="主导航">
      <button id="nav-dashboard" class="nav-btn active" onclick="setPage('dashboard')">控制台</button>
      <button id="nav-settings" class="nav-btn" onclick="setPage('settings')">设置</button>
    </nav>
  <main class="scroll">
    <div class="wrap">
      <section id="page-dashboard" class="page active">
      <section class="hero">
        <div class="hero-top">
          <div>
            <h1>实时散热控制</h1>
            <div id="summaryText" class="subtitle">正在等待硬件温度与散热器状态...</div>
          </div>
          <div class="chips">
            <span id="onlineChip" class="chip"><span class="dot"></span><span id="onlineText">设备状态</span></span>
            <span id="modeChip" class="chip">只监控</span>
            <span id="sourceChip" class="chip">CPU 基准</span>
          </div>
        </div>
        <div class="stats">
          <article class="stat">
            <span>CPU 温度</span>
            <b id="cpuVal" class="value">--</b>
            <small id="cpuDetail">--</small>
          </article>
          <article class="stat">
            <span>GPU 温度</span>
            <b id="gpuVal" class="value">--</b>
            <small id="gpuDetail">--</small>
          </article>
          <article class="stat">
            <span>控制温度</span>
            <b id="controlVal" class="value primary">--</b>
            <small id="controlDetail">--</small>
          </article>
          <article class="stat">
            <span>风扇转速</span>
            <b id="fanVal" class="value">--</b>
            <small id="fanDetail">--</small>
          </article>
        </div>
      </section>

      <section class="layout">
        <section class="card">
          <div class="card-head">
            <div class="card-title">
              <b>运行控制</b>
              <span>切换模式、手动转速和温度基准</span>
            </div>
          </div>
          <div class="card-body">
            <div class="mode-row">
              <button id="mode-monitor" class="btn" onclick="setMode('monitor')">只监控</button>
              <button id="mode-manual" class="btn" onclick="setMode('manual')">手动</button>
              <button id="mode-auto" class="btn" onclick="setMode('auto')">自动</button>
              <button id="mode-off" class="btn danger" onclick="setMode('off')">关闭</button>
            </div>
            <div class="speed-card">
              <div class="speed-head">
                <span>手动转速</span>
                <input id="speedText" class="speed-value" type="number" min="0" max="100" onchange="setSpeed(this.value,true)">
              </div>
              <input id="speed" class="range" type="range" min="0" max="100" oninput="syncSpeed(this.value)" onchange="setSpeed(this.value,true)">
            </div>
            <div class="quick-row">
              <button id="speedPreset20" class="btn speed-preset" onclick="setSpeed(20,true)">20%</button>
              <button id="speedPreset45" class="btn speed-preset" onclick="setSpeed(45,true)">45%</button>
              <button id="speedPreset75" class="btn speed-preset" onclick="setSpeed(75,true)">75%</button>
              <button id="speedPreset100" class="btn speed-preset" onclick="setSpeed(100,true)">100%</button>
            </div>
          </div>
        </section>

        <aside class="card">
          <div class="card-head">
            <div class="card-title">
              <b>设备回传</b>
              <span>远程散热器当前状态</span>
            </div>
          </div>
          <div class="card-body">
            <div class="mini-list">
              <div class="mini-row"><span>连接状态</span><span id="deviceOnline">--</span></div>
              <div class="mini-row"><span>当前转速</span><span id="deviceCurrent">--</span></div>
              <div class="mini-row"><span>目标转速</span><span id="deviceTarget">--</span></div>
              <div class="mini-row"><span>设备目标</span><span id="deviceBackTarget">--</span></div>
              <div class="mini-row"><span>设备温度</span><span id="deviceTemp">--</span></div>
              <div class="mini-row"><span>设备模式</span><span id="deviceMode">--</span></div>
            </div>
            <div id="messageText" class="message-box">准备就绪</div>
          </div>
        </aside>
      </section>

      <section class="card">
        <div class="card-head">
          <div class="card-title">
            <b>风扇曲线</b>
            <span id="pointText">拖动控制点可微调曲线</span>
          </div>
          <div class="curve-toolbar">
            <button class="btn" onclick="addCurvePoint()">添加点</button>
            <button class="btn" onclick="removeCurvePoint()">删除点</button>
            <button class="btn" onclick="resetCurve()">重置</button>
            <button class="btn primary" onclick="saveCurve()">保存曲线</button>
          </div>
        </div>
        <div class="card-body">
          <div class="curve-wrap"><svg id="curve" class="curve"></svg></div>
          <input id="curveInput" class="curve-input" spellcheck="false" onchange="previewCurveFromInput()" placeholder="30:20,42:30,60:60,82:100">
          <div id="curveText" class="muted">--</div>
          <div id="pointList" class="point-list"></div>
        </div>
      </section>

      <section class="card">
        <div class="card-head">
          <div class="card-title">
            <b>使用说明</b>
            <span>常用流程和曲线编辑方式</span>
          </div>
        </div>
        <div class="card-body">
          <div class="help-list">
            <div class="help-item"><span class="help-num">1</span><div><b>填写散热器地址</b><small>支持 IP、IP:端口、http://IP:端口，保存后会立即应用。</small></div></div>
            <div class="help-item"><span class="help-num">2</span><div><b>选择温度基准</b><small>CPU、GPU 或最高温度会作为自动曲线的输入。</small></div></div>
            <div class="help-item"><span class="help-num">3</span><div><b>编辑风扇曲线</b><small>点击空白处添加点，拖动圆点调整；右键/双击圆点或在列表中点删除可减少控制点。</small></div></div>
            <div class="help-item"><span class="help-num">4</span><div><b>切换运行模式</b><small>只监控不会下发；手动固定转速；自动按曲线下发；关闭会下发 0%。</small></div></div>
          </div>
        </div>
      </section>
      </section>

      <section id="page-settings" class="page">
        <section class="card">
          <div class="card-head">
            <div class="card-title">
              <b>温度来源</b>
              <span>选择自动曲线使用的温度基准，并固定 CPU/GPU 传感器。</span>
            </div>
          </div>
          <div class="card-body">
            <div class="source-row">
              <button id="source-cpu" class="btn" onclick="setSource('cpu')">CPU</button>
              <button id="source-gpu" class="btn" onclick="setSource('gpu')">GPU</button>
              <button id="source-max" class="btn" onclick="setSource('max')">最高温度</button>
            </div>
            <div class="sensor-grid">
              <div class="field">
                <label for="cpuSensorSelect">CPU 温度来源</label>
                <select id="cpuSensorSelect" class="select" onchange="setTemperatureSensor('cpu',this.value)"></select>
                <div id="cpuSensorCurrent" class="sensor-current">当前：--</div>
              </div>
              <div class="field">
                <label for="gpuSensorSelect">GPU 温度来源</label>
                <select id="gpuSensorSelect" class="select" onchange="setTemperatureSensor('gpu',this.value)"></select>
                <div id="gpuSensorCurrent" class="sensor-current">当前：--</div>
              </div>
            </div>
          </div>
        </section>

        <section class="card">
          <div class="card-head">
            <div class="card-title">
              <b>导航位置</b>
              <span>选择导航常驻位置，窄窗口会自动切回顶部导航。</span>
            </div>
          </div>
          <div class="card-body">
            <div class="nav-choice">
              <button id="navPlacementTop" class="btn" onclick="setNavigationPlacement('top')">顶部导航</button>
              <button id="navPlacementSide" class="btn" onclick="setNavigationPlacement('side')">左侧导航</button>
            </div>
          </div>
        </section>

        <section class="card">
          <div class="card-head">
            <div class="card-title">
              <b>后台行为</b>
              <span>设置关闭窗口和启动后的默认状态。</span>
            </div>
          </div>
          <div class="card-body">
            <label class="toggle-row">
              <span><b>关闭时挂到托盘</b><small>点关闭按钮时保留后台控制</small></span>
              <input id="closeTray" type="checkbox" onchange="send({command:'toggleCloseToTray',value:this.checked})"><span class="switch"></span>
            </label>
            <label class="toggle-row">
              <span><b>启动后最小化</b><small>开机或手动启动后直接进入托盘</small></span>
              <input id="startMin" type="checkbox" onchange="send({command:'toggleStartMinimized',value:this.checked})"><span class="switch"></span>
            </label>
            <label class="toggle-row">
              <span><b>开机自启</b><small>登录 Windows 后自动启动；配合启动后最小化可直接进入托盘。</small></span>
              <input id="startWin" type="checkbox" onchange="send({command:'toggleStartWithWindows',value:this.checked})"><span class="switch"></span>
            </label>
          </div>
        </section>

        <section class="card">
          <div class="card-head">
            <div class="card-title">
              <b>优化性能</b>
              <span>后台运行时减少界面和内存占用</span>
            </div>
          </div>
          <div class="card-body">
            <label class="toggle-row">
              <span><b>后台释放界面引擎</b><small>提醒：下次打开窗口会重新加载界面，可能慢一点。</small></span>
              <input id="releaseWebView" type="checkbox" onchange="send({command:'toggleReleaseWebViewInBackground',value:this.checked})"><span class="switch"></span>
            </label>
            <label class="toggle-row">
              <span><b>后台压缩工作集</b><small>提醒：主要降低任务管理器显示占用，切回时可能轻微卡顿。</small></span>
              <input id="trimWorkingSet" type="checkbox" onchange="send({command:'toggleTrimWorkingSetInBackground',value:this.checked})"><span class="switch"></span>
            </label>
            <div class="perf-note">后台模式会使用 2 秒轮询，并只刷新已选 CPU/GPU 温度传感器；前台保持 1 秒刷新。</div>
          </div>
        </section>

        <section class="card">
          <div class="card-head">
            <div class="card-title">
              <b>配置操作</b>
              <span>导入、导出配置，或控制当前窗口。</span>
            </div>
          </div>
          <div class="card-body">
            <div class="quick-row">
              <button class="btn" onclick="send({command:'saveConfig'})">导出配置</button>
              <button class="btn" onclick="send({command:'loadConfig'})">导入配置</button>
              <button class="btn" onclick="send({command:'showWindow'})">显示窗口</button>
              <button class="btn danger" onclick="send({command:'exit'})">退出</button>
            </div>
          </div>
        </section>

        <section class="card">
          <div class="card-head">
            <div class="card-title">
              <b>诊断信息</b>
              <span>当前温度采集状态和传感器日志摘要。</span>
            </div>
          </div>
          <div class="card-body">
            <div id="diagnosticText" class="message-box">温度诊断信息会显示在这里。</div>
          </div>
        </section>

      </section>
    </div>
  </main>
  </div>
</div>

<script>
const DEFAULT_CURVE='30:20,36:22,42:26,48:34,54:44,60:56,66:70,74:86,82:100';
const webview=window.chrome&&window.chrome.webview?window.chrome.webview:null;
const $=id=>document.getElementById(id);
const clamp=(v,min,max)=>Math.max(min,Math.min(max,v));
let state=null,points=parseCurve(DEFAULT_CURVE),selectedPoint=null,dragging=false,currentPage='dashboard';

const labels={
  mode:{monitor:'只监控',manual:'手动',auto:'自动',off:'关闭'},
  source:{cpu:'CPU 基准',gpu:'GPU 基准',max:'最高温度'},
  ready:'准备就绪'
};

function send(payload){if(webview)webview.postMessage(payload)}
function modeLabel(mode){return labels.mode[mode]||labels.mode.monitor}
function sourceLabel(source){return labels.source[source]||labels.source.cpu}
function pct(value){return value==null?'--':`${Math.round(Number(value))}%`}
function temp(value){return value==null?'--':`${Math.round(Number(value))}°C`}
function tempTone(value){if(value==null)return '';if(value>=82)return 'danger';if(value>=72)return 'warn';return 'good'}
function setText(id,value){const el=$(id);if(el)el.textContent=value}
function setValueClass(id,value,base='value'){const tone=tempTone(value);$(id).className=base+(tone?' '+tone:'')}
function normalizeMode(mode){return ['manual','auto','off'].includes(mode)?mode:'monitor'}
function normalizeSource(source){return ['gpu','max'].includes(source)?source:'cpu'}
function normalizeSensorId(sensorId){return sensorId&&String(sensorId).trim()?String(sensorId).trim():'auto'}
function normalizeNavigationPlacement(value){return value==='side'?'side':'top'}

function setPage(page){
  currentPage=['dashboard','settings'].includes(page)?page:'dashboard';
  document.querySelectorAll('.page').forEach(el=>el.classList.toggle('active',el.id==='page-'+currentPage));
  ['dashboard','settings'].forEach(name=>{
    const btn=$('nav-'+name);
    if(btn)btn.classList.toggle('active',name===currentPage);
  });
  if(currentPage==='dashboard')requestAnimationFrame(()=>drawCurve());
}

function setNavigationPlacement(placement){
  placement=normalizeNavigationPlacement(placement);
  applyNavigationPlacement(placement);
  send({command:'setNavigationPlacement',placement});
}

function applyNavigationPlacement(placement){
  placement=normalizeNavigationPlacement(placement);
  const app=document.querySelector('.app');
  app.classList.toggle('nav-side',placement==='side');
  $('navPlacementTop')?.classList.toggle('active',placement==='top');
  $('navPlacementSide')?.classList.toggle('active',placement==='side');
}

function setMode(mode){
  renderModeButtons(mode);
  send({command:'setMode',mode});
}

function setSource(source){
  renderSourceButtons(source);
  send({command:'setSource',source});
}

function setTemperatureSensor(sensorKind,sensorId){
  send({command:'setTemperatureSensor',sensorKind,sensorId:normalizeSensorId(sensorId)});
}

function saveIp(){send({command:'saveIp',ip:$('deviceIp').value.trim()})}
function testIp(){send({command:'test',ip:$('deviceIp').value.trim()})}

function syncSpeed(value){
  const speed=clamp(parseInt(value,10)||0,0,100);
  $('speed').value=speed;
  $('speedText').value=speed;
  renderSpeedPresets(speed);
}

function setSpeed(value,forceMode){
  const speed=clamp(parseInt(value,10)||0,0,100);
  syncSpeed(speed);
  send({command:'setSpeed',speed,forceMode});
}

function renderSpeedPresets(speed){
  [20,45,75,100].forEach(value=>{
    $('speedPreset'+value)?.classList.toggle('active',Math.round(Number(speed))===value);
  });
}

function renderModeButtons(mode){
  mode=normalizeMode(mode);
  ['monitor','manual','auto','off'].forEach(name=>{
    $('mode-'+name).classList.toggle('active',name===mode);
  });
}

function renderSourceButtons(source){
  source=normalizeSource(source);
  ['cpu','gpu','max'].forEach(name=>{
    $('source-'+name).classList.toggle('active',name===source);
  });
}

function addOption(select,value,text,disabled=false){
  const option=document.createElement('option');
  option.value=value;
  option.textContent=text;
  option.disabled=disabled;
  select.appendChild(option);
}

function sensorOptionLabel(sensor){
  return `${sensor.name||sensor.id||'未知传感器'} · ${temp(sensor.value)}`;
}

function syncTemperatureSensorSelect(kind,sensors,selectedId,currentName,currentTemp){
  const select=$(kind+'SensorSelect');
  if(!select)return;
  sensors=Array.isArray(sensors)?sensors:[];
  selectedId=normalizeSensorId(selectedId);
  const firstId=sensors.length?normalizeSensorId(sensors[0].id):'__none';
  const effectiveId=selectedId==='auto'?firstId:selectedId;
  const matched=sensors.some(sensor=>normalizeSensorId(sensor.id)===effectiveId);

  if(document.activeElement!==select){
    select.innerHTML='';
    sensors.forEach(sensor=>addOption(select,normalizeSensorId(sensor.id),sensorOptionLabel(sensor)));
    if(!sensors.length)addOption(select,'__none','未检测到传感器',true);
    if(selectedId!=='auto'&&!matched)addOption(select,selectedId,'已保存但未检测到的传感器');
    select.value=matched?effectiveId:firstId;
  }

  const suffix=matched?'手动':'已回退';
  setText(kind+'SensorCurrent',`当前：${currentName||'--'} ${temp(currentTemp)}（${suffix}）`);
}

function parseCurve(text){
  const raw=(text&&String(text).trim())?String(text):DEFAULT_CURVE;
  const map=new Map();
  raw.split(',').forEach(item=>{
    const parts=item.split(':').map(v=>Number(String(v).trim()));
    if(parts.length!==2||Number.isNaN(parts[0])||Number.isNaN(parts[1]))return;
    map.set(Math.round(clamp(parts[0],20,95)),Math.round(clamp(parts[1],0,100)));
  });
  const list=[...map.entries()].map(([t,s])=>({t,s})).sort((a,b)=>a.t-b.t);
  if(list.length>=2)return list;
  return raw===DEFAULT_CURVE?[]:parseCurve(DEFAULT_CURVE);
}

function curveValue(){
  const map=new Map();
  points.forEach(point=>map.set(Math.round(clamp(point.t,20,95)),Math.round(clamp(point.s,0,100))));
  return [...map.entries()].sort((a,b)=>a[0]-b[0]).map(([t,s])=>`${t}:${s}`).join(',');
}

function previewCurveFromInput(){
  points=parseCurve($('curveInput').value);
  selectedPoint=null;
  $('curveInput').value=curveValue()||DEFAULT_CURVE;
  drawCurve();
}

function saveCurve(){
  previewCurveFromInput();
  send({command:'setCurve',curve:$('curveInput').value.trim()});
}

function resetCurve(){
  points=parseCurve(DEFAULT_CURVE);
  selectedPoint=null;
  $('curveInput').value=DEFAULT_CURVE;
  drawCurve();
  send({command:'resetCurve'});
}

function addCurvePoint(){
  const control=state?.fan?.controlTemp??55;
  const t=nextAvailableTemp(control);
  selectedPoint={t,s:speedAt(t)};
  points.push(selectedPoint);
  points.sort((a,b)=>a.t-b.t);
  $('curveInput').value=curveValue();
  drawCurve();
  send({command:'setCurve',curve:$('curveInput').value.trim()});
}

function removeCurvePoint(){
  if(points.length<=2)return;
  if(!selectedPoint)selectedPoint=points[points.length-1];
  points=points.filter(point=>point!==selectedPoint);
  selectedPoint=null;
  $('curveInput').value=curveValue();
  drawCurve();
  send({command:'setCurve',curve:$('curveInput').value.trim()});
}

function selectCurvePoint(index){
  points.sort((a,b)=>a.t-b.t);
  selectedPoint=points[index]||null;
  drawCurve();
}

function deleteCurvePoint(index){
  if(points.length<=2)return;
  points.sort((a,b)=>a.t-b.t);
  if(index<0||index>=points.length)return;
  const removed=points[index];
  points=points.filter(point=>point!==removed);
  if(selectedPoint===removed)selectedPoint=null;
  $('curveInput').value=curveValue();
  drawCurve();
  send({command:'setCurve',curve:$('curveInput').value.trim()});
}

function deleteNearestCurvePoint(event){
  const point=nearestPoint(event);
  if(!point||points.length<=2)return false;
  points=points.filter(candidate=>candidate!==point);
  if(selectedPoint===point)selectedPoint=null;
  $('curveInput').value=curveValue();
  drawCurve();
  send({command:'setCurve',curve:$('curveInput').value.trim()});
  return true;
}

function nextAvailableTemp(seed){
  let t=Math.round(clamp(seed,20,95));
  const used=()=>points.some(point=>Math.round(point.t)===t);
  while(used()&&t<95)t++;
  while(used()&&t>20)t--;
  return t;
}

function speedAt(t){
  const sorted=points.slice().sort((a,b)=>a.t-b.t);
  if(!sorted.length)return 45;
  if(t<=sorted[0].t)return sorted[0].s;
  if(t>=sorted[sorted.length-1].t)return sorted[sorted.length-1].s;
  for(let i=0;i<sorted.length-1;i++){
    const a=sorted[i],b=sorted[i+1];
    if(t>=a.t&&t<=b.t){
      const ratio=(t-a.t)/Math.max(1,b.t-a.t);
      return Math.round(a.s+ratio*(b.s-a.s));
    }
  }
  return 45;
}

function render(){
  if(!state)return;
  const settings=state.settings||{},hardware=state.hardware||{},fan=state.fan||{};
  const mode=normalizeMode(settings.mode);
  const source=normalizeSource(settings.source);
  const controlTemp=fan.controlTemp;
  const message=fan.message||labels.ready;
  const diagnostic=hardware.diagnostic||'温度采集正常。';

  if(document.activeElement!==$('deviceIp'))$('deviceIp').value=settings.ip||'';
  syncSpeed(settings.manualSpeed??45);
  $('closeTray').checked=!!settings.closeToTray;
  $('startMin').checked=!!settings.startMinimized;
  $('startWin').checked=!!settings.startWithWindows;
  $('releaseWebView').checked=!!settings.releaseWebViewInBackground;
  $('trimWorkingSet').checked=!!settings.trimWorkingSetInBackground;
  applyNavigationPlacement(settings.navigationPlacement);

  $('onlineChip').className='chip '+(fan.online?'good':'danger');
  setText('onlineText',fan.online?'设备在线':'设备离线');
  setText('modeChip',modeLabel(mode));
  setText('sourceChip',sourceLabel(source));
  setText('summaryText',`${message} | 目标 ${pct(fan.target)} | 控制温度 ${temp(controlTemp)}`);

  setText('cpuVal',temp(hardware.cpuTemp));
  setText('gpuVal',temp(hardware.gpuTemp));
  setText('controlVal',temp(controlTemp));
  setText('fanVal',pct(fan.current));
  setValueClass('cpuVal',hardware.cpuTemp);
  setValueClass('gpuVal',hardware.gpuTemp);
  setValueClass('controlVal',controlTemp,'value primary');
  $('fanVal').className='value '+(fan.online?'good':'danger');
  setText('cpuDetail',hardware.cpuSensor||diagnostic);
  setText('gpuDetail',hardware.gpuSensor||diagnostic);
  setText('controlDetail',sourceLabel(source));
  setText('fanDetail',`目标 ${pct(fan.target)} / ${modeLabel(mode)}`);

  setText('deviceOnline',fan.online?'在线':'离线');
  setText('deviceCurrent',pct(fan.current));
  setText('deviceTarget',pct(fan.target));
  setText('deviceBackTarget',pct(fan.deviceTarget));
  setText('deviceTemp',temp(fan.deviceTemp));
  setText('deviceMode',fan.deviceMode||'--');
  setText('messageText',message);
  setText('diagnosticText',diagnostic);

  renderModeButtons(mode);
  renderSourceButtons(source);
  syncTemperatureSensorSelect('cpu',hardware.cpuSensors,settings.cpuSensorId,hardware.cpuSensor,hardware.cpuTemp);
  syncTemperatureSensorSelect('gpu',hardware.gpuSensors,settings.gpuSensorId,hardware.gpuSensor,hardware.gpuTemp);

  if(!dragging&&document.activeElement!==$('curveInput')){
    $('curveInput').value=settings.curve||DEFAULT_CURVE;
    points=parseCurve(settings.curve);
    selectedPoint=null;
  }
  drawCurve();
}

function drawCurve(){
  const svg=$('curve');
  const box=svg.getBoundingClientRect();
  const w=Math.max(320,box.width),h=Math.max(220,box.height);
  if(w<120||h<120)return;
  svg.setAttribute('viewBox',`0 0 ${w} ${h}`);
  svg.innerHTML='';
  const styles=getComputedStyle(document.documentElement);
  const grid=styles.getPropertyValue('--border').trim();
  const fg=styles.getPropertyValue('--muted').trim();
  const primary=styles.getPropertyValue('--primary').trim();
  const danger=styles.getPropertyValue('--danger').trim();
  const pad={l:52,r:22,t:24,b:38};
  const x=t=>pad.l+(t-20)*(w-pad.l-pad.r)/75;
  const y=s=>h-pad.b-s*(h-pad.t-pad.b)/100;
  const make=name=>document.createElementNS('http://www.w3.org/2000/svg',name);
  const attr=(el,props)=>{for(const key in props)el.setAttribute(key,props[key]);return el};
  const append=el=>svg.append(el);
  const line=(x1,y1,x2,y2,color,width=1,dash='')=>{
    const el=attr(make('line'),{x1,y1,x2,y2,stroke:color,'stroke-width':width});
    if(dash)el.setAttribute('stroke-dasharray',dash);
    append(el);
  };
  const text=(x,y,value,color=fg,size=12)=>{
    const el=attr(make('text'),{x,y,fill:color,'font-size':size,'font-family':'Microsoft YaHei UI, Segoe UI, Arial'});
    el.textContent=value;
    append(el);
  };
  for(let s=0;s<=100;s+=25){line(pad.l,y(s),w-pad.r,y(s),grid);text(12,y(s)+4,`${s}%`)}
  for(let t=30;t<=90;t+=15){line(x(t),pad.t,x(t),h-pad.b,grid);text(x(t)-16,h-14,`${t}°C`)}
  const sorted=points.slice().sort((a,b)=>a.t-b.t);
  if(sorted.length){
    const d=sorted.map((p,i)=>(i?'L':'M')+x(p.t).toFixed(1)+' '+y(p.s).toFixed(1)).join(' ');
    const area=d+` L ${x(sorted[sorted.length-1].t).toFixed(1)} ${h-pad.b} L ${x(sorted[0].t).toFixed(1)} ${h-pad.b} Z`;
    append(attr(make('path'),{d:area,fill:primary,opacity:.12}));
    append(attr(make('path'),{d,fill:'none',stroke:primary,'stroke-width':4,'stroke-linecap':'round','stroke-linejoin':'round'}));
    sorted.forEach(point=>{
      append(attr(make('circle'),{cx:x(point.t),cy:y(point.s),r:point===selectedPoint?8:6,fill:point===selectedPoint?'#fff':primary,stroke:primary,'stroke-width':3}));
    });
  }
  const current=state?.fan?.controlTemp;
  if(current!=null){
    const cx=x(clamp(current,20,95));
    line(cx,pad.t,cx,h-pad.b,danger,2,'6 6');
    text(cx+8,pad.t+20,`${Math.round(current)}°C`,danger,13);
  }
  setText('pointText',selectedPoint?`当前控制点：${Math.round(selectedPoint.t)}°C / ${Math.round(selectedPoint.s)}%`:'拖动控制点可微调曲线');
  setText('curveText',curveValue()||DEFAULT_CURVE);
  renderPointList();
}

function renderPointList(){
  const host=$('pointList');
  if(!host)return;
  if(!points.length){
    host.innerHTML='';
    return;
  }
  const sorted=points.slice().sort((a,b)=>a.t-b.t);
  host.innerHTML=sorted.map((point,index)=>{
    const active=point===selectedPoint?' active':'';
    return `<div class="point-item${active}"><div><b>${Math.round(point.t)}°C / ${Math.round(point.s)}%</b><small>第 ${index+1} 个控制点</small></div><button class="point-chip" onclick="selectCurvePoint(${index})">选中</button><button class="point-chip" onclick="deleteCurvePoint(${index})">删除</button></div>`;
  }).join('');
}

function curveGeometry(){
  const svg=$('curve'),box=svg.getBoundingClientRect(),w=Math.max(320,box.width),h=Math.max(220,box.height);
  const pad={l:52,r:22,t:24,b:38};
  return {
    x:t=>pad.l+(t-20)*(w-pad.l-pad.r)/75,
    y:s=>h-pad.b-s*(h-pad.t-pad.b)/100,
    point:e=>({
      t:Math.round(clamp(20+(e.clientX-box.left-pad.l)*75/(w-pad.l-pad.r),20,95)),
      s:Math.round(clamp(100-(e.clientY-box.top-pad.t)*100/(h-pad.t-pad.b),0,100))
    })
  };
}

function nearestPoint(event){
  const geo=curveGeometry(),point=geo.point(event);
  let best=null,bestDistance=14;
  points.forEach(candidate=>{
    const distance=Math.abs(candidate.t-point.t)+Math.abs(candidate.s-point.s)/2;
    if(distance<bestDistance){best=candidate;bestDistance=distance}
  });
  return best;
}

$('curve').addEventListener('pointerdown',event=>{
  if(event.button===2)return;
  selectedPoint=nearestPoint(event);
  if(!selectedPoint){
    selectedPoint=curveGeometry().point(event);
    points.push(selectedPoint);
    points.sort((a,b)=>a.t-b.t);
  }
  dragging=true;
  $('curve').setPointerCapture(event.pointerId);
  drawCurve();
});

$('curve').addEventListener('pointermove',event=>{
  if(!dragging||!selectedPoint)return;
  const point=curveGeometry().point(event);
  selectedPoint.t=point.t;
  selectedPoint.s=point.s;
  points.sort((a,b)=>a.t-b.t);
  $('curveInput').value=curveValue();
  drawCurve();
});

function endCurveDrag(){
  if(!dragging)return;
  dragging=false;
  $('curveInput').value=curveValue();
  send({command:'setCurve',curve:$('curveInput').value.trim()});
}

$('curve').addEventListener('pointerup',endCurveDrag);
$('curve').addEventListener('pointercancel',endCurveDrag);
$('curve').addEventListener('dblclick',event=>{deleteNearestCurvePoint(event);});
$('curve').addEventListener('contextmenu',event=>{event.preventDefault();deleteNearestCurvePoint(event);});
window.addEventListener('keydown',event=>{
  if(event.target&&['INPUT','TEXTAREA'].includes(event.target.tagName))return;
  if(event.key==='Delete' || event.key==='Backspace'){
    if(selectedPoint){
      event.preventDefault();
      deleteCurvePoint(points.indexOf(selectedPoint));
    }
  }
});
new ResizeObserver(()=>drawCurve()).observe($('curve'));

if(webview){
  webview.addEventListener('message',event=>{
    if(event.data?.type==='state'){state=event.data;render()}
    if(event.data?.type==='notice'){setText('messageText',event.data.message||labels.ready)}
  });
  send({command:'ready'});
}else{
  state={
    type:'state',
    settings:{ip:'192.168.1.20:8080',mode:'auto',source:'max',cpuSensorId:'/intelcpu/0/temperature/16',gpuSensorId:'/gpu-nvidia/0/temperature/0',manualSpeed:45,curve:DEFAULT_CURVE,closeToTray:true,startMinimized:false,startWithWindows:false,releaseWebViewInBackground:true,trimWorkingSetInBackground:true,navigationPlacement:'top'},
    hardware:{
      cpuTemp:61,cpuSensor:'CPU Package',gpuTemp:68,gpuSensor:'GPU Core',diagnostic:'预览模式：硬件读取正常。',
      cpuSensors:[{id:'/intelcpu/0/temperature/16',name:'Intel Core Ultra 5 225H / CPU Package',value:61},{id:'/intelcpu/0/temperature/13',name:'Intel Core Ultra 5 225H / P-Core #4',value:65}],
      gpuSensors:[{id:'/gpu-nvidia/0/temperature/0',name:'NVIDIA GeForce RTX 5060 Laptop GPU / GPU Core',value:68},{id:'/gpu-nvidia/0/temperature/3',name:'NVIDIA GeForce RTX 5060 Laptop GPU / GPU Memory Junction',value:72}]
    },
    fan:{online:true,current:45,target:56,deviceTemp:35,controlTemp:68,message:'预览模式'}
  };
  render();
}
</script>
</body>
</html>
""";

    public const string TrayHtml = """
<!doctype html>
<html lang="zh-CN" class="dark">
<head>
<meta charset="utf-8">
<style>
:root{--bg:#0b111d;--fg:#eaf0f8;--panel:#111a2b;--surface:#1b2638;--muted:#a7b4c6;--border:#2e3b52;--primary:#60a5fa;--good:#34d399;--danger:#f87171;--shadow:0 20px 48px rgba(0,0,0,.34)}
*{box-sizing:border-box}
html,body{width:100%;height:100%;margin:0;overflow:hidden;background:transparent;color:var(--fg);font-family:"Microsoft YaHei UI","Segoe UI",Arial,sans-serif;letter-spacing:0;-webkit-font-smoothing:antialiased;user-select:none}
button{font:inherit;cursor:pointer}
.panel{width:100vw;height:100vh;border:1px solid var(--border);border-radius:22px;background:linear-gradient(180deg,var(--panel),var(--bg));box-shadow:var(--shadow);padding:15px;display:grid;grid-template-rows:auto auto minmax(0,1fr);gap:12px}
.head{display:flex;align-items:center;justify-content:space-between;gap:10px;min-width:0}
.brand{display:flex;align-items:center;gap:10px;min-width:0}
.logo{width:34px;height:34px;border-radius:12px;display:grid;place-items:center;background:linear-gradient(145deg,var(--primary),#22d3ee);font-weight:950;color:#fff}
.title{min-width:0}.title b{display:block;font-size:14px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.title span{display:block;margin-top:2px;font-size:11px;color:var(--muted);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.chip{height:25px;padding:0 10px;border-radius:999px;border:1px solid var(--border);background:var(--surface);font-size:11px;font-weight:900;color:var(--muted);display:inline-flex;align-items:center}
.chip.good{color:var(--good);border-color:rgba(52,211,153,.34);background:rgba(52,211,153,.10)}.chip.danger{color:var(--danger);border-color:rgba(248,113,113,.34);background:rgba(248,113,113,.10)}
.status{height:70px;border:1px solid var(--border);border-radius:18px;background:var(--panel);padding:12px;display:grid;grid-template-columns:1fr auto;gap:12px;align-items:center}
.status span{display:block;font-size:12px;color:var(--muted);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.status strong{display:block;margin-top:5px;font-size:15px;color:var(--fg);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.target{font-size:28px;font-weight:950;color:var(--primary)}
.grid{display:grid;grid-template-columns:1fr 1fr;gap:9px;align-content:start}
.btn{height:39px;border:1px solid var(--border);border-radius:14px;background:var(--panel);color:var(--fg);font-size:13px;font-weight:850;transition:background-color .16s,border-color .16s}
.btn:hover{background:var(--surface);border-color:var(--primary)}
.btn.active,.btn.primary{background:var(--primary);border-color:var(--primary);color:white}.btn.danger.active{background:var(--danger);border-color:var(--danger)}.wide{grid-column:span 2}
</style>
</head>
<body>
<div class="panel">
  <div class="head">
    <div class="brand"><div class="logo">F</div><div class="title"><b>外置风扇</b><span id="sub">准备就绪</span></div></div>
    <span id="stateChip" class="chip">--</span>
  </div>
  <div class="status"><div><span id="line">--</span><strong id="modeText">--</strong></div><div id="target" class="target">--</div></div>
  <div class="grid">
    <button id="monitor" class="btn" onclick="cmd('setMode',{mode:'monitor'})">只监控</button>
    <button id="manual" class="btn" onclick="cmd('setSpeed',{speed:45,forceMode:true})">手动 45%</button>
    <button id="auto" class="btn" onclick="cmd('setMode',{mode:'auto'})">自动</button>
    <button id="off" class="btn danger" onclick="cmd('setMode',{mode:'off'})">关闭</button>
    <button class="btn" onclick="cmd('setSpeed',{speed:20,forceMode:true})">20%</button>
    <button class="btn" onclick="cmd('setSpeed',{speed:75,forceMode:true})">75%</button>
    <button class="btn wide primary" onclick="cmd('showWindow',{})">打开完整面板</button>
    <button class="btn wide" onclick="cmd('exit',{})">退出</button>
  </div>
</div>
<script>
const webview=window.chrome&&window.chrome.webview?window.chrome.webview:null;
const $=id=>document.getElementById(id);
const labels={monitor:'只监控',manual:'手动',auto:'自动',off:'关闭'};
function cmd(command,payload){if(webview)webview.postMessage(Object.assign({command},payload))}
function pct(value){return value==null?'--':`${Math.round(Number(value))}%`}
function modeLabel(mode){return labels[mode]||labels.monitor}
if(webview){
  webview.addEventListener('message',event=>{
    if(event.data?.type!=='state')return;
    const settings=event.data.settings||{},fan=event.data.fan||{};
    $('sub').textContent=fan.message||'准备就绪';
    $('line').textContent=`当前 ${pct(fan.current)} | 目标 ${pct(fan.target)}`;
    $('modeText').textContent=modeLabel(settings.mode);
    $('target').textContent=pct(fan.target);
    $('stateChip').textContent=fan.online?'在线':'离线';
    $('stateChip').className='chip '+(fan.online?'good':'danger');
    ['monitor','manual','auto','off'].forEach(name=>$(''+name).classList.toggle('active',(settings.mode||'monitor')===name));
  });
  cmd('ready',{});
}
</script>
</body>
</html>
""";
}
