'use client';

import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import { Download, Heart, Mail, MessageCircle, RefreshCw, Rocket, Sparkles } from 'lucide-react';
import { toast } from 'sonner';
import { useTranslation } from 'react-i18next';
import { BRAND } from '../lib/brand';
import { ALIPAY_QR_DATA_URL, WECHAT_PAY_QR_DATA_URL } from '../lib/support-assets';
import { apiService, type UpdateRelease } from '../services/api';
import { useUpdateStore } from './UpdateProgressWidget';
import { Badge, Button, ScrollArea } from './ui/index';

type ReleaseChannel = 'stable' | 'prerelease';

function openUrl(url: string) {
  try {
    BrowserOpenURL(url);
  } catch {
    window.open(url, '_blank', 'noopener,noreferrer');
  }
}

function isLatestVersion(currentVersion: string, latestVersion: string) {
  const normalize = (value: string) => value.trim().replace(/^v/i, '').toLowerCase();
  const currentRaw = normalize(currentVersion);
  const latestRaw = normalize(latestVersion);
  if (!currentRaw || !latestRaw) return true;
  if (currentRaw === latestRaw) return true;

  const parseNightly = (value: string): number | null => {
    const match = value.match(/^nightly[-.]?(\d{8})$/i);
    return match ? Number(match[1]) : null;
  };

  const parseSemverParts = (value: string): { parts: number[]; prerelease: boolean } | null => {
    const [withoutBuild] = value.split('+');
    const [base, suffix] = withoutBuild.split('-', 2);
    if (!/^\d+(\.\d+){0,3}$/.test(base)) return null;
    return { parts: base.split('.').map((part) => Number(part)), prerelease: Boolean(suffix) };
  };

  const currentNightly = parseNightly(currentRaw);
  const latestNightly = parseNightly(latestRaw);
  if (currentNightly !== null && latestNightly !== null) {
    return latestNightly <= currentNightly;
  }

  const currentSemver = parseSemverParts(currentRaw);
  const latestSemver = parseSemverParts(latestRaw);
  if (!currentSemver || !latestSemver) {
    return false;
  }

  const current = currentSemver.parts;
  const latest = latestSemver.parts;
  const length = Math.max(current.length, latest.length);

  for (let index = 0; index < length; index += 1) {
    const currentPart = current[index] ?? 0;
    const latestPart = latest[index] ?? 0;
    if (latestPart > currentPart) return false;
    if (latestPart < currentPart) return true;
  }

  if (currentSemver.prerelease !== latestSemver.prerelease) {
    return !latestSemver.prerelease;
  }

  return true;
}

const ABOUT_CARD_CLASS = 'min-w-0 rounded-3xl border border-border/70 bg-card p-5';
const SUPPORT_EMAIL = '1989005183@qq.com';
const SUPPORT_QQ_GROUP = '928338191';

