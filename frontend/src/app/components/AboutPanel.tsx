'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import { Heart, Mail, RefreshCw, Rocket, Sparkles } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { BRAND } from '../lib/brand';
import { ALIPAY_QR_DATA_URL, WECHAT_PAY_QR_DATA_URL } from '../lib/support-assets';
import { apiService } from '../services/api';
import { Badge, Button, ScrollArea } from './ui/index';

type ReleaseChannel = 'stable' | 'prerelease';

type GithubRelease = {
  tag_name?: string;
  html_url?: string;
  body?: string;
  prerelease?: boolean;
  draft?: boolean;
};

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

  const parseSemverParts = (value: string): number[] | null => {
    const base = value.split('-')[0].split('+')[0];
    if (!/^\d+(\.\d+){0,3}$/.test(base)) return null;
    return base.split('.').map((part) => Number(part));
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

  const current = currentSemver;
  const latest = latestSemver;
  const length = Math.max(current.length, latest.length);

  for (let index = 0; index < length; index += 1) {
    const currentPart = current[index] ?? 0;
    const latestPart = latest[index] ?? 0;
    if (latestPart > currentPart) return false;
    if (latestPart < currentPart) return true;
  }

  return true;
}

const ABOUT_CARD_CLASS = 'min-w-0 rounded-3xl border border-border/70 bg-card p-5';

