(function () {
  "use strict";

  const ns = window.FanControlOmenFanPlugin = window.FanControlOmenFanPlugin || {};

  function createUiKit(host) {
        const React = host.React;
        const components = host.components || {};
        const icons = host.icons || {};
        const e = React.createElement;
        const HostButton = components.Button;
        const HostToggleSwitch = components.ToggleSwitch;
        const HostSelect = components.Select;
        const HostSlider = components.Slider;
        const HostCard = components.Card;
        const HostBadge = components.Badge;
        const HostTabs = components.Tabs;
        const HostTabsList = components.TabsList;
        const HostTabsTrigger = components.TabsTrigger;
        const FanCurveEditor = components.FanCurveEditor;
        // Constants from omen-core.js are exposed on the plugin namespace (ns) but each asset
        // runs inside its own `new Function` scope, so we must pull them explicitly here.
        const TABS = ns.TABS;

        function Icon({ name, size = 18, className = "" }) {
          const Component = icons[name] || icons.Circle;
          return Component ? e(Component, { size, className }) : null;
        }

        function OmenMark() {
          return e("span", { className: "omen-diamond-mark", "aria-hidden": "true" });
        }

        function StatusPill({ children, tone = "muted", icon }) {
          if (HostBadge) {
            const variant = tone === "success"
              ? "success"
              : tone === "warning"
                ? "warning"
                : tone === "info"
                  ? "info"
                  : "default";
            return e(HostBadge, { variant, size: "sm" }, icon ? [icon, children] : children);
          }
          return e("span", { className: `omen-pill ${tone}` }, icon || null, children);
        }

        function CardShell({ children, className = "", padding = "md", hover = false, onClick, dataThemeCard, dataThemeUi, dataThemeSection }) {
          const interactive = Boolean(hover || onClick);
          const cardProps = {
            className,
            padding,
            hover: interactive,
            ...(dataThemeCard ? { "data-theme-card": dataThemeCard } : {}),
            ...(dataThemeUi ? { "data-theme-ui": dataThemeUi } : {}),
            ...(dataThemeSection ? { "data-theme-section": dataThemeSection } : {}),
          };
          const card = HostCard
            ? e(HostCard, cardProps, children)
            : e("div", {
              className: `rounded-xl border border-border bg-card shadow-sm shadow-black/5 ${className}`,
              ...(dataThemeCard ? { "data-theme-card": dataThemeCard } : {}),
              ...(dataThemeUi ? { "data-theme-ui": dataThemeUi } : {}),
              ...(dataThemeSection ? { "data-theme-section": dataThemeSection } : {}),
            }, children);

          if (!onClick) {
            return card;
          }
          return e("button", {
            type: "button",
            className: "omen-card-button",
            onClick,
          }, card);
        }

        function TabSwitch({ settings, updateSettings }) {
          if (HostTabs && HostTabsList && HostTabsTrigger) {
            return e(HostTabs, {
              value: settings.tab,
              onValueChange: (tab) => updateSettings({ tab }),
            },
              e(HostTabsList, null, TABS.map(([id, label]) => (
                e(HostTabsTrigger, { key: id, value: id }, label)
              ))),
            );
          }
          return e("div", { className: "omen-tab-list" }, TABS.map(([id, label]) => (
            e("button", {
              key: id,
              type: "button",
              className: `omen-tab${settings.tab === id ? " active" : ""}`,
              onClick: () => updateSettings({ tab: id }),
            }, label)
          )));
        }

        function Button({ children, onClick, primary, disabled, icon }) {
          if (HostButton) {
            return e(HostButton, {
              variant: primary ? "primary" : "secondary",
              size: "sm",
              onClick,
              disabled,
              icon,
            }, children);
          }
          return e("button", {
            type: "button",
            className: `omen-native-button${primary ? " primary" : ""}`,
            onClick,
            disabled,
          }, icon ? [icon, children] : children);
        }

        function Toggle({ checked, onChange, label }) {
          if (HostToggleSwitch) {
            return e(HostToggleSwitch, { enabled: checked, onChange, label, size: "sm", color: "blue" });
          }
          return e("button", {
            type: "button",
            className: `omen-segment${checked ? " active" : ""}`,
            "aria-pressed": checked,
            onClick: () => onChange(!checked),
          }, label || (checked ? "开" : "关"));
        }

        function SelectControl({ value, onChange, options }) {
          if (HostSelect) {
            return e(HostSelect, { value, onChange, options, size: "sm", className: "w-full" });
          }
          return e("select", {
            className: "omen-select-fallback",
            value,
            onChange: (event) => onChange(event.target.value),
          }, options.map((option) => e("option", { key: option.value, value: option.value }, option.label)));
        }

        function SliderControl({ title, value, min, max, suffix, onChange }) {
          if (HostSlider) {
            // Empty title means the caller already renders label + value elsewhere and expects a
            // bare slider. Host Slider defaults showValue=true; we must both suppress the label
            // and disable the top-row value or it prints "70°C" above the track a second time.
            const hasTitle = typeof title === "string" && title.length > 0;
            return e(HostSlider, {
              value,
              min,
              max,
              step: 1,
              label: hasTitle ? title : undefined,
              showValue: hasTitle,
              valueFormatter: (next) => `${next}${suffix || ""}`,
              onChange,
            });
          }
          return e("label", { className: "omen-subtle" },
            e("div", { className: "omen-inline", style: { justifyContent: "space-between", marginBottom: 8 } },
              e("span", null, title),
              e("strong", { style: { color: "var(--primary)" } }, `${value}${suffix || ""}`),
            ),
            e("input", {
              className: "omen-slider-fallback",
              type: "range",
              min,
              max,
              step: 1,
              value,
              onChange: (event) => onChange(Number(event.target.value)),
            }),
          );
        }

    return {
      e,
      Icon,
      OmenMark,
      StatusPill,
      CardShell,
      TabSwitch,
      Button,
      Toggle,
      SelectControl,
      SliderControl,
      FanCurveEditor,
    };
  }

  Object.assign(ns, { createUiKit });
})();
