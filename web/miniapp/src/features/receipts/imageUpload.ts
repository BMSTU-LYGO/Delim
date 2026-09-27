const MAX_UPLOAD_BYTES = 10 * 1024 * 1024;
const MAX_SOURCE_BYTES = 25 * 1024 * 1024;
const MAX_LONG_EDGE = 1600;
const JPEG_QUALITY = 0.78;

export const supportedReceiptImageTypes = new Set(['image/jpeg', 'image/png', 'image/webp']);

export class ReceiptImageError extends Error {}

function jpegName(name: string) {
  const stem = name.replace(/\.[^/.]+$/, '') || 'receipt';
  return `${stem}.jpg`;
}

function canvasBlob(canvas: HTMLCanvasElement): Promise<Blob | null> {
  return new Promise((resolve) => canvas.toBlob(resolve, 'image/jpeg', JPEG_QUALITY));
}

interface DecodedReceiptImage {
  close(): void;
  height: number;
  source: CanvasImageSource;
  width: number;
}

async function decodeReceiptImage(source: File): Promise<DecodedReceiptImage> {
  if (typeof createImageBitmap === 'function') {
    try {
      const bitmap = await createImageBitmap(source, { imageOrientation: 'from-image' });
      return {
        close: () => bitmap.close(),
        height: bitmap.height,
        source: bitmap,
        width: bitmap.width,
      };
    } catch {
      // Fall through to the image element decoder used by iOS WebViews.
    }
  }

  const url = URL.createObjectURL(source);
  const image = new Image();
  try {
    await new Promise<void>((resolve, reject) => {
      image.onload = () => resolve();
      image.onerror = () => reject(new ReceiptImageError('Не удалось прочитать изображение'));
      image.src = url;
    });
    if (!image.naturalWidth || !image.naturalHeight) {
      throw new ReceiptImageError('Не удалось прочитать изображение');
    }
    return {
      close: () => URL.revokeObjectURL(url),
      height: image.naturalHeight,
      source: image,
      width: image.naturalWidth,
    };
  } catch (cause) {
    URL.revokeObjectURL(url);
    throw cause;
  }
}

/** Produces one oriented, appropriately sized JPEG for OCR. */
export async function prepareReceiptImage(source: File): Promise<File> {
  if (!supportedReceiptImageTypes.has(source.type)) {
    throw new ReceiptImageError('Поддерживаются только JPEG, PNG и WebP');
  }
  if (source.size <= 0 || source.size > MAX_SOURCE_BYTES) {
    throw new ReceiptImageError('Исходный файл должен быть не больше 25 МБ');
  }
  let decoded: DecodedReceiptImage;
  try {
    decoded = await decodeReceiptImage(source);
  } catch {
    if (source.size > MAX_UPLOAD_BYTES) {
      throw new ReceiptImageError('Не удалось подготовить изображение. Выберите файл до 10 МБ');
    }
    return source;
  }
  try {
    const scale = Math.min(1, MAX_LONG_EDGE / Math.max(decoded.width, decoded.height));
    const width = Math.max(1, Math.round(decoded.width * scale));
    const height = Math.max(1, Math.round(decoded.height * scale));
    const canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;
    const context = canvas.getContext('2d');
    if (!context) throw new ReceiptImageError('Не удалось подготовить изображение');
    context.fillStyle = '#fff';
    context.fillRect(0, 0, width, height);
    context.drawImage(decoded.source, 0, 0, width, height);
    const blob = await canvasBlob(canvas);
    if (!blob || blob.size > MAX_UPLOAD_BYTES) throw new ReceiptImageError('Не удалось сжать изображение до 10 МБ');
    return new File([blob], jpegName(source.name), { type: 'image/jpeg', lastModified: source.lastModified });
  } finally {
    decoded.close();
  }
}
