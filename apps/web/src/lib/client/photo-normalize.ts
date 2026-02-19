import {
  PHOTO_UPLOAD_DIMENSION_STEP,
  PHOTO_UPLOAD_INITIAL_JPEG_QUALITY,
  PHOTO_UPLOAD_MAX_DIMENSION_PX,
  PHOTO_UPLOAD_MAX_ENCODING_ATTEMPTS,
  PHOTO_UPLOAD_MIN_JPEG_QUALITY,
  PHOTO_UPLOAD_QUALITY_STEP,
  PHOTO_UPLOAD_TARGET_MAX_BYTES
} from '$lib/constants';

export interface PhotoNormalizationCandidate {
  width: number;
  height: number;
  quality: number;
}

export interface PhotoNormalizationConfig {
  maxBytes: number;
  maxDimensionPx: number;
  initialQuality: number;
  minQuality: number;
  qualityStep: number;
  dimensionStep: number;
  maxAttempts: number;
}

const DEFAULT_CONFIG: PhotoNormalizationConfig = {
  maxBytes: PHOTO_UPLOAD_TARGET_MAX_BYTES,
  maxDimensionPx: PHOTO_UPLOAD_MAX_DIMENSION_PX,
  initialQuality: PHOTO_UPLOAD_INITIAL_JPEG_QUALITY,
  minQuality: PHOTO_UPLOAD_MIN_JPEG_QUALITY,
  qualityStep: PHOTO_UPLOAD_QUALITY_STEP,
  dimensionStep: PHOTO_UPLOAD_DIMENSION_STEP,
  maxAttempts: PHOTO_UPLOAD_MAX_ENCODING_ATTEMPTS
};

export const fitDimensionsWithinMaxEdge = (
  width: number,
  height: number,
  maxDimensionPx: number
): { width: number; height: number } => {
  if (width <= 0 || height <= 0) {
    throw new Error('Invalid source image size');
  }

  const longestEdge = Math.max(width, height);
  if (longestEdge <= maxDimensionPx) {
    return { width, height };
  }

  const scale = maxDimensionPx / longestEdge;
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale))
  };
};

export const buildNormalizationCandidates = (
  sourceWidth: number,
  sourceHeight: number,
  config: PhotoNormalizationConfig = DEFAULT_CONFIG
): PhotoNormalizationCandidate[] => {
  const fitted = fitDimensionsWithinMaxEdge(sourceWidth, sourceHeight, config.maxDimensionPx);
  let width = fitted.width;
  let height = fitted.height;
  let quality = config.initialQuality;

  const candidates: PhotoNormalizationCandidate[] = [];
  for (let attempt = 0; attempt < config.maxAttempts; attempt += 1) {
    candidates.push({ width, height, quality });

    const loweredQuality = Number((quality - config.qualityStep).toFixed(2));
    if (loweredQuality >= config.minQuality) {
      quality = loweredQuality;
      continue;
    }

    const nextWidth = Math.max(1, Math.round(width * config.dimensionStep));
    const nextHeight = Math.max(1, Math.round(height * config.dimensionStep));
    if (nextWidth === width && nextHeight === height) {
      quality = config.minQuality;
      continue;
    }

    width = nextWidth;
    height = nextHeight;
    quality = config.initialQuality;
  }

  return candidates;
};

const toJpegFileName = (fileName: string): string => {
  const trimmed = fileName.trim();
  if (trimmed === '') {
    return 'photo.jpg';
  }
  if (!/\.[^.]+$/u.test(trimmed)) {
    return `${trimmed}.jpg`;
  }
  return trimmed.replace(/\.[^.]+$/u, '.jpg');
};

const loadImage = async (file: File): Promise<HTMLImageElement> => {
  const objectUrl = URL.createObjectURL(file);
  return await new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => {
      URL.revokeObjectURL(objectUrl);
      resolve(image);
    };
    image.onerror = () => {
      URL.revokeObjectURL(objectUrl);
      reject(new Error('Could not decode source image'));
    };
    image.src = objectUrl;
  });
};

const encodeImageToJpegBlob = async (
  image: HTMLImageElement,
  width: number,
  height: number,
  quality: number
): Promise<Blob> => {
  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext('2d');
  if (!context) {
    throw new Error('Canvas rendering context unavailable');
  }

  context.fillStyle = '#ffffff';
  context.fillRect(0, 0, width, height);
  context.drawImage(image, 0, 0, width, height);

  const blob = await new Promise<Blob | null>((resolve) => {
    canvas.toBlob(resolve, 'image/jpeg', quality);
  });

  if (!blob) {
    throw new Error('Failed to encode image');
  }
  return blob;
};

export const normalizePhotoForUpload = async (
  file: File,
  config: PhotoNormalizationConfig = DEFAULT_CONFIG
): Promise<File> => {
  if (file.type !== '' && !file.type.startsWith('image/')) {
    throw new Error('Only image files are supported');
  }

  const image = await loadImage(file);
  const candidates = buildNormalizationCandidates(image.naturalWidth, image.naturalHeight, config);

  let bestBlob: Blob | null = null;
  for (const candidate of candidates) {
    const encoded = await encodeImageToJpegBlob(
      image,
      candidate.width,
      candidate.height,
      candidate.quality
    );
    bestBlob = encoded;
    if (encoded.size <= config.maxBytes) {
      break;
    }
  }

  if (!bestBlob) {
    throw new Error('Failed to encode image');
  }

  return new File([bestBlob], toJpegFileName(file.name), {
    type: 'image/jpeg',
    lastModified: Date.now()
  });
};
