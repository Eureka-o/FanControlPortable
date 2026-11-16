"use client";

import { createContext, useContext, useEffect, useMemo, useRef, useState } from "react";
import i18n, { type Resource } from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import zhCN from "../locales/zh-CN/translation.json";

export const SUPPORTED_LOCALES = ["zh-CN", "en-US", "ja-JP"] as const;
export type AppLocale = (typeof SUPPORTED_LOCALES)[number];

const DEFAULT_LOCALE: AppLocale = "zh-CN";
const LOCALE_STORAGE_KEY = "fancontrol.locale";
const LEGACY_LOCALE_STORAGE_KEY = "thrm.locale";
const TRANSLATION_NS = "translation";
// 语言包加载兜底超时：超时后按“回退当前/默认语言内容”处理，绝不显示翻译 key。
const LOCALE_BUNDLE_TIMEOUT_MS = 15000;

const resources: Resource = {
  "zh-CN": { translation: zhCN },
};

type LocaleBundleModule = { default: Record<string, unknown> };

// zh-CN（默认 / 回退语言）随首包同步加载；en-US / ja-JP 拆为按需 chunk。
// 下面的模块级预取只负责让动态 import 尽早开始（与启动流程并行），
// 不阻塞首帧：默认语言用户的首屏不等待这两个 chunk。
const localeBundleLoaders: Record<AppLocale, () => Promise<LocaleBundleModule>> = {
  "zh-CN": () => Promise.resolve({ default: zhCN }),
  "en-US": () => import("../locales/en-US/translation.json"),
  "ja-JP": () => import("../locales/ja-JP/translation.json"),
};

const pendingBundleLoads = new Map<AppLocale, Promise<boolean>>();

function loadLocaleBundle(locale: AppLocale): Promise<boolean> {
  if (i18n.hasResourceBundle(locale, TRANSLATION_NS)) {
    return Promise.resolve(true);
  }

  const inFlight = pendingBundleLoads.get(locale);
  if (inFlight) {
    return inFlight;
  }

  const load = (async () => {
    try {
      const bundle = await localeBundleLoaders[locale]();
      i18n.addResourceBundle(locale, TRANSLATION_NS, bundle.default, true, true);
      return true;
    } catch (error) {
      console.warn(`加载 ${locale} 语言包失败:`, error);
      return false;
    } finally {
      pendingBundleLoads.delete(locale);
    }
  })();

  pendingBundleLoads.set(locale, load);
  return load;
}

function settleWithTimeout(load: Promise<boolean>): Promise<boolean> {
  return new Promise<boolean>((resolve) => {
    const timer = window.setTimeout(() => resolve(false), LOCALE_BUNDLE_TIMEOUT_MS);
    load.then(
      (loaded) => {
        window.clearTimeout(timer);
        resolve(loaded);
      },
      () => {
        window.clearTimeout(timer);
        resolve(false);
      },
    );
  });
}

// 切换语言前必须先经此确认目标语言包就绪；返回 false 时调用方应保持
// 当前内容（回退语言），不要把无资源的目标语言设为激活语言。
export function ensureLocaleBundle(locale: AppLocale): Promise<boolean> {
  if (i18n.hasResourceBundle(locale, TRANSLATION_NS)) {
    return Promise.resolve(true);
  }
  return settleWithTimeout(loadLocaleBundle(locale));
}

function normalizeLocale(value?: string | null): AppLocale {
  if (!value) {
    return DEFAULT_LOCALE;
  }

  if (SUPPORTED_LOCALES.includes(value as AppLocale)) {
    return value as AppLocale;
  }

  const lowered = value.toLowerCase();
  if (lowered.startsWith("zh")) {
    return "zh-CN";
  }
  if (lowered.startsWith("ja")) {
    return "ja-JP";
  }
  if (lowered.startsWith("en")) {
    return "en-US";
  }

  return DEFAULT_LOCALE;
}

function readPreferredLocale(): AppLocale {
  if (typeof window === "undefined") {
    return DEFAULT_LOCALE;
  }

  const stored = window.localStorage.getItem(LOCALE_STORAGE_KEY)
    ?? window.localStorage.getItem(LEGACY_LOCALE_STORAGE_KEY);
  if (stored) {
    return normalizeLocale(stored);
  }

  return normalizeLocale(window.navigator.language);
}

function syncDocumentLocale(locale: AppLocale) {
  if (typeof document === "undefined") {
    return;
  }

  document.documentElement.lang = locale;
  document.documentElement.dataset.locale = locale;
}

