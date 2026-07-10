import type { Metadata } from "next";
import "./globals.css";
import SystemThemeSync from "./components/SystemThemeSync";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { BRAND } from "./lib/brand";
import { AppI18nProvider } from "./lib/i18n";
import { getThemeBootstrapScript } from "./lib/theme-bootstrap";

// Temporary diagnostic — mounts a fixed banner on any window error or unhandled
// promise rejection so we can see the real stack even without devtools. Safe to
// remove once the diagnostic is over.
const FATAL_CATCHER_SCRIPT = `
(() => {
  function showBanner(kind, text) {
    try {
      var id = 'thrm-fatal-banner';
      var el = document.getElementById(id);
      if (!el) {
        el = document.createElement('pre');
        el.id = id;
        el.style.cssText = 'position:fixed;top:0;left:0;right:0;z-index:2147483647;margin:0;padding:12px 16px;font:12px/1.5 ui-monospace,Consolas,monospace;color:#fff;background:#b91c1c;white-space:pre-wrap;word-break:break-word;max-height:60vh;overflow:auto;box-shadow:0 4px 12px rgba(0,0,0,.4);border-bottom:2px solid #7f1d1d;';
        (document.body || document.documentElement).appendChild(el);
      }
      el.textContent = '[' + kind + '] ' + text;
      try { console.error('[thrm-fatal]', kind, text); } catch (_) {}
      try { window.localStorage.setItem('thrm_last_fatal', JSON.stringify({kind: kind, text: text, at: new Date().toISOString()})); } catch (_) {}
    } catch (_) {}
  }
  window.addEventListener('error', function (e) {
    var msg = (e && e.message) ? e.message : String(e);
    var src = e && e.filename ? e.filename : '';
    var line = e && e.lineno ? e.lineno : '';
    var col = e && e.colno ? e.colno : '';
    var stack = (e && e.error && e.error.stack) ? e.error.stack : '';
    showBanner('error', msg + ' @ ' + src + ':' + line + ':' + col + (stack ? ('\\n' + stack) : ''));
  }, true);
  window.addEventListener('unhandledrejection', function (e) {
    var reason = e && e.reason;
    var text = reason && reason.stack ? reason.stack : (reason && reason.message ? reason.message : String(reason));
    showBanner('rejection', text);
  });
})();
`.trim();

export const metadata: Metadata = {
  title: BRAND.name,
  description: BRAND.description,
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const themeBootstrapScript = getThemeBootstrapScript();

  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <head>
        <script
          id="thrm-fatal-catcher"
          dangerouslySetInnerHTML={{ __html: FATAL_CATCHER_SCRIPT }}
        />
        <script
          id="thrm-theme-bootstrap"
          dangerouslySetInnerHTML={{ __html: themeBootstrapScript }}
        />
      </head>
      <body>
        <AppI18nProvider>
          <SystemThemeSync />
          <TooltipProvider delayDuration={180}>
            {children}
            <Toaster richColors closeButton position="top-right" />
          </TooltipProvider>
        </AppI18nProvider>
      </body>
    </html>
  );
}