export default function AboutPanel() {
  const { t } = useTranslation();
  const [appVersion, setAppVersion] = useState('');
  const [releaseChannel, setReleaseChannel] = useState<ReleaseChannel>('stable');
  const [latestReleaseTag, setLatestReleaseTag] = useState('');
  const [latestReleaseUrl, setLatestReleaseUrl] = useState<string>(BRAND.latestReleaseUrl);
  const [latestReleaseBody, setLatestReleaseBody] = useState('');
  const [latestReleaseIsPrerelease, setLatestReleaseIsPrerelease] = useState(false);
  const [releaseLoading, setReleaseLoading] = useState(false);
  const [releaseError, setReleaseError] = useState('');
  const [sponsorOpen, setSponsorOpen] = useState(false);

  const faqItems = useMemo(
    () => t('aboutPanel.faq.items', { returnObjects: true }) as Array<{ question: string; answer: string }>,
    [t],
  );

  const checkLatestRelease = useCallback(async (channel: ReleaseChannel = releaseChannel) => {
    setReleaseLoading(true);
    setReleaseError('');

    const headers = { Accept: 'application/vnd.github+json' };

    try {
      let targetRelease: GithubRelease | null = null;

      if (channel === 'prerelease') {
        const response = await fetch(`${BRAND.releasesApiUrl}?per_page=30`, { headers });
        if (!response.ok) throw new Error(`HTTP ${response.status}`);

        const releases = (await response.json()) as GithubRelease[];
        targetRelease = (Array.isArray(releases) ? releases : []).find((item) => !item?.draft && !!item?.prerelease) || null;

        if (!targetRelease) {
          setLatestReleaseTag('');
          setLatestReleaseUrl(BRAND.latestReleaseUrl);
          setLatestReleaseBody('');
          setLatestReleaseIsPrerelease(false);
          setReleaseError(t('aboutPanel.version.noPrereleaseFound'));
          return;
        }
      } else {
        const response = await fetch(BRAND.latestReleaseApiUrl, { headers });
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        targetRelease = (await response.json()) as GithubRelease;
      }

      setLatestReleaseTag(targetRelease?.tag_name || '');
      setLatestReleaseUrl(targetRelease?.html_url || BRAND.latestReleaseUrl);
      setLatestReleaseBody(typeof targetRelease?.body === 'string' ? targetRelease.body.trim() : '');
      setLatestReleaseIsPrerelease(!!targetRelease?.prerelease);
    } catch {
      setLatestReleaseTag('');
      setLatestReleaseUrl(BRAND.latestReleaseUrl);
      setLatestReleaseBody('');
      setLatestReleaseIsPrerelease(false);
      setReleaseError(t('aboutPanel.version.checkFailed'));
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

  useEffect(() => {
    void checkLatestRelease(releaseChannel);
  }, [checkLatestRelease, releaseChannel]);

  const hasNewVersion = useMemo(() => {
    return !!appVersion && !!latestReleaseTag && !isLatestVersion(appVersion, latestReleaseTag);
  }, [appVersion, latestReleaseTag]);

  return (
    <div className="mx-auto max-w-[860px] space-y-4">
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

                <div className="mt-4 flex flex-wrap gap-2">
                  <Button
                    variant="primary"
                    size="sm"
                    loading={releaseLoading}
                    onClick={() => {
                      void checkLatestRelease(releaseChannel);
                    }}
                    icon={<RefreshCw className="h-3.5 w-3.5" />}
                  >
                    {releaseLoading ? t('aboutPanel.version.checkingButton') : t('aboutPanel.version.checkUpdate')}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => openUrl(latestReleaseUrl || BRAND.latestReleaseUrl)}
                    icon={<Rocket className="h-3.5 w-3.5" />}
                  >
                    {t('aboutPanel.version.openReleasePage')}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setSponsorOpen((open) => !open)}
                    aria-expanded={sponsorOpen}
                    icon={<Heart className="h-3.5 w-3.5" />}
                  >
                    {t('aboutPanel.sponsor.button')}
                  </Button>
                </div>

                {sponsorOpen && (
                  <div className="mt-4 rounded-2xl border border-primary/20 bg-primary/5 p-4">
                    <div className="mb-3 flex items-center gap-2 text-sm font-medium text-foreground">
                      <Heart className="h-4 w-4 text-primary" />
                      <span>{t('aboutPanel.sponsor.title')}</span>
                    </div>
                    <div className="grid gap-3 sm:grid-cols-2">
                      {[
                        { label: t('aboutPanel.sponsor.methods.wechat'), src: WECHAT_PAY_QR_DATA_URL },
                        { label: t('aboutPanel.sponsor.methods.alipay'), src: ALIPAY_QR_DATA_URL },
                      ].map((item) => (
                        <div key={item.label} className="rounded-2xl border border-border/70 bg-background/70 p-3">
                          <div className="mb-2 text-center text-xs font-medium text-muted-foreground">{item.label}</div>
                          <img
                            src={item.src}
                            alt={t('aboutPanel.images.supportQrAlt', { label: item.label })}
                            className="mx-auto aspect-square max-h-[260px] w-full max-w-[220px] rounded-xl border border-border/70 bg-white object-contain p-2"
                            draggable={false}
                          />
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {releaseError && <div className="mt-3 text-xs text-amber-600 dark:text-amber-300">{releaseError}</div>}
              </div>
            </div>

          </div>

          <div className={`${ABOUT_CARD_CLASS} flex h-full min-h-[18rem] flex-col`}>
            <div className="flex items-center gap-2 text-sm font-medium text-foreground">
              <Rocket className="h-4 w-4 text-primary" />
              <span>{t('aboutPanel.contact.title')}</span>
            </div>

            <div className="mt-4 min-h-[5.5rem] rounded-2xl border border-border/70 bg-background/70 p-3 text-sm leading-relaxed text-muted-foreground">
              {t('aboutPanel.contact.tagline')}
            </div>

            <div className="mt-4 grid flex-1 content-between gap-3">
              <button
                type="button"
                onClick={() => openUrl(BRAND.repositoryUrl)}
                className="flex min-h-12 w-full cursor-pointer items-center justify-between gap-3 rounded-2xl border border-border/70 bg-background/70 px-3 py-2.5 text-left transition-colors hover:border-primary/30 hover:bg-primary/5"
              >
                <span className="flex min-w-0 items-center gap-2 text-sm text-foreground">
                  <Rocket className="h-4 w-4 text-muted-foreground" />
                  {t('aboutPanel.contact.repository')}
                </span>
                <span className="shrink-0 text-xs text-muted-foreground">{t('aboutPanel.contact.repositoryPlatform')}</span>
              </button>

              <div className="flex min-h-12 items-center justify-between gap-3 rounded-2xl border border-border/70 bg-background/70 px-3 py-2.5">
                <span className="flex min-w-0 items-center gap-2 text-sm text-foreground">
                  <Mail className="h-4 w-4 text-muted-foreground" />
                  {t('aboutPanel.contact.email')}
                </span>
                <span className="truncate text-xs text-muted-foreground">1989005183@qq.com</span>
              </div>
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

    </div>
  );
}