function syncDocumentMetadata(locale: AppLocale) {
  if (typeof document === "undefined") {
    return;
  }

  const t = i18n.getFixedT(locale);
  document.title = t("common.appName");

  const description = t("common.metaDescription");
  const descriptionMeta = document.querySelector('meta[name="description"]');
  if (descriptionMeta) {
    descriptionMeta.setAttribute("content", description);
  }
}

if (!i18n.isInitialized) {
  void i18n.use(initReactI18next).init({
    resources,
    lng: DEFAULT_LOCALE,
    fallbackLng: DEFAULT_LOCALE,
    supportedLngs: [...SUPPORTED_LOCALES],
    defaultNS: TRANSLATION_NS,
    ns: [TRANSLATION_NS],
    interpolation: {
      escapeValue: false,
    },
    returnNull: false,
    initAsync: false,
  });
}

// 让两个非默认语言包的动态 import 在模块求值时立即启动（客户端），
// 与 hydration 并行；首帧渲染只用同步在场的 zh-CN 资源，不等待它们。
function startNonDefaultBundlePrefetch(): void {
  for (const locale of SUPPORTED_LOCALES) {
    if (locale !== DEFAULT_LOCALE) {
      void loadLocaleBundle(locale);
    }
  }
}

if (typeof window !== "undefined") {
  startNonDefaultBundlePrefetch();
}

type LocaleContextValue = {
  locale: AppLocale;
  setLocale: (locale: AppLocale) => void;
  supportedLocales: readonly AppLocale[];
};

const LocaleContext = createContext<LocaleContextValue | null>(null);

export function AppI18nProvider({ children }: { children: React.ReactNode }) {
  const [locale, setLocaleState] = useState<AppLocale>(DEFAULT_LOCALE);
  const localeRef = useRef<AppLocale>(DEFAULT_LOCALE);

  useEffect(() => {
    setLocaleState(readPreferredLocale());
  }, []);

  useEffect(() => {
    localeRef.current = locale;
    syncDocumentLocale(locale);
    window.localStorage.setItem(LOCALE_STORAGE_KEY, locale);

    let cancelled = false;
    // 先确保目标语言包就绪，再 changeLanguage：等待期间界面停留在
    // 上一语言内容上（由同步在场的回退语言兜底），永远不会闪出翻译 key。
    void ensureLocaleBundle(locale).then((loaded) => {
      if (cancelled) {
        return;
      }
      if (!loaded) {
        console.warn(`语言包 ${locale} 不可用，暂时保持当前语言内容`);
        return;
      }
      void i18n.changeLanguage(locale).then(() => {
        if (!cancelled) {
          syncDocumentMetadata(locale);
        }
      });
    });

    return () => {
      cancelled = true;
    };
  }, [locale]);

  // 首帧绘制后的空闲时段预取剩余语言包：之后的语言切换零等待；
  // 预取失败不致命（切换时 ensureLocaleBundle 仍会按需重试加载）。
  useEffect(() => {
    const idlePrefetch = () => {
      for (const candidate of SUPPORTED_LOCALES) {
        if (candidate === DEFAULT_LOCALE) {
          continue;
        }
        void ensureLocaleBundle(candidate).then((loaded) => {
          // 自愈：若该语言包此前加载失败导致 changeLanguage 未执行，
          // 包就绪后补上切换，最终与用户选择的语言保持一致。
          if (loaded && localeRef.current === candidate && i18n.language !== candidate) {
            void i18n.changeLanguage(candidate).then(() => {
              syncDocumentMetadata(candidate);
            });
          }
        });
      }
    };

    const idleHandle = typeof window.requestIdleCallback === "function"
      ? window.requestIdleCallback(idlePrefetch)
      : window.setTimeout(idlePrefetch, 1200);

    return () => {
      if (typeof window.cancelIdleCallback === "function") {
        window.cancelIdleCallback(idleHandle);
      } else {
        window.clearTimeout(idleHandle);
      }
    };
  }, []);

  const value = useMemo<LocaleContextValue>(
    () => ({
      locale,
      setLocale: (nextLocale) => setLocaleState(normalizeLocale(nextLocale)),
      supportedLocales: SUPPORTED_LOCALES,
    }),
    [locale],
  );

  return (
    <I18nextProvider i18n={i18n}>
      <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>
    </I18nextProvider>
  );
}

export function useLocale() {
  const context = useContext(LocaleContext);

  if (!context) {
    throw new Error("useLocale must be used within AppI18nProvider");
  }

  return context;
}

export { i18n };
