// The host executes this file via `new Function(source)` and awaits the returned value.
// We must return the boot() Promise so the host waits until every asset has finished
// evaluating and registerPage() has actually been called — otherwise the host sees the
// script as "loaded" while boot() is still asynchronously fetching child assets, and
// falsely reports "plugin loaded but no page registered".
return (function () {
  "use strict";

  const PLUGIN_ID = "omen-fan";
  const ASSET_PATHS = [
    "ui/omen-core.js",
    "ui/omen-style.js",
    "ui/omen-components.js",
    "ui/omen-views.js",
    "ui/omen-app.js",
  ];
  const BOOTSTRAP_KEY = "__FanControlOmenFanPluginBootstrap";

  const state = window[BOOTSTRAP_KEY] = window[BOOTSTRAP_KEY] || { loading: false, loaded: false };
  if (state.loaded || state.loading) {
    return;
  }
  state.loading = true;

  function sourceName(assetPath) {
    return "fancontrol-plugin-" + PLUGIN_ID + "-" + assetPath.replace(/[^a-z0-9_-]/gi, "_");
  }

  function evaluateAsset(assetPath, source) {
    const run = new Function(String(source || "") + "\n//# sourceURL=" + sourceName(assetPath));
    run();
  }

  async function loadAsset(host, assetPath) {
    const api = host && host.apiService;
    if (api && typeof api.getPluginFrontendAsset === "function") {
      return api.getPluginFrontendAsset(PLUGIN_ID, assetPath);
    }
    if (api && typeof api.getPluginFrontendAssetPath === "function") {
      return api.getPluginFrontendAssetPath(PLUGIN_ID, assetPath);
    }
    throw new Error("Plugin frontend asset API is not available");
  }

  function registerFallback(host, reason) {
    if (!host || typeof host.registerPage !== "function") {
      window.setTimeout(() => registerFallback(window.FanControlPluginHost, reason), 60);
      return;
    }
    const React = host.React;
    const e = React.createElement;
    const icons = host.icons || {};
    const AlertTriangle = icons.AlertTriangle || icons.Circle;
    host.registerPage(PLUGIN_ID, {
      title: "OMEN 笔记本风扇",
      component: function OmenFanPluginLoadFallback() {
        return e("div", { className: "rounded-xl border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive" },
          e("div", { className: "flex items-center gap-2 font-semibold" }, AlertTriangle ? e(AlertTriangle, { size: 16 }) : null, "OMEN 插件前端加载失败"),
          e("p", { className: "mt-2 text-destructive/85" }, reason && reason.message ? reason.message : String(reason || "无法加载插件拆分资源。")),
        );
      },
    });
  }

  async function boot(host) {
    if (!host || typeof host.registerPage !== "function") {
      window.setTimeout(() => boot(window.FanControlPluginHost), 60);
      return;
    }
    try {
      for (const assetPath of ASSET_PATHS) {
        const source = await loadAsset(host, assetPath);
        if (!String(source || "").trim()) {
          throw new Error(assetPath + " is empty");
        }
        evaluateAsset(assetPath, source);
      }
      state.loaded = true;
    } catch (error) {
      console.error("OMEN plugin frontend asset loading failed", error);
      registerFallback(host, error);
    } finally {
      state.loading = false;
    }
  }

  return boot(window.FanControlPluginHost);
})();
