/**
 * 前端 → 核心日志的唯一上报通道。
 */
export function reportClientIssue(kind: string, message: string, detail = ''): void {
  if (typeof window === 'undefined') return;
  const app = (
    window as unknown as { go?: { main?: { App?: Record<string, unknown> } } }
  ).go?.main?.App;
  const fn = app?.ReportClientIssue as
    | ((k: string, m: string, d: string) => Promise<unknown>)
    | undefined;
  if (!fn) return;
  try {
    void fn(kind, message, detail);
  } catch {
    // 上报失败必须静默，否则会再抛一次。
  }
}
