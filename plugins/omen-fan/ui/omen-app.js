(function () {
  "use strict";

  const ns = window.FanControlOmenFanPlugin = window.FanControlOmenFanPlugin || {};

  function registerWithHost(host) {
    if (!host || typeof host.registerPage !== "function") {
      window.setTimeout(() => registerWithHost(window.FanControlPluginHost), 60);
      return;
    }
    if (typeof ns.register !== "function") {
      throw new Error("OMEN plugin views are not loaded");
    }
    ns.register(host);
  }

  Object.assign(ns, { registerWithHost });
  registerWithHost(window.FanControlPluginHost);
})();
