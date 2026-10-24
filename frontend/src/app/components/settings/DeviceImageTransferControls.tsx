'use client';

import { useEffect, useRef, useState } from 'react';
import { ImageUp, Move, Settings2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, Slider } from '../ui';
import { apiService } from '../../services/api';

const MAX_IMAGE_BYTES = 4 * 1024 * 1024;

interface DeviceImageTransferControlsProps {
  isConnected: boolean;
  transport: string;
  canTransfer: boolean;
}

const IMAGE_WIDTH = 428;
const IMAGE_HEIGHT = 142;
const PREVIEW_MAX_ZOOM = 3;

function getPreviewGeometry(image: HTMLImageElement, zoom: number) {
  const coverScale = Math.max(IMAGE_WIDTH / image.naturalWidth, IMAGE_HEIGHT / image.naturalHeight) * zoom;
  return {
    width: image.naturalWidth * coverScale,
    height: image.naturalHeight * coverScale,
  };
}

function clampPreviewOffset(image: HTMLImageElement, zoom: number, nextOffset: { x: number; y: number }) {
  const { width, height } = getPreviewGeometry(image, zoom);
  return {
    x: Math.max((IMAGE_WIDTH - width) / 2, Math.min((width - IMAGE_WIDTH) / 2, nextOffset.x)),
    y: Math.max((IMAGE_HEIGHT - height) / 2, Math.min((height - IMAGE_HEIGHT) / 2, nextOffset.y)),
  };
}

function readImage(file: File): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    const objectURL = URL.createObjectURL(file);
    image.onload = () => {
      URL.revokeObjectURL(objectURL);
      resolve(image);
    };
    image.onerror = () => {
      URL.revokeObjectURL(objectURL);
      reject(new Error('Unable to decode image'));
    };
    image.src = objectURL;
  });
}

function encodeRGB565Base64(canvas: HTMLCanvasElement, onProgress?: (progress: number) => void): string {
  const context = canvas.getContext('2d', { willReadFrequently: true });
  if (!context) throw new Error('Image canvas is unavailable');

  const pixels = context.getImageData(0, 0, IMAGE_WIDTH, IMAGE_HEIGHT).data;
  const packed = new Uint8Array(IMAGE_WIDTH * IMAGE_HEIGHT * 2);
  for (let source = 0, target = 0; source < pixels.length; source += 4) {
    const value = ((pixels[source] >> 3) << 11)
      | ((pixels[source + 1] >> 2) << 5)
      | (pixels[source + 2] >> 3);
    packed[target++] = value >> 8;
    packed[target++] = value & 0xff;
    if (onProgress && target % 8192 === 0) {
      onProgress(Math.min(1, target / packed.length));
    }
  }
  onProgress?.(1);

  let binary = '';
  for (let index = 0; index < packed.length; index += 1) {
    binary += String.fromCharCode(packed[index]);
  }
  return btoa(binary);
}

