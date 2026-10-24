'use client';

import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { reportClientIssue } from './lib/client-report';

/**
 * 页签级错误边界。
 */
export default function AppError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const { t } = useTranslation();

  useEffect(() => {
    reportClientIssue(
      'react-error-boundary',
      error.message || 'unknown error',
      `${error.digest ? `digest=${error.digest} ` : ''}${error.stack ?? ''}`,
    );
  }, [error]);

  return (
    <div className="flex min-h-[50vh] flex-col items-center justify-center gap-3 px-8 py-10 text-center">
      <div className="text-base font-medium text-foreground">
        {t('appError.title', { defaultValue: '这个页签出了点问题' })}
      </div>
      <p className="max-w-md text-xs leading-relaxed text-muted-foreground">
        {t('appError.hint', {
          defaultValue:
            '只有当前页签受影响，切到别的页签可以继续使用。错误已经记入日志；也可以点下面的按钮重新挂载这个页签。',
        })}
      </p>
      <pre className="max-h-40 max-w-lg overflow-auto rounded-lg border border-border/60 bg-muted/30 p-3 text-left text-[11px] leading-relaxed text-muted-foreground">
        {error.message}
      </pre>
      <button
        type="button"
        onClick={() => reset()}
        className="rounded-lg bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground transition-opacity hover:opacity-90"
      >
        {t('appError.retry', { defaultValue: '重新加载这个页签' })}
      </button>
    </div>
  );
}
