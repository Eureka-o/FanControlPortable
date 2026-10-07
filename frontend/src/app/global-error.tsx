'use client';

import { useEffect } from 'react';

/**
 * 根级错误边界（最后一层兜底）。
 */
export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    const app = (
      window as unknown as { go?: { main?: { App?: Record<string, unknown> } } }
    ).go?.main?.App;
    const fn = app?.ReportClientIssue as
      | ((k: string, m: string, d: string) => Promise<unknown>)
      | undefined;
    if (!fn) return;
    try {
      void fn(
        'react-global-error',
        error.message || 'unknown error',
        `${error.digest ? `digest=${error.digest} ` : ''}${error.stack ?? ''}`,
      );
    } catch {
      // 上报失败不能再抛。
    }
  }, [error]);

  return (
    <html lang="zh-CN">
      <body
        style={{
          margin: 0,
          minHeight: '100vh',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 12,
          padding: 32,
          fontFamily: 'system-ui, -apple-system, "Segoe UI", "Microsoft YaHei", sans-serif',
          background: '#0b0b0c',
          color: '#f5f5f6',
          textAlign: 'center',
        }}
      >
        <div style={{ fontSize: 16, fontWeight: 600 }}>界面初始化失败</div>
        <div style={{ fontSize: 12, lineHeight: 1.7, opacity: 0.72, maxWidth: 420 }}>
          应用没能完成初始化。错误已经记入日志（`logs/app-*.log` 里搜 `[前端]`），
          重新加载通常可以恢复。
        </div>
        <pre
          style={{
            maxWidth: 520,
            maxHeight: 160,
            overflow: 'auto',
            textAlign: 'left',
            fontSize: 11,
            lineHeight: 1.6,
            opacity: 0.72,
            border: '1px solid rgba(255,255,255,0.14)',
            borderRadius: 8,
            padding: 10,
            margin: 0,
          }}
        >
          {error.message}
        </pre>
        <button
          type="button"
          onClick={() => reset()}
          style={{
            border: 'none',
            borderRadius: 8,
            padding: '7px 14px',
            fontSize: 13,
            fontWeight: 500,
            cursor: 'pointer',
            background: '#f5f5f6',
            color: '#0b0b0c',
          }}
        >
          重新加载
        </button>
      </body>
    </html>
  );
}