export default function DeviceImageTransferControls({
  isConnected,
  transport,
  canTransfer,
}: DeviceImageTransferControlsProps) {
  const { t } = useTranslation();
  const inputRef = useRef<HTMLInputElement>(null);
  const cancelRequestedRef = useRef(false);
  const [loading, setLoading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [cancelRequested, setCancelRequested] = useState(false);
  const [previewFile, setPreviewFile] = useState<File | null>(null);
  const [previewImage, setPreviewImage] = useState<HTMLImageElement | null>(null);
  const [previewUrl, setPreviewUrl] = useState('');
  const [zoom, setZoom] = useState(1);
  const [offset, setOffset] = useState({ x: 0, y: 0 });
  const cropAreaRef = useRef<HTMLDivElement>(null);
  const dragRef = useRef<{ x: number; y: number; offsetX: number; offsetY: number } | null>(null);
  const wired = transport.toLowerCase() === 'usb' || transport.toLowerCase() === 'wifi';

  useEffect(() => apiService.onDeviceImageTransferProgress((payload) => {
    const backendProgress = Math.max(0, Math.min(100, Number(payload.percent) || 0));
    setProgress((current) => {
      const next = payload.stage === 'committing'
        ? 98
        : payload.stage === 'completed'
          ? 100
          : 60 + Math.round(backendProgress * 0.4);
      return Math.max(current, next);
    });
  }), []);

  const openPreview = async (file: File | undefined) => {
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      toast.error(t('controlPanel.device.imageTransfer.invalidType'));
      return;
    }
    if (file.size > MAX_IMAGE_BYTES) {
      toast.error(t('controlPanel.device.imageTransfer.tooLarge'));
      return;
    }
    if (!wired) {
      toast.info(t('controlPanel.device.imageTransfer.bleWiredOnly'));
      return;
    }
    if (!canTransfer) {
      toast.error(t('controlPanel.device.imageTransfer.unavailable'));
      return;
    }

    try {
      const image = await readImage(file);
      setPreviewFile(file);
      setPreviewImage(image);
      setPreviewUrl(URL.createObjectURL(file));
      setZoom(1);
      setOffset({ x: 0, y: 0 });
    } catch (error) {
      toast.error(t('controlPanel.device.imageTransfer.failed', { error: error instanceof Error ? error.message : String(error) }));
    }
  };

  const applyImage = async () => {
    if (!previewImage || !previewFile) return;
    setPreviewFile(null);
    setPreviewImage(null);
    if (previewUrl) URL.revokeObjectURL(previewUrl);
    setPreviewUrl('');
    setLoading(true);
    setCancelRequested(false);
    cancelRequestedRef.current = false;
    setProgress(5);
    try {
      const canvas = document.createElement('canvas');
      canvas.width = IMAGE_WIDTH;
      canvas.height = IMAGE_HEIGHT;
      const context = canvas.getContext('2d');
      if (!context) throw new Error('Image canvas is unavailable');
      context.fillStyle = '#000';
      context.fillRect(0, 0, IMAGE_WIDTH, IMAGE_HEIGHT);
      const { width, height } = getPreviewGeometry(previewImage, zoom);
      const boundedOffset = clampPreviewOffset(previewImage, zoom, offset);
      context.drawImage(previewImage, (IMAGE_WIDTH - width) / 2 + boundedOffset.x, (IMAGE_HEIGHT - height) / 2 + boundedOffset.y, width, height);
      const dataBase64 = encodeRGB565Base64(canvas, (encodingProgress) => setProgress(15 + Math.round(encodingProgress * 45)));
      setProgress(65);
      await apiService.transferDeviceImage({ dataBase64, fileName: previewFile.name, mimeType: previewFile.type, format: 'rgb565-be', width: IMAGE_WIDTH, height: IMAGE_HEIGHT });
      setProgress(100);
      toast.success(t('controlPanel.device.imageTransfer.success'));
      await new Promise((resolve) => setTimeout(resolve, 250));
    } catch (error) {
      if (cancelRequestedRef.current) toast.info(t('controlPanel.device.imageTransfer.canceled'));
      else toast.error(t('controlPanel.device.imageTransfer.failed', { error: error instanceof Error ? error.message : String(error) }));
    } finally {
      setLoading(false);
      setCancelRequested(false);
      cancelRequestedRef.current = false;
      setProgress(0);
      if (inputRef.current) inputRef.current.value = '';
    }
  };

  const handleCancel = async () => {
    if (!loading || cancelRequested) return;
    setCancelRequested(true);
    cancelRequestedRef.current = true;
    try {
      await apiService.cancelDeviceImageTransfer();
    } catch (error) {
      setCancelRequested(false);
      cancelRequestedRef.current = false;
      toast.error(t('controlPanel.device.imageTransfer.cancelFailed', {
        error: error instanceof Error ? error.message : String(error),
      }));
    }
  };

  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0 flex-1">
        <div className="text-sm font-medium text-foreground">{t('controlPanel.device.imageTransfer.title')}</div>
        <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground">
          {t('controlPanel.device.imageTransfer.description')}
        </div>
        <div className="mt-1 text-xs leading-relaxed text-muted-foreground">
          {t('controlPanel.device.imageTransfer.slowWarning')}
        </div>
        {!wired && isConnected && (
          <div className="mt-1 text-xs text-muted-foreground">
            {t('controlPanel.device.imageTransfer.bleWiredOnly')}
          </div>
        )}
        {loading && (
          <div className="mt-2 max-w-md" aria-live="polite">
            <div className="mb-1 flex items-center justify-between text-xs text-muted-foreground">
              <span>{t('controlPanel.device.imageTransfer.progress')}</span>
              <span>{progress}%</span>
            </div>
            <div
              className="h-1.5 w-full overflow-hidden rounded-full bg-muted"
              role="progressbar"
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={progress}
              aria-label={t('controlPanel.device.imageTransfer.progress')}
            >
              <div
                className="h-full rounded-full bg-primary transition-[width] duration-150 ease-out"
                style={{ width: `${progress}%` }}
              />
            </div>
            <button
              type="button"
              className="mt-2 text-xs font-medium text-destructive transition hover:text-destructive/80 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={cancelRequested}
              onClick={() => void handleCancel()}
            >
              {cancelRequested
                ? t('controlPanel.device.imageTransfer.canceling')
                : t('controlPanel.device.imageTransfer.cancel')}
            </button>
          </div>
        )}
      </div>
      <input
        ref={inputRef}
        type="file"
        accept="image/*"
        className="hidden"
        disabled={!isConnected || !wired || loading}
        onChange={(event) => void openPreview(event.target.files?.[0])}
      />
      <Button
        type="button"
        variant="outline"
        size="sm"
        icon={<ImageUp className="h-4 w-4" />}
        disabled={!isConnected || !wired || loading}
        loading={loading}
        onClick={() => inputRef.current?.click()}
      >
        {t('controlPanel.device.imageTransfer.select')}
      </Button>
      <Dialog open={!!previewImage} onOpenChange={(open) => { if (!open && !loading) { setPreviewImage(null); setPreviewFile(null); if (previewUrl) URL.revokeObjectURL(previewUrl); setPreviewUrl(''); } }}>
        <DialogContent className="max-w-4xl">
          <DialogHeader><DialogTitle>{t('controlPanel.device.imageTransfer.cropTitle')}</DialogTitle><DialogDescription>{t('controlPanel.device.imageTransfer.cropDescription')}</DialogDescription></DialogHeader>
          <div className="mx-auto w-full max-w-[760px]">
            <div
              ref={cropAreaRef}
              className="relative aspect-[428/142] w-full cursor-move touch-none overflow-hidden rounded-lg border-2 border-primary/60 bg-black shadow-inner"
              onWheel={(event) => {
                event.preventDefault();
                if (!previewImage) return;
                const nextZoom = Math.max(1, Math.min(PREVIEW_MAX_ZOOM, zoom - event.deltaY * 0.001));
                setZoom(nextZoom);
                setOffset((current) => clampPreviewOffset(previewImage, nextZoom, current));
              }}
              onPointerDown={(event) => { event.currentTarget.setPointerCapture(event.pointerId); dragRef.current = { x: event.clientX, y: event.clientY, offsetX: offset.x, offsetY: offset.y }; }}
              onPointerMove={(event) => {
                if (!dragRef.current || !cropAreaRef.current) return;
                const nextOffset = {
                  x: dragRef.current.offsetX + (event.clientX - dragRef.current.x) * IMAGE_WIDTH / cropAreaRef.current.clientWidth,
                  y: dragRef.current.offsetY + (event.clientY - dragRef.current.y) * IMAGE_HEIGHT / cropAreaRef.current.clientHeight,
                };
                setOffset(clampPreviewOffset(previewImage!, zoom, nextOffset));
              }}
              onPointerUp={() => { dragRef.current = null; }}
              onPointerCancel={() => { dragRef.current = null; }}
            >
              {previewUrl && previewImage && (() => {
                const { width, height } = getPreviewGeometry(previewImage, zoom);
                return <img src={previewUrl} alt="" className="pointer-events-none absolute max-w-none select-none" style={{ left: `${50 + offset.x / IMAGE_WIDTH * 100}%`, top: `${50 + offset.y / IMAGE_HEIGHT * 100}%`, width: `${width / IMAGE_WIDTH * 100}%`, height: `${height / IMAGE_HEIGHT * 100}%`, transform: 'translate(-50%, -50%)' }} />;
              })()}
              <div className="pointer-events-none absolute inset-0 flex items-end justify-end p-2 text-[10px] text-white/70"><Move className="mr-1 h-3 w-3" />{t('controlPanel.device.imageTransfer.dragHint')}</div>
            </div>
            <Slider value={zoom} min={1} max={PREVIEW_MAX_ZOOM} step={0.01} label={t('controlPanel.device.imageTransfer.zoom')} valueFormatter={(value) => `${Math.round(value * 100)}%`} onChange={(nextZoom) => { setZoom(nextZoom); if (previewImage) setOffset((current) => clampPreviewOffset(previewImage, nextZoom, current)); }} />
          </div>
          <DialogFooter><Button variant="secondary" onClick={() => { setPreviewImage(null); setPreviewFile(null); if (previewUrl) URL.revokeObjectURL(previewUrl); setPreviewUrl(''); }}>{t('common.actions.cancel')}</Button><Button icon={<Settings2 className="h-4 w-4" />} onClick={() => void applyImage()}>{t('controlPanel.device.imageTransfer.applyImage')}</Button></DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
