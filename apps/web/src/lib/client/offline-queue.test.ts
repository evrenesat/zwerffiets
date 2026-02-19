import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { QUEUE_STORAGE_KEY } from '$lib/constants';
import { enqueueReport, flushQueue, type QueuedReport } from './offline-queue';

class MemoryStorage implements Storage {
  private data = new Map<string, string>();

  clear(): void {
    this.data.clear();
  }

  getItem(key: string): string | null {
    return this.data.get(key) ?? null;
  }

  key(index: number): string | null {
    return [...this.data.keys()][index] ?? null;
  }

  removeItem(key: string): void {
    this.data.delete(key);
  }

  setItem(key: string, value: string): void {
    this.data.set(key, value);
  }

  get length(): number {
    return this.data.size;
  }
}

const buildQueuedReport = (clientTs: string): QueuedReport => ({
  location: { lat: 52.37, lng: 4.9, accuracy_m: 10 },
  tags: ['flat_tires'],
  note: null,
  photos: ['data:image/jpeg;base64,ZmFrZS1waG90bw=='],
  client_ts: clientTs,
  reporter_email: 'reporter@example.com',
  ui_language: 'en'
});

describe('offline queue', () => {
  let storage: MemoryStorage;

  beforeEach(() => {
    storage = new MemoryStorage();
    Object.defineProperty(globalThis, 'localStorage', {
      configurable: true,
      value: storage
    });
    vi.stubGlobal('fetch', vi.fn());
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-02-19T10:00:00.000Z'));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it('removes expired queue entries older than 24h', async () => {
    const stale = buildQueuedReport('2026-02-18T08:59:59.000Z');
    const fresh = buildQueuedReport('2026-02-19T09:30:00.000Z');
    localStorage.setItem(QUEUE_STORAGE_KEY, JSON.stringify([stale, fresh]));

    vi.mocked(fetch).mockRejectedValue(new Error('offline'));

    await flushQueue();

    const saved = JSON.parse(localStorage.getItem(QUEUE_STORAGE_KEY) ?? '[]') as QueuedReport[];
    expect(saved).toHaveLength(1);
    expect(saved[0].client_ts).toBe(fresh.client_ts);
  });

  it('removes successfully submitted entries from storage', async () => {
    enqueueReport(buildQueuedReport('2026-02-19T09:59:59.000Z'));

    vi.mocked(fetch).mockResolvedValue({ ok: true } as Response);

    await flushQueue();

    expect(localStorage.getItem(QUEUE_STORAGE_KEY)).toBe('[]');
  });

  it('keeps failed entries for retry', async () => {
    enqueueReport(buildQueuedReport('2026-02-19T09:59:59.000Z'));

    vi.mocked(fetch).mockResolvedValue({ ok: false, status: 503 } as Response);

    await flushQueue();

    const saved = JSON.parse(localStorage.getItem(QUEUE_STORAGE_KEY) ?? '[]') as QueuedReport[];
    expect(saved).toHaveLength(1);
    expect(saved[0].photos).toEqual(['data:image/jpeg;base64,ZmFrZS1waG90bw==']);
  });

  it('drops malformed queue entries before sending', async () => {
    localStorage.setItem(
      QUEUE_STORAGE_KEY,
      JSON.stringify([
        {
          ...buildQueuedReport('2026-02-19T09:59:59.000Z'),
          photos: ['not-a-data-url']
        }
      ])
    );

    await flushQueue();

    expect(fetch).not.toHaveBeenCalled();
    expect(localStorage.getItem(QUEUE_STORAGE_KEY)).toBe('[]');
  });

  it('drops permanent client failures and keeps queue moving', async () => {
    enqueueReport(buildQueuedReport('2026-02-19T09:59:59.000Z'));
    enqueueReport(buildQueuedReport('2026-02-19T09:59:58.000Z'));

    vi.mocked(fetch)
      .mockResolvedValueOnce({ ok: false, status: 400 } as Response)
      .mockResolvedValueOnce({ ok: true, status: 201 } as Response);

    await flushQueue();

    expect(localStorage.getItem(QUEUE_STORAGE_KEY)).toBe('[]');
  });
});
