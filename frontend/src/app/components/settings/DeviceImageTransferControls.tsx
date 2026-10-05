'use client';

import { useRef, useState } from 'react';
import { ImageUp } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '../ui';
import { apiService } from '../../services/api';

const MAX_IMAGE_BYTES = 4 * 1024 * 1024;

interface DeviceImageTransferControlsProps {
  isConnected: boolean;
  transport: string;
  canTransfer: boolean;
}

const IMAGE_WIDTH = 428;
const IMAGE_HEIGHT = 142;

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

function encodeRGB565Base64(image: HTMLImageElement, onProgress?: (progress: number) => void): string {
  const canvas = document.createElement('canvas');
  canvas.width = IMAGE_WIDTH;
  canvas.height = IMAGE_HEIGHT;
  const context = canvas.getContext('2d', { willReadFrequently: true });
  if (!context) throw new Error('Image canvas is unavailable');

  // Keep aspect ratio and use a deterministic black letterbox for the fixed panel.
  context.fillStyle = '#000';
  context.fillRect(0, 0, IMAGE_WIDTH, IMAGE_HEIGHT);
  const scale = Math.min(IMAGE_WIDTH / image.naturalWidth, IMAGE_HEIGHT / image.naturalHeight);
  const width = Math.max(1, Math.round(image.naturalWidth * scale));
  const height = Math.max(1, Math.round(image.naturalHeight * scale));
  context.drawImage(image, Math.floor((IMAGE_WIDTH - width) / 2), Math.floor((IMAGE_HEIGHT - height) / 2), width, height);

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
  const [loading, setLoading] = useState(false);
  const [progress, setProgress] = useState(0);

  const handleSelect = async (file: File | undefined) => {
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      toast.error(t('controlPanel.device.imageTransfer.invalidType'));
      return;
    }
    if (file.size > MAX_IMAGE_BYTES) {
      toast.error(t('controlPanel.device.imageTransfer.tooLarge'));
      return;
    }
    if (transport.toLowerCase() === 'ble') {
      toast.info(t('controlPanel.device.imageTransfer.bleWiredOnly'));
      return;
    }
    if (!canTransfer) {
      toast.error(t('controlPanel.device.imageTransfer.unavailable'));
      return;
    }

    setLoading(true);
    setProgress(5);
    let progressTimer: ReturnType<typeof setInterval> | undefined;
    try {
      const image = await readImage(file);
      setProgress(15);
      const dataBase64 = encodeRGB565Base64(image, (encodingProgress) => {
        setProgress(15 + Math.round(encodingProgress * 45));
      });
      setProgress(65);
      progressTimer = setInterval(() => {
        setProgress((current) => Math.min(95, current + 1));
      }, 150);
      await apiService.transferDeviceImage({
        dataBase64,
        fileName: file.name,
        mimeType: file.type,
        format: 'rgb565-be',
        width: IMAGE_WIDTH,
        height: IMAGE_HEIGHT,
      });
      clearInterval(progressTimer);
      progressTimer = undefined;
      setProgress(100);
      toast.success(t('controlPanel.device.imageTransfer.success'));
      await new Promise((resolve) => setTimeout(resolve, 250));
    } catch (error) {
      toast.error(t('controlPanel.device.imageTransfer.failed', {
        error: error instanceof Error ? error.message : String(error),
      }));
    } finally {
      if (progressTimer) clearInterval(progressTimer);
      setLoading(false);
      setProgress(0);
      if (inputRef.current) inputRef.current.value = '';
    }
  };

  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0 flex-1">
        <div className="text-sm font-medium text-foreground">{t('controlPanel.device.imageTransfer.title')}</div>
        <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground">
          {t('controlPanel.device.imageTransfer.description')}
        </div>
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
          </div>
        )}
      </div>
      <input
        ref={inputRef}
        type="file"
        accept="image/*"
        className="hidden"
        disabled={!isConnected || loading}
        onChange={(event) => void handleSelect(event.target.files?.[0])}
      />
      <Button
        type="button"
        variant="outline"
        size="sm"
        icon={<ImageUp className="h-4 w-4" />}
        disabled={!isConnected || loading}
        loading={loading}
        onClick={() => inputRef.current?.click()}
      >
        {t('controlPanel.device.imageTransfer.select')}
      </Button>
    </div>
  );
}
