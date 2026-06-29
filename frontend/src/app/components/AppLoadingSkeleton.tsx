import { Skeleton } from '@/components/ui/skeleton';

export default function AppLoadingSkeleton() {
  return (
    <div
      data-theme-page="status"
      data-theme-section="app-shell"
      className="glacier-shell glacier-native-backdrop relative flex h-dvh w-full overflow-hidden bg-background text-foreground"
    >
      <aside
        data-theme-section="sidebar"
        className="glacier-sidebar flex w-16 shrink-0 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground shadow-[1px_0_0_rgba(15,23,42,0.04)] dark:shadow-[1px_0_0_rgba(255,255,255,0.04)]"
      >
        <div className="flex h-[76px] items-center justify-center px-2">
          <Skeleton className="h-9 w-9 rounded-lg" />
        </div>
        <div className="flex flex-1 flex-col items-center gap-3 px-2">
          <Skeleton className="h-11 w-11 rounded-xl" />
          <Skeleton className="h-11 w-11 rounded-xl" />
          <Skeleton className="h-11 w-11 rounded-xl" />
          <Skeleton className="h-11 w-11 rounded-xl" />
        </div>
        <div className="px-2 pb-5">
          <Skeleton className="mx-auto h-11 w-11 rounded-xl" />
        </div>
      </aside>

      <section data-theme-section="content" className="glacier-content relative flex min-w-0 flex-1 flex-col overflow-hidden">
        <div className="glacier-titlebar pointer-events-auto absolute left-16 right-0 top-0 flex h-10 items-center justify-between bg-background">
          <div className="flex h-full min-w-0 flex-1 items-center gap-2 px-3 pt-1">
            <Skeleton className="h-6 w-24 rounded-full" />
            <Skeleton className="h-6 w-24 rounded-full" />
          </div>
          <div className="flex h-full items-center gap-1 pr-2">
            <Skeleton className="h-8 w-10 rounded-md" />
            <Skeleton className="h-8 w-10 rounded-md" />
            <Skeleton className="h-8 w-10 rounded-md" />
          </div>
        </div>

        <div data-theme-section="content-panel" className="glacier-content-panel relative min-h-0 flex-1 overflow-hidden">
          <div className="app-scroll-root h-full">
            <div className="min-h-full px-4 pb-6 pt-4 sm:px-5 lg:px-6">
              <div className="mx-auto w-full max-w-[1120px] space-y-4 px-1 pb-2 min-[1680px]:max-w-[1280px] min-[2200px]:max-w-[1480px]">
                <Skeleton className="glacier-hero-card h-32 w-full rounded-2xl" />
                <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
                  <Skeleton className="glacier-metric-card h-36 rounded-2xl" />
                  <Skeleton className="glacier-metric-card h-36 rounded-2xl" />
                  <Skeleton className="glacier-metric-card h-36 rounded-2xl" />
                </div>
                <Skeleton className="glacier-control-card h-28 w-full rounded-2xl" />
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}
