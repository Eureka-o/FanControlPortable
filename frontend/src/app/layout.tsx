import type { Metadata } from "next";
import "./globals.css";
import SystemThemeSync from "./components/SystemThemeSync";
import FrontendDiagnostics from "./components/FrontendDiagnostics";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { BRAND } from "./lib/brand";
import { AppI18nProvider } from "./lib/i18n";
import { getThemeBootstrapScript } from "./lib/theme-bootstrap";

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
        <link rel="stylesheet" href="/fonts/manrope.css" />
        <link rel="stylesheet" href="/fonts/geist-mono.css" />
        <script
          id="fancontrol-theme-bootstrap"
          dangerouslySetInnerHTML={{ __html: themeBootstrapScript }}
        />
      </head>
      <body>
        <AppI18nProvider>
          <SystemThemeSync />
          {/* 前端「留痕」层：异常 / 导航 / 卸载上报到核心日志。
              挂在最外层，保证任何页签出的问题都能留下证据。 */}
          <FrontendDiagnostics />
          <TooltipProvider delayDuration={180}>
            {children}
            <Toaster richColors closeButton position="top-right" />
          </TooltipProvider>
        </AppI18nProvider>
      </body>
    </html>
  );
}
