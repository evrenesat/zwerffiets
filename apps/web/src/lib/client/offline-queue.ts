import {
  MAX_NOTE_LENGTH,
  MAX_PHOTO_COUNT,
  MAX_TAG_COUNT,
  MIN_PHOTO_COUNT,
  MIN_TAG_COUNT,
  QUEUE_STORAGE_KEY
} from '$lib/constants';

const OFFLINE_QUEUE_TTL_MS = 24 * 60 * 60 * 1000;

// Offline queue entries persist report payloads (location, tags, note, photos, and optional email/language)
// in localStorage so reports can be retried while offline. Entries older than 24h are discarded.
export interface QueuedReport {
  location: { lat: number; lng: number; accuracy_m: number };
  tags: string[];
  note: string | null;
  photos: string[];
  client_ts: string;
  reporter_email?: string;
  ui_language?: string;
}

const isQueueEntryFresh = (item: QueuedReport, nowMs: number): boolean => {
  const createdAtMs = Date.parse(item.client_ts);
  if (Number.isNaN(createdAtMs)) {
    return false;
  }
  return nowMs - createdAtMs <= OFFLINE_QUEUE_TTL_MS;
};

const isFiniteNumber = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isFinite(value);
};

const isDataURLPhoto = (photo: unknown): photo is string => {
  if (typeof photo !== 'string') {
    return false;
  }
  return /^data:image\/(?:jpeg|webp);base64,/u.test(photo);
};

const isQueueEntryValid = (item: unknown): item is QueuedReport => {
  if (!item || typeof item !== 'object') {
    return false;
  }

  const candidate = item as Partial<QueuedReport>;
  if (!candidate.location || typeof candidate.location !== 'object') {
    return false;
  }

  if (
    !isFiniteNumber(candidate.location.lat) ||
    !isFiniteNumber(candidate.location.lng) ||
    !isFiniteNumber(candidate.location.accuracy_m)
  ) {
    return false;
  }

  if (
    !Array.isArray(candidate.tags) ||
    candidate.tags.length < MIN_TAG_COUNT ||
    candidate.tags.length > MAX_TAG_COUNT ||
    candidate.tags.some((tag) => typeof tag !== 'string' || tag.trim() === '')
  ) {
    return false;
  }

  if (
    !Array.isArray(candidate.photos) ||
    candidate.photos.length < MIN_PHOTO_COUNT ||
    candidate.photos.length > MAX_PHOTO_COUNT ||
    candidate.photos.some((photo) => !isDataURLPhoto(photo))
  ) {
    return false;
  }

  if (typeof candidate.client_ts !== 'string' || Number.isNaN(Date.parse(candidate.client_ts))) {
    return false;
  }

  if (candidate.note !== null && typeof candidate.note !== 'string') {
    return false;
  }
  if (typeof candidate.note === 'string' && candidate.note.length > MAX_NOTE_LENGTH) {
    return false;
  }

  if (
    candidate.reporter_email !== undefined &&
    (typeof candidate.reporter_email !== 'string' || candidate.reporter_email.trim() === '')
  ) {
    return false;
  }

  if (
    candidate.ui_language !== undefined &&
    candidate.ui_language !== 'nl' &&
    candidate.ui_language !== 'en'
  ) {
    return false;
  }

  return true;
};

const readQueue = (): QueuedReport[] => {
  const raw = localStorage.getItem(QUEUE_STORAGE_KEY);
  if (!raw) {
    return [];
  }

  try {
    const parsed = JSON.parse(raw) as QueuedReport[];
    if (!Array.isArray(parsed)) {
      return [];
    }
    const nowMs = Date.now();
    const validEntries = parsed.filter((item) => isQueueEntryValid(item) && isQueueEntryFresh(item, nowMs));
    if (validEntries.length !== parsed.length) {
      writeQueue(validEntries);
    }
    return validEntries;
  } catch {
    return [];
  }
};

const writeQueue = (queue: QueuedReport[]): void => {
  localStorage.setItem(QUEUE_STORAGE_KEY, JSON.stringify(queue));
};

export const enqueueReport = (item: QueuedReport): void => {
  const queue = readQueue();
  queue.push(item);
  writeQueue(queue);
};

type QueuePostResult = 'submitted' | 'retry' | 'drop';

const isPermanentClientFailure = (response: Response): boolean => {
  if (response.status === 429) {
    return false;
  }
  return response.status >= 400 && response.status < 500;
};

const postQueuedReport = async (item: QueuedReport): Promise<QueuePostResult> => {
  const response = await fetch('/api/v1/reports', {
    method: 'POST',
    headers: {
      'content-type': 'application/json'
    },
    body: JSON.stringify(item)
  });

  if (response.ok) {
    return 'submitted';
  }
  if (isPermanentClientFailure(response)) {
    return 'drop';
  }
  return 'retry';
};

export const flushQueue = async (): Promise<void> => {
  const pendingQueue = readQueue();
  if (pendingQueue.length === 0) {
    return;
  }

  for (let index = 0; index < pendingQueue.length; ) {
    const item = pendingQueue[index];
    try {
      const result = await postQueuedReport(item);
      if (result === 'submitted' || result === 'drop') {
        pendingQueue.splice(index, 1);
        writeQueue(pendingQueue);
        continue;
      }
    } catch {
      // Keep queued entry for retry.
    }
    index += 1;
  }

  writeQueue(pendingQueue);
};
