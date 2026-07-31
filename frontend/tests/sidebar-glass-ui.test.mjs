import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import test from "node:test";

const css = readFileSync(
  new URL("../src/app/globals.css", import.meta.url),
  "utf8",
);
const appShell = readFileSync(
  new URL("../src/app/components/AppShell.tsx", import.meta.url),
  "utf8",
);
const deviceStatus = readFileSync(
  new URL("../src/app/components/DeviceStatus.tsx", import.meta.url),
  "utf8",
);
const duneTheme = readFileSync(
  new URL("../../themes/dune/theme.css", import.meta.url),
  "utf8",
);
const mikuTheme = readFileSync(
  new URL("../../themes/miku-01/theme.css", import.meta.url),
  "utf8",
);
const shinchanTheme = readFileSync(
  new URL("../../themes/shinchan/theme.css", import.meta.url),
  "utf8",
);
const xiaobaDeluxeTheme = readFileSync(
  new URL("../../themes/xiaoba-deluxe/theme.css", import.meta.url),
  "utf8",
);
const cyberpunkTheme = readFileSync(
  new URL("../../themes/cyberpunk2077/theme.css", import.meta.url),
  "utf8",
);

test("keeps the built-in dock as one continuous surface", () => {
  assert.match(
    css,
    /--sidebar-dock-surface:\s*color-mix\(in srgb, var\(--sidebar\) 88%, transparent\)/,
  );
  assert.match(
    css,
    /html\.dark:not\(\[data-theme\]\) \.glacier-sidebar[\s\S]*?--sidebar-dock-surface:\s*color-mix\(in srgb, var\(--sidebar\) 86%, transparent\)/,
  );
  assert.match(
    css,
    /html:not\(\[data-theme\]\) \.glacier-sidebar::before[\s\S]*?z-index:\s*0[\s\S]*?border:\s*1px solid var\(--sidebar-dock-edge\)/,
  );
  assert.match(
    css,
    /background:\s*var\(--sidebar-dock-surface\)[\s\S]*?backdrop-filter:\s*blur\(18px\) saturate\(126%\)/,
  );
  assert.match(
    css,
    /\.glacier-sidebar\s*>\s*\[data-app-ui="sidebar-active-indicator"\][\s\S]*?z-index:\s*2\s*!important/,
  );
  assert.match(
    css,
    /--sidebar-active-fill:\s*color-mix\(in srgb, var\(--primary\) 28%, transparent\)/,
  );
  assert.match(
    css,
    /html\.dark:not\(\[data-theme\]\) \.glacier-sidebar[\s\S]*?--sidebar-active-fill:\s*color-mix\(in srgb, var\(--primary\) 32%, transparent\)/,
  );
  assert.match(
    css,
    /html:not\(\[data-theme\]\)[\s\S]*?sidebar-active-indicator"\][\s\S]*?background:\s*var\(--sidebar-active-fill\)\s*!important/,
  );
  assert.match(
    css,
    /html:not\(\[data-theme\]\) \.glacier-sidebar > \*[\s\S]*?z-index:\s*3/,
  );
  assert.match(
    css,
    /\[data-theme-ui="sidebar-item"\]\[aria-selected="true"\][\s\S]*?background:\s*transparent/,
  );
});

test("keeps built-in glass quieter and cards aligned with the dock palette", () => {
  assert.match(css, /--window-glass-panel-bg:\s*rgba\(247, 250, 253, 0\.62\)/);
  assert.match(css, /--window-glass-card-bg:\s*rgba\(255, 255, 255, 0\.8\)/);
  assert.match(css, /--window-glass-card-strong-bg:\s*rgba\(255, 255, 255, 0\.86\)/);
  assert.match(css, /--window-glass-panel-bg:\s*rgba\(10, 13, 18, 0\.68\)/);
  assert.match(css, /--window-glass-card-bg:\s*rgba\(16, 21, 28, 0\.78\)/);
  assert.match(css, /--window-glass-card-strong-bg:\s*rgba\(18, 24, 32, 0\.84\)/);
});

test("keeps the built-in workspace continuous when window blur is off", () => {
  const contentPanelRule = css.match(
    /\.glacier-native-backdrop \.glacier-content-panel\s*\{[\s\S]*?\n\}/,
  )?.[0];
  assert.ok(contentPanelRule);
  assert.match(contentPanelRule, /background:\s*transparent/);
  assert.match(contentPanelRule, /border:\s*0/);
  assert.match(contentPanelRule, /border-radius:\s*0/);
  assert.match(contentPanelRule, /box-shadow:\s*none/);
  assert.doesNotMatch(contentPanelRule, /border-(?:top|left):/);
});

