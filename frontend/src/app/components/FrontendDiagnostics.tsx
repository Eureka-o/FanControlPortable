'use client';

import { useEffect } from 'react';
import { reportClientIssue } from '../lib/client-report';

/**
 * localStorage 键：卸载时留下的「退出时所在地址」，下次启动读出来上报。
 */
const LAST_UNLOAD_KEY = 'fancontrol:last-unload';

/** 应用自身的根地址：Wails 的 assetserver 就挂在这里。 */
const APP_ROOT_URL = 'http://wails.localhost/';

/**
 * 前端「留痕」层：把界面上发生的异常与导航事件上报给核心日志。
 */
export default function FrontendDiagnostics() {
  useEffect(() => {
    // 回放上次运行留下的地址记录。
    try {
      const raw = window.localStorage.getItem(LAST_UNLOAD_KEY);
      if (raw) {
        window.localStorage.removeItem(LAST_UNLOAD_KEY);
        // 只有"根地址以外"才当信号，否则不刷日志，避免每次启动一条噪声。
        const url = (() => {
          try {
            return String((JSON.parse(raw) as { url?: string }).url ?? '');
          } catch {
            return '';
          }
        })();
        if (url && url !== APP_ROOT_URL) {
          reportClientIssue(
            'last-unload',
            '上一次运行退出时不在应用根路径（可能是被整页导航带走的）',
            raw,
          );
        }
      }
    } catch {
      // localStorage 不可用就算了，不能因此让应用起不来
    }

    const onError = (event: ErrorEvent) => {
      const err = event.error as Error | undefined;
      reportClientIssue(
        'error',
        event.message || 'unknown error',
        `${event.filename || ''}:${event.lineno || 0}:${event.colno || 0} ${err?.stack ?? ''}`,
      );
    };

    const onRejection = (event: PromiseRejectionEvent) => {
      const reason = event.reason as { message?: string; stack?: string } | string | undefined;
      const message =
        typeof reason === 'string' ? reason : reason?.message ?? String(reason ?? 'unknown');
      reportClientIssue('rejection', message, typeof reason === 'object' ? reason?.stack ?? '' : '');
    };

    const onBeforeUnload = () => {
      // 卸载路径上只有同步动作可靠，所以在此同步写 localStorage。
      try {
        window.localStorage.setItem(
          LAST_UNLOAD_KEY,
          JSON.stringify({
            url: window.location.href,
            at: new Date().toISOString(),
            readyState: document.readyState,
          }),
        );
      } catch {
        // 存储满/被禁用就算了
      }
    };

    const onClickCapture = (event: MouseEvent) => {
      const target = event.target as HTMLElement | null;
      const anchor = target?.closest?.('a[href]') as HTMLAnchorElement | null;
      if (!anchor) return;
      const href = anchor.getAttribute('href') || '';
      if (!href || href.startsWith('#')) return;
      // 外链交给 Wails 的默认处理（会开系统浏览器），不拦。
      if (/^(https?:|mailto:)/i.test(href)) return;
      event.preventDefault();
      reportClientIssue('link', href, anchor.outerHTML.slice(0, 300));
    };

    window.addEventListener('error', onError);
    window.addEventListener('unhandledrejection', onRejection);
    window.addEventListener('beforeunload', onBeforeUnload);
    document.addEventListener('click', onClickCapture, true);

    return () => {
      window.removeEventListener('error', onError);
      window.removeEventListener('unhandledrejection', onRejection);
      window.removeEventListener('beforeunload', onBeforeUnload);
      document.removeEventListener('click', onClickCapture, true);
    };
  }, []);

  return null;
}