export default function AboutPanel() {
  const { t } = useTranslation();
  const [appVersion, setAppVersion] = useState('');
  const [releaseChannel, setReleaseChannel] = useState<ReleaseChannel>('stable');
  const [latestReleaseTag, setLatestReleaseTag] = useState('');
  const [latestReleaseUrl, setLatestReleaseUrl] = useState<string>(BRAND.latestReleaseUrl);
  const [latestReleaseBody, setLatestReleaseBody] = useState('');
  const [latestReleaseIsPrerelease, setLatestReleaseIsPrerelease] = useState(false);
  const [installerUrl, setInstallerUrl] = useState('');
  const [releaseLoading, setReleaseLoading] = useState(false);
  const [releaseError, setReleaseError] = useState('');
  const updateStage = useUpdateStore((state) => state.stage);
  const updatePercent = useUpdateStore((state) => state.percent);
  const startUpdate = useUpdateStore((state) => state.startUpdate);
  const updateStarting = updateStage === 'downloading' || updateStage === 'retrying' || updateStage === 'installing';
  const [isSponsorHovered, setIsSponsorHovered] = useState(false);
  const [isSponsorPinned, setIsSponsorPinned] = useState(false);
  const [sponsorPopupStyle, setSponsorPopupStyle] = useState<{ top: number; left: number; placement: 'top' | 'bottom' } | null>(null);
  const sponsorRef = useRef<HTMLDivElement | null>(null);
  const sponsorPopupRef = useRef<HTMLDivElement | null>(null);
  const sponsorHoverTimerRef = useRef<number | null>(null);

  const supportMethods = useMemo(
    () => [
      { label: t('aboutPanel.sponsor.methods.wechat'), src: WECHAT_PAY_QR_DATA_URL },
      { label: t('aboutPanel.sponsor.methods.alipay'), src: ALIPAY_QR_DATA_URL },
    ],
    [t],
  );

  const faqItems = useMemo(
    () => t('aboutPanel.faq.items', { returnObjects: true }) as Array<{ question: string; answer: string }>,
    [t],
  );

  const copyText = useCallback(async (value: string) => {
    try {
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(value);
      } else {
        const textarea = document.createElement('textarea');
        textarea.value = value;
        textarea.style.position = 'fixed';
        textarea.style.opacity = '0';
        document.body.appendChild(textarea);
        textarea.focus();
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
      }
      toast.success(t('aboutPanel.contact.copySuccess'), { description: value, duration: 1800 });
    } catch {
      toast.error(t('aboutPanel.contact.copyFailed'));
    }
  }, [t]);

  const checkLatestRelease = useCallback(async (channel: ReleaseChannel = releaseChannel) => {
    setReleaseLoading(true);
    setReleaseError('');

    try {
      const targetRelease: UpdateRelease | null = await apiService.checkLatestRelease(channel);
      if (!targetRelease?.tag_name) {
        setLatestReleaseTag('');
        setLatestReleaseUrl(BRAND.latestReleaseUrl);
        setLatestReleaseBody('');
        setLatestReleaseIsPrerelease(false);
        setInstallerUrl('');
        setReleaseError(channel === 'prerelease'
          ? t('aboutPanel.version.noPrereleaseFound')
          : t('aboutPanel.version.checkFailed'));
        return null;
      }

      setLatestReleaseTag(targetRelease.tag_name || '');
      setLatestReleaseUrl(targetRelease.html_url || BRAND.latestReleaseUrl);
      setLatestReleaseBody(typeof targetRelease?.body === 'string' ? targetRelease.body.trim() : '');
      setLatestReleaseIsPrerelease(!!targetRelease?.prerelease);
      setInstallerUrl(targetRelease.installer_url || '');
      return targetRelease;
    } catch (error) {
      setLatestReleaseTag('');
      setLatestReleaseUrl(BRAND.latestReleaseUrl);
      setLatestReleaseBody('');
      setLatestReleaseIsPrerelease(false);
      setInstallerUrl('');
      setReleaseError(`${t('aboutPanel.version.checkFailed')}: ${error instanceof Error ? error.message : String(error)}`);
      return null;
    } finally {
      setReleaseLoading(false);
    }
  }, [releaseChannel, t]);

  useEffect(() => {
    let disposed = false;
    apiService.getAppVersion()
      .then((value) => {
        if (!disposed) setAppVersion(value || '');
      })
      .catch(() => {
        if (!disposed) setAppVersion('');
      });
    return () => {
      disposed = true;
    };
  }, []);

  const clearSponsorHoverTimer = useCallback(() => {
    if (sponsorHoverTimerRef.current !== null) {
      window.clearTimeout(sponsorHoverTimerRef.current);
      sponsorHoverTimerRef.current = null;
    }
  }, []);

  const handleSponsorHoverEnter = useCallback(() => {
    clearSponsorHoverTimer();
    setIsSponsorHovered(true);
  }, [clearSponsorHoverTimer]);

  const handleSponsorHoverLeave = useCallback(() => {
    clearSponsorHoverTimer();
    sponsorHoverTimerRef.current = window.setTimeout(() => {
      setIsSponsorHovered(false);
      sponsorHoverTimerRef.current = null;
    }, 120);
  }, [clearSponsorHoverTimer]);

  const hasNewVersion = useMemo(() => {
    return !!appVersion && !!latestReleaseTag && !isLatestVersion(appVersion, latestReleaseTag);
  }, [appVersion, latestReleaseTag]);

  const handleCheckUpdate = useCallback(async () => {
    const release = await checkLatestRelease(releaseChannel);
    if (release?.tag_name && appVersion) {
      if (isLatestVersion(appVersion, release.tag_name)) {
        toast.success(t('aboutPanel.version.upToDate'));
      } else {
        toast.info(t('aboutPanel.version.newVersionFound', { version: release.tag_name }));
      }
    }
  }, [appVersion, checkLatestRelease, releaseChannel, t]);

  const handleDownloadInstall = useCallback(async () => {
    if (updateStarting) return;
    let targetTag = latestReleaseTag;
    let targetInstallerUrl = installerUrl;
    if (!targetTag || !targetInstallerUrl) {
      const release = await checkLatestRelease(releaseChannel);
      if (!release) return;
      targetTag = release.tag_name || '';
      targetInstallerUrl = release.installer_url || '';
    }
    if (targetTag && appVersion && isLatestVersion(appVersion, targetTag)) {
      toast.success(t('aboutPanel.version.upToDate'));
      return;
    }
    if (!targetInstallerUrl) {
      toast.error(t('aboutPanel.version.noInstallerHint'));
      return;
    }
    void startUpdate({
      downloadURL: targetInstallerUrl,
      windowTitle: t('aboutPanel.version.updaterWindowTitle'),
      windowBody: t('aboutPanel.version.updaterWindowBody'),
      windowRestarting: t('aboutPanel.version.updaterWindowRestarting'),
    });
  }, [appVersion, checkLatestRelease, installerUrl, latestReleaseTag, releaseChannel, startUpdate, t, updateStarting]);

  const isSponsorOpen = isSponsorHovered || isSponsorPinned;

  useEffect(() => {
    if (!isSponsorPinned) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) {
        return;
      }
      if (!sponsorRef.current?.contains(target) && !sponsorPopupRef.current?.contains(target)) {
        setIsSponsorPinned(false);
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsSponsorPinned(false);
      }
    };

    window.addEventListener('pointerdown', handlePointerDown);
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('pointerdown', handlePointerDown);
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [isSponsorPinned]);

  useEffect(() => {
    return () => {
      clearSponsorHoverTimer();
    };
  }, [clearSponsorHoverTimer]);

  const updateSponsorPopupPosition = useCallback(() => {
    const trigger = sponsorRef.current;
    const popup = sponsorPopupRef.current;
    if (!trigger || !popup) {
      return;
    }

    const gap = 12;
    const viewportPadding = 16;
    const triggerRect = trigger.getBoundingClientRect();
    const popupRect = popup.getBoundingClientRect();
    const width = popupRect.width || 448;
    const height = popupRect.height || 0;

    let left = triggerRect.left + (triggerRect.width / 2) - (width / 2);
    left = Math.max(viewportPadding, Math.min(left, window.innerWidth - width - viewportPadding));

    let top = triggerRect.bottom + gap;
    let placement: 'top' | 'bottom' = 'bottom';

    if (top + height > window.innerHeight - viewportPadding && triggerRect.top - gap - height >= viewportPadding) {
      top = triggerRect.top - height - gap;
      placement = 'top';
    }

    setSponsorPopupStyle({ top, left, placement });
  }, []);

  useLayoutEffect(() => {
    if (!isSponsorOpen) {
      setSponsorPopupStyle(null);
      return;
    }

    const handlePositionChange = () => updateSponsorPopupPosition();
    handlePositionChange();

    window.addEventListener('resize', handlePositionChange);
    window.addEventListener('scroll', handlePositionChange, true);

    let resizeObserver: ResizeObserver | null = null;
    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => handlePositionChange());
      if (sponsorRef.current) {
        resizeObserver.observe(sponsorRef.current);
      }
      if (sponsorPopupRef.current) {
        resizeObserver.observe(sponsorPopupRef.current);
      }
    }

    return () => {
      window.removeEventListener('resize', handlePositionChange);
      window.removeEventListener('scroll', handlePositionChange, true);
      resizeObserver?.disconnect();
    };
  }, [isSponsorOpen, updateSponsorPopupPosition]);

  return (
    <div data-page-reveal="cards" className="mx-auto max-w-[980px] space-y-4">
      <section className="rounded-[28px] border border-border bg-card">
        <div className="flex items-center gap-2 border-b border-border/60 px-5 py-4">
          <Rocket className="h-4 w-4 text-muted-foreground" />
          <h3 className="text-sm font-semibold text-foreground">{t('aboutPanel.title', { name: BRAND.name })}</h3>
        </div>

        <div className="grid items-stretch gap-4 p-5 lg:grid-cols-[minmax(0,1fr)_300px]">
          <div className={`${ABOUT_CARD_CLASS} flex h-full flex-col`}>
            <div className="flex flex-1 flex-col justify-between gap-5">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
                <img src="/brand/appicon.png" alt={t('aboutPanel.images.logoAlt', { name: BRAND.name })} className="h-20 w-20 shrink-0 object-contain" draggable={false} />

                <div className="min-w-0 flex-1">
                  <h2 className="text-xl font-semibold text-foreground">{BRAND.name}</h2>

                  <p className="mt-3 max-w-[36rem] text-sm leading-relaxed text-muted-foreground">
                    {t('aboutPanel.description', { name: BRAND.name })}
                  </p>
                </div>
              </div>

              <div className="rounded-2xl border border-border/70 bg-background/70 p-4">
                <div className="flex flex-wrap gap-2">
                  <span className="relative inline-flex items-center rounded-full border border-border/70 bg-background px-3 py-1 text-xs text-muted-foreground">
                    {t('aboutPanel.version.current', { version: appVersion ? `v${appVersion}` : '--' })}
                    {hasNewVersion && (
                      <span
                        className="pointer-events-none absolute -right-1 -top-1 size-2 shrink-0 rounded-full bg-red-500"
                        aria-label="有新版本"
                        title="有新版本"
                      />
                    )}
                  </span>
                  <span className="inline-flex items-center rounded-full border border-border/70 bg-background px-3 py-1 text-xs text-muted-foreground">
                    {t('aboutPanel.version.latest', { version: releaseLoading ? t('aboutPanel.version.checkingShort') : latestReleaseTag || '--' })}
                  </span>
                  <span className="inline-flex items-center rounded-full border border-border/70 bg-background px-3 py-1 text-xs text-muted-foreground">
                    {releaseChannel === 'prerelease' ? t('aboutPanel.version.channelPrerelease') : t('aboutPanel.version.channelStable')}
                  </span>
                  {latestReleaseIsPrerelease && <Badge variant="info">{t('aboutPanel.version.prereleaseBadge')}</Badge>}
                </div>

                <div className="mt-3 inline-flex rounded-xl border border-border/70 bg-background/70 p-1">
                  <button
                    type="button"
                    className={`rounded-lg px-3 py-1 text-xs transition ${releaseChannel === 'stable' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'}`}
                    onClick={() => setReleaseChannel('stable')}
                    disabled={releaseLoading}
                  >
                    {t('aboutPanel.version.channelStable')}
                  </button>
                  <button
                    type="button"
                    className={`rounded-lg px-3 py-1 text-xs transition ${releaseChannel === 'prerelease' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'}`}
                    onClick={() => setReleaseChannel('prerelease')}
                    disabled={releaseLoading}
                  >
                    {t('aboutPanel.version.channelPrerelease')}
                  </button>
                </div>

                <div data-about-actions className="mt-4 flex flex-wrap items-center gap-2 lg:flex-nowrap">
                  <div data-update-actions className="inline-flex overflow-hidden rounded-lg border border-primary bg-primary shadow-sm">
                    <Button
                      variant="primary"
                      size="sm"
                      loading={updateStarting}
                      disabled={releaseLoading}
                      onClick={() => {
                        void handleDownloadInstall();
                      }}
                      className="rounded-r-none shadow-none hover:opacity-100"
                      icon={<Download className="h-3.5 w-3.5" />}
                    >
                      {updateStage === 'installing'
                        ? t('aboutPanel.version.installing')
                        : updateStarting
                          ? t('aboutPanel.version.downloading', { percent: updatePercent })
                          : t('aboutPanel.version.downloadAndInstall')}
                    </Button>
                    <Button
                      variant="primary"
                      size="sm"
                      loading={releaseLoading}
                      disabled={updateStarting}
                      onClick={() => {
                        void handleCheckUpdate();
                      }}
                      className="rounded-l-none border-l border-primary-foreground/25 shadow-none hover:opacity-100"
                      icon={<RefreshCw className="h-3.5 w-3.5" />}
                    >
                      {releaseLoading ? t('aboutPanel.version.checkingButton') : t('aboutPanel.version.checkUpdate')}
                    </Button>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => openUrl(latestReleaseUrl || BRAND.latestReleaseUrl)}
                    icon={<Rocket className="h-3.5 w-3.5" />}
                  >
                    {t('aboutPanel.version.openReleasePage')}
                  </Button>
                  <div
                    ref={sponsorRef}
                    className="relative"
                    onPointerEnter={handleSponsorHoverEnter}
                    onPointerLeave={handleSponsorHoverLeave}
                  >
                    <Button
                      variant={isSponsorPinned ? 'secondary' : 'outline'}
                      size="sm"
                      onClick={() => {
                        clearSponsorHoverTimer();
                        setIsSponsorHovered(true);
                        setIsSponsorPinned((value) => !value);
                      }}
                      aria-expanded={isSponsorOpen}
                      aria-pressed={isSponsorPinned}
                      icon={<Heart className="h-3.5 w-3.5" />}
                    >
                      {t('aboutPanel.sponsor.button')}
                    </Button>
                  </div>
                </div>

                {releaseError && <div className="mt-3 text-xs text-amber-600 dark:text-amber-300">{releaseError}</div>}
                {hasNewVersion && !installerUrl && !releaseLoading && (
                  <div className="mt-3 text-xs text-muted-foreground">{t('aboutPanel.version.noInstallerHint')}</div>
                )}
              </div>
            </div>

          </div>

          <div className={`${ABOUT_CARD_CLASS} flex h-full min-h-[18rem] flex-col`}>
            <div className="flex items-center gap-2 text-sm font-medium text-foreground">
              <Rocket className="h-4 w-4 text-primary" />
              <span>{t('aboutPanel.contact.title')}</span>
            </div>

            <p className="mt-3 text-xs leading-relaxed text-muted-foreground">
              {t('aboutPanel.contact.tagline')}
            </p>

            <div className="mt-4 overflow-hidden rounded-2xl border border-border/70 bg-background/70">
              <button
                type="button"
                onClick={() => openUrl(BRAND.repositoryUrl)}
                className="flex min-h-12 w-full cursor-pointer items-center justify-between gap-3 px-3 py-3 text-left transition-colors hover:bg-primary/5"
              >
                <span className="flex min-w-0 items-center gap-2 text-sm text-foreground">
                  <Rocket className="h-4 w-4 text-muted-foreground" />
                  {t('aboutPanel.contact.repository')}
                </span>
                <span className="shrink-0 text-xs text-muted-foreground">{t('aboutPanel.contact.repositoryPlatform')}</span>
              </button>

              <button
                type="button"
                onClick={() => {
                  void copyText(SUPPORT_EMAIL);
                }}
                className="flex min-h-12 w-full cursor-pointer items-center justify-between gap-3 border-t border-border/60 px-3 py-3 text-left transition-colors hover:bg-primary/5"
              >
                <span className="flex min-w-0 items-center gap-2 text-sm text-foreground">
                  <Mail className="h-4 w-4 text-muted-foreground" />
                  {t('aboutPanel.contact.email')}
                </span>
                <span className="truncate text-xs text-muted-foreground">{SUPPORT_EMAIL}</span>
              </button>

              <button
                type="button"
                onClick={() => {
                  void copyText(SUPPORT_QQ_GROUP);
                }}
                className="flex min-h-12 w-full cursor-pointer items-center justify-between gap-3 border-t border-border/60 px-3 py-3 text-left transition-colors hover:bg-primary/5"
              >
                <span className="flex min-w-0 items-center gap-2 text-sm text-foreground">
                  <MessageCircle className="h-4 w-4 text-muted-foreground" />
                  {t('aboutPanel.contact.feedbackGroup')}
                </span>
                <span className="truncate text-xs text-muted-foreground">QQ {SUPPORT_QQ_GROUP}</span>
              </button>
            </div>
          </div>

          {hasNewVersion && (
            <div className={`${ABOUT_CARD_CLASS} lg:col-span-2`}>
              <div className="flex flex-wrap items-center gap-2 text-sm font-medium text-foreground">
                <Rocket className="h-4 w-4 text-primary" />
                <span>{t('aboutPanel.version.newVersionFound', { version: latestReleaseTag })}</span>
                {latestReleaseIsPrerelease && <Badge variant="info">{t('aboutPanel.version.prereleaseBadge')}</Badge>}
              </div>

              <div className="mt-3 rounded-2xl border border-border/70 bg-background/70 p-3">
                {latestReleaseBody ? (
                  <ScrollArea className="h-56 pr-2">
                    <div className="flex flex-col gap-2 text-xs leading-relaxed text-foreground/90">
                      {latestReleaseBody.split(/\r?\n/).map((line, index) => {
                        const trimmed = line.trim();
                        if (!trimmed) {
                          return <div key={`release-line-${index}`} className="h-1" />;
                        }

                        if (/^#{1,6}\s+/.test(trimmed)) {
                          return (
                            <div key={`release-line-${index}`} className="pt-1 text-sm font-semibold text-foreground">
                              {trimmed.replace(/^#{1,6}\s+/, '')}
                            </div>
                          );
                        }

                        if (/^[-*]\s+/.test(trimmed) || /^\d+\.\s+/.test(trimmed)) {
                          const content = trimmed.replace(/^[-*]\s+/, '').replace(/^\d+\.\s+/, '');
                          return (
                            <div key={`release-line-${index}`} className="flex items-start gap-2 text-foreground/90">
                              <span className="mt-[1px] text-muted-foreground">-</span>
                              <span>{content}</span>
                            </div>
                          );
                        }

                        return (
                          <p key={`release-line-${index}`} className="text-foreground/85">
                            {trimmed}
                          </p>
                        );
                      })}
                    </div>
                  </ScrollArea>
                ) : (
                  <p className="text-xs text-muted-foreground">{t('aboutPanel.version.emptyReleaseNotes')}</p>
                )}
              </div>
            </div>
          )}

          <div className={`${ABOUT_CARD_CLASS} lg:col-span-2`}>
            <div className="flex items-center gap-2 text-sm font-medium text-foreground">
              <Sparkles className="h-4 w-4 text-primary" />
              <span>{t('aboutPanel.faq.title')}</span>
            </div>

            <div className="mt-4 divide-y divide-border/60 rounded-2xl border border-border/70 bg-background/70">
              {faqItems.map((item) => (
                <div key={item.question} className="px-4 py-3">
                  <div className="text-sm font-medium text-foreground">{item.question}</div>
                  <p className="mt-1.5 text-xs leading-relaxed text-muted-foreground">{item.answer}</p>
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      {isSponsorOpen && typeof document !== 'undefined' && createPortal(
        <div
          ref={sponsorPopupRef}
          onPointerEnter={handleSponsorHoverEnter}
          onPointerLeave={handleSponsorHoverLeave}
          className="fixed z-[80] w-[28rem] max-w-[calc(100vw-2rem)] rounded-3xl border border-border/80 bg-popover/98 p-4 shadow-2xl backdrop-blur-xl animate-in fade-in-0 zoom-in-95"
          style={sponsorPopupStyle ? { top: sponsorPopupStyle.top, left: sponsorPopupStyle.left } : { top: 0, left: 0, visibility: 'hidden' }}
        >
          <div className="mb-3 flex items-center justify-between gap-3 px-1">
            <div className="flex min-w-0 items-center gap-2 text-sm font-medium text-foreground">
              <Heart className="h-4 w-4 text-primary" />
              <span>{t('aboutPanel.sponsor.title')}</span>
            </div>
            {isSponsorPinned && <Badge variant="info">{t('aboutPanel.sponsor.pinned')}</Badge>}
          </div>

          <div className="grid grid-cols-2 gap-3">
            {supportMethods.map((item) => (
              <div key={item.label} className="rounded-2xl border border-border/70 bg-background/80 p-3">
                <div className="mb-2 text-center text-xs font-medium text-muted-foreground">{item.label}</div>
                <img
                  src={item.src}
                  alt={t('aboutPanel.images.supportQrAlt', { label: item.label })}
                  className="mx-auto aspect-square max-h-[190px] w-full rounded-xl border border-border/70 bg-white object-contain p-2"
                  draggable={false}
                />
              </div>
            ))}
          </div>
        </div>,
        document.body,
      )}

    </div>
  );
}