test("expands one themed dock without transient pointer focus frames", () => {
  assert.equal(
    appShell.match(/data-app-ui="sidebar-active-indicator"/g)?.length,
    1,
  );
  assert.equal(appShell.match(/onPointerDown=\{\(event\)/g)?.length, 1);
  assert.match(
    appShell,
    /onPointerDown=\{\(event\) => \{[\s\S]*?event\.preventDefault\(\);[\s\S]*?event\.currentTarget\.blur\(\);/,
  );
  assert.match(
    appShell,
    /const pointerTarget = event\.detail > 0 \? event\.currentTarget : null;[\s\S]*?pointerTarget\?\.blur\(\);[\s\S]*?onClick\(\);[\s\S]*?requestAnimationFrame\(\(\) => pointerTarget\.blur\(\)\)/,
  );
  assert.doesNotMatch(appShell, /layoutId="sidebar-active-indicator"/);
  assert.match(appShell, /DOCK_COLLAPSED_WIDTH = 64/);
  assert.match(appShell, /DOCK_EXPANDED_WIDTH = 192/);
  assert.match(
    appShell,
    /DOCK_EXPANDED_STORAGE_KEY = ["']fancontrol\.dock\.expanded["']/,
  );
  assert.match(appShell, /style=\{\{\s*width:\s*dockWidth\s*\}\}/);
  assert.match(
    appShell,
    /style=\{\{[\s\S]*?\.\.\.DRAG_STYLE,[\s\S]*?left:\s*leftOffset,[\s\S]*?zIndex:/,
  );
  assert.match(
    appShell,
    /data-dock-expanded=\{dockExpanded \? ["']true["'] : ["']false["']\}/,
  );
  assert.match(
    css,
    /top:\s*4\.8125rem\s*!important;\s*\n\s*right:\s*0\.6875rem\s*!important;\s*\n\s*left:\s*0\.6875rem\s*!important/,
  );
  assert.match(css, /height:\s*2\.625rem\s*!important/);
  assert.match(css, /--sidebar-active-offset:\s*calc\(100dvh - 11\.75rem\)/);
  assert.match(
    css,
    /transform:\s*translateY\(var\(--sidebar-active-offset\)\)\s*!important/,
  );
  assert.match(css, /transform 260ms cubic-bezier/);
  assert.match(
    css,
    /html\[data-theme\]\[data-theme\][\s\S]*?sidebar-active-indicator"\][\s\S]*?border:\s*0\s*!important[\s\S]*?var\(--primary\) 24%[\s\S]*?var\(--sidebar\) 76%/,
  );
  assert.match(
    css,
    /box-shadow:\s*inset 0 1px 0[\s\S]*?white 22%, transparent/,
  );
  const themedIndicatorRule = css.match(
    /html\[data-theme\]\[data-theme\][\s\S]*?> \[data-app-ui="sidebar-active-indicator"\]\s*\{[\s\S]*?\n\}/,
  )?.[0];
  assert.ok(themedIndicatorRule);
  assert.doesNotMatch(themedIndicatorRule, /box-shadow:\s*0 0/);
  assert.match(
    css,
    /\[data-theme="doro"\],[\s\S]*?\[data-theme="xiaoba-deluxe"\][\s\S]*?data-dock-expanded="false"\]::after[\s\S]*?bottom:\s*8\.25rem\s*!important/,
  );
  for (const part of ["f", "an", "c", "ontrol"]) {
    assert.match(appShell, new RegExp(`data-brand-part=["']${part}["']`));
  }
  assert.match(css, /fancontrol-wordmark-v1-dark\.png/);
  assert.match(css, /fancontrol-wordmark-v1\.png/);
  assert.match(appShell, /fancontrol-fc-mark-v1-dark\.png/);
  assert.match(appShell, /fancontrol-fc-mark-v1\.png/);
  assert.match(
    css,
    /data-app-ui="brand-compact"[\s\S]*?width:\s*32px[\s\S]*?height:\s*32px/,
  );
  assert.match(
    css,
    /data-dock-expanded="true"[\s\S]*?brand-wordmark[\s\S]*?left:\s*20px/,
  );
  assert.match(
    css,
    /data-dock-expanded="true"[\s\S]*?data-brand-part="f"[\s\S]*?data-brand-part="c"[\s\S]*?translateX\(0\) scale\(1\)/,
  );
  assert.match(css, /transition-delay:\s*100ms/);
  assert.match(appShell, /duration-\[420ms\]/);
  assert.match(appShell, /PanelLeftClose/);
  assert.match(appShell, /data-app-ui="sidebar-item-label"/);
  for (const asset of [
    "../public/brand/fancontrol-wordmark-v1.png",
    "../public/brand/fancontrol-wordmark-v1-dark.png",
  ]) {
    assert.ok(
      existsSync(new URL(asset, import.meta.url)),
      `${asset} should exist`,
    );
  }
  assert.match(
    css,
    /Custom themes own their artwork[\s\S]*?html\[data-theme\]\[data-theme\] body \.glacier-shell \.glacier-sidebar[\s\S]*?border:\s*0\s*!important[\s\S]*?background-color:\s*color-mix\(in srgb, var\(--card\) 90%, transparent\)\s*!important[\s\S]*?clip-path:\s*inset\(0\.5rem 0\.375rem round 1\.25rem\)/,
  );
  assert.doesNotMatch(
    css,
    /html\[data-theme\]\[data-theme\] body \.glacier-shell \.glacier-sidebar::before/,
  );
  assert.match(
    css,
    /\.glacier-sidebar\[data-dock-expanded="true"\]::after[\s\S]*?opacity:\s*0\s*!important/,
  );
  assert.match(
    duneTheme,
    /\.glacier-sidebar::before[\s\S]*?var\(--dune-sidebar-pattern\)/,
  );
  assert.match(
    css,
    /html\[data-theme\]\[data-theme\]\s+body\s+\.glacier-shell\s+:is\(\.glacier-titlebar, \.glacier-content, \.glacier-content-panel\)[\s\S]*?border:\s*0\s*!important[\s\S]*?border-radius:\s*0\s*!important[\s\S]*?background:\s*transparent\s*!important[\s\S]*?box-shadow:\s*none\s*!important[\s\S]*?backdrop-filter:\s*none\s*!important/,
  );
  assert.doesNotMatch(
    appShell,
    /<span[^>]+data-app-ui="sidebar-item-(?:icon|label)"/,
  );
  assert.doesNotMatch(
    css,
    /html\[data-theme\]\[data-theme\] body \.glacier-shell \.glacier-content::before/,
  );
  assert.doesNotMatch(css, /html\[data-theme\][^{]*\.glacier-titlebar::before/);
  assert.doesNotMatch(css, /html\[data-theme\][^{]*\.glacier-titlebar::after/);
  assert.match(
    css,
    /\.glacier-sidebar\s*>\s*\[data-app-ui="sidebar-active-indicator"\][\s\S]*?z-index:\s*2\s*!important/,
  );
  assert.match(
    css,
    /\.glacier-sidebar\s*>\s*:not\(\[data-app-ui="sidebar-active-indicator"\]\)[\s\S]*?z-index:\s*3\s*!important/,
  );
  assert.match(
    css,
    /html\[data-theme\]\[data-theme\]\s+body\s+\.glacier-shell\s+\.glacier-sidebar\s+button\[data-theme-ui="sidebar-item"\]\[aria-selected="true"\][\s\S]*?border:\s*0\s*!important[\s\S]*?background:\s*transparent\s*!important[\s\S]*?animation:\s*none\s*!important[\s\S]*?backdrop-filter:\s*none\s*!important/,
  );
  assert.doesNotMatch(
    css,
    /\.glacier-shell \[data-theme-ui="sidebar-item"\]\[aria-selected="true"\],\s*\.glacier-shell \[data-theme-ui="settings-tab"\]\[aria-selected="true"\]/,
  );
  assert.match(
    css,
    /button\[data-theme-ui="sidebar-item"\]\[aria-selected="true"\]::before,[\s\S]*?::after[\s\S]*?content:\s*none\s*!important/,
  );
  assert.match(
    css,
    /button\[data-theme-ui="sidebar-item"\]:not\(\[aria-selected="true"\]\):not\(\s*:hover\s*\):not\(:focus-visible\)[\s\S]*?box-shadow:\s*none\s*!important/,
  );
  assert.match(
    css,
    /html\[data-theme\]\[data-theme\][\s\S]*?button\[data-theme-ui="sidebar-item"\]\s*\{[\s\S]*?border-color:\s*transparent\s*!important[\s\S]*?background:\s*transparent\s*!important[\s\S]*?background-image:\s*none\s*!important[\s\S]*?box-shadow:\s*none\s*!important[\s\S]*?transition:\s*color 160ms ease\s*!important/,
  );
  assert.match(
    css,
    /button\[data-theme-ui="sidebar-item"\]:is\(:hover, :active\)[\s\S]*?transform:\s*none\s*!important[\s\S]*?box-shadow:\s*none\s*!important[\s\S]*?filter:\s*none\s*!important[\s\S]*?backdrop-filter:\s*none\s*!important/,
  );
  assert.match(
    css,
    /button\[data-theme-ui="sidebar-item"\]:focus-visible[\s\S]*?outline:\s*none\s*!important[\s\S]*?box-shadow:\s*none\s*!important/,
  );
  assert.match(
    css,
    /button\[data-theme-ui="sidebar-item"\]:focus:not\(:focus-visible\)[\s\S]*?outline:\s*none\s*!important/,
  );
  assert.match(
    css,
    /button\[data-theme-ui="sidebar-item"\]:focus-visible\s+\[data-app-ui="sidebar-item-icon"\][\s\S]*?drop-shadow/,
  );
  assert.match(appShell, /data-app-ui="titlebar-info"/);
  assert.match(appShell, /data-app-ui="titlebar-controls"/);
  assert.equal(
    appShell.match(/data-app-ui="titlebar-status-card"/g)?.length,
    4,
  );
  assert.equal(
    appShell.match(/data-app-ui="titlebar-window-button"/g)?.length,
    1,
  );
  assert.match(
    css,
    /:is\(\.glacier-titlebar-info, \.glacier-titlebar-controls\)[\s\S]*?background:\s*transparent[\s\S]*?box-shadow:\s*none[\s\S]*?backdrop-filter:\s*none/,
  );
  assert.match(
    css,
    /\[data-app-ui="titlebar-status-card"\][\s\S]*?backdrop-filter:\s*blur\(10px\) saturate\(116%\)/,
  );
  assert.match(
    css,
    /\[data-app-ui="titlebar-window-button"\]:not\(:hover\)[\s\S]*?background:\s*color-mix\(in srgb, var\(--card\) 24%, transparent\)[\s\S]*?backdrop-filter:\s*blur\(10px\) saturate\(116%\)/,
  );
  assert.match(
    css,
    /html:not\(\[data-theme\]\) \[data-theme-ui="hero-brand-mark"\][\s\S]*?display:\s*none/,
  );
  assert.doesNotMatch(deviceStatus, /fancontrol-flow-loop-v2/);
});

test("keeps illustration curtains on the full window shell", () => {
  assert.match(duneTheme, /\.glacier-sidebar\s*\{[\s\S]*?background:/);
  assert.match(mikuTheme, /\.glacier-sidebar\s*\{[\s\S]*?background:/);
  assert.match(shinchanTheme, /\.glacier-sidebar\s*\{[\s\S]*?background:/);
  assert.match(xiaobaDeluxeTheme, /\.glacier-sidebar\s*\{[\s\S]*?background:/);
  const xiaobaSidebarFrame = xiaobaDeluxeTheme.match(
    /\.glacier-sidebar::before\s*\{[\s\S]*?\n\}/,
  )?.[0];
  assert.ok(xiaobaSidebarFrame);
  assert.doesNotMatch(xiaobaSidebarFrame, /--xiaoba-deluxe-star/);
  assert.match(
    duneTheme,
    /One continuous window curtain[\s\S]*?\.glacier-shell::before[\s\S]*?var\(--dune-poster-image\)[\s\S]*?\.glacier-content-panel::before,[\s\S]*?content:\s*none\s*!important/,
  );

  for (const [theme, imageVariable] of [
    [mikuTheme, "--miku-01-curtain-image"],
    [shinchanTheme, "--shinchan-curtain-image"],
    [xiaobaDeluxeTheme, "--xiaoba-deluxe-curtain-image"],
  ]) {
    const curtainOverride = theme.lastIndexOf(
      "Keep the illustration on the window curtain",
    );
    assert.ok(curtainOverride >= 0);
    const finalCss = theme.slice(curtainOverride);
    assert.match(
      finalCss,
      /\.glacier-shell::before[\s\S]*?inset:\s*0\s*!important/,
    );
    assert.ok(finalCss.includes(`var(${imageVariable})`));
    assert.match(
      finalCss,
      /\.glacier-content-panel::before[\s\S]*?content:\s*none\s*!important/,
    );
  }
});

test("keeps the cyberpunk compact dock hover inside the icon slot", () => {
  assert.ok(
    cyberpunkTheme.lastIndexOf("Keep compact dock feedback") >
      cyberpunkTheme.lastIndexOf('.glacier-shell button,'),
  );
  assert.match(
    cyberpunkTheme,
    /data-dock-expanded="false"\][\s\S]*?button\[data-theme-ui="sidebar-item"\][\s\S]*?background:\s*transparent\s*!important[\s\S]*?clip-path:\s*none\s*!important/,
  );
  assert.match(
    cyberpunkTheme,
    /button\[data-theme-ui="sidebar-item"\]:not\(\[aria-selected="true"\]\):hover\s*>\s*i:first-of-type[\s\S]*?border:[\s\S]*?var\(--cyberpunk-cyan\)[\s\S]*?background:/,
  );
  assert.match(
    cyberpunkTheme,
    /data-dock-expanded="false"\]::after[\s\S]*?bottom:\s*11\.75rem\s*!important[\s\S]*?width:\s*7\.25rem/,
  );
});
