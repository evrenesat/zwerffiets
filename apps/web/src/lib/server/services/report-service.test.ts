import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { CreateReportPayload } from '$lib/types';
import { ApiError } from '$server/errors';
import { setRepository } from '$server/repositories';
import { MemoryRepository } from '$server/repositories/memory-repository';
import { createTrackingToken } from '$server/security/signed-links';
import { buildPublicUrl } from '$server/env';
import {
  createReport,
  generateExportBatch,
  getExportAsset,
  getReportPublicStatus,
  listOperatorReports,
  listReportEvents,
  updateReportStatus
} from '$server/services/report-service';

const SAMPLE_JPEG_BYTES = new Uint8Array([0xff, 0xd8, 0xff, 0xd9]);
const BASE_TIME_ISO = '2026-01-01T09:00:00.000Z';
const ONE_DAY_MS = 24 * 60 * 60 * 1000;
const RECONFIRMATION_GAP_DAYS = 28;
const RECONFIRMATION_GAP_MS = RECONFIRMATION_GAP_DAYS * ONE_DAY_MS;

const REPORTER_A = 'a'.repeat(64);
const REPORTER_B = 'b'.repeat(64);
const REPORTER_C = 'c'.repeat(64);

const buildPayload = (
  fingerprintHash: string,
  reporterHash: string,
  overrides?: Partial<Pick<CreateReportPayload, 'location' | 'tags' | 'note'>>
): CreateReportPayload => ({
  photos: [
    {
      name: 'bike.jpg',
      mimeType: 'image/jpeg',
      bytes: SAMPLE_JPEG_BYTES
    }
  ],
  location: overrides?.location ?? {
    lat: 52.3676,
    lng: 4.9041,
    accuracy_m: 7
  },
  tags: overrides?.tags ?? ['flat_tires'],
  note: overrides?.note ?? 'Bike appears abandoned',
  clientTs: new Date().toISOString(),
  source: 'web',
  ip: `127.0.0.${(fingerprintHash.charCodeAt(0) % 250) + 1}`,
  fingerprintHash,
  reporterHash
});

describe('report service', () => {
  beforeEach(() => {
    setRepository(new MemoryRepository());
    vi.useFakeTimers();
    vi.setSystemTime(new Date(BASE_TIME_ISO));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('creates reports and suggests duplicates', async () => {
    const first = await createReport(buildPayload('d'.repeat(64), REPORTER_A));
    const second = await createReport(buildPayload('e'.repeat(64), REPORTER_B));

    expect(first.status).toBe('new');
    expect(first.tracking_url).toContain(buildPublicUrl('/report/status/'));
    expect(first.signal_strength).toBe('none');
    expect(second.dedupe_candidates.length).toBeGreaterThan(0);
    expect(second.dedupe_candidates[0]).toBeDefined();
    expect(second.bike_group_id).toBe(first.bike_group_id);
  });

  it('transitions signal strength from none to weak to strong across reconfirmations', async () => {
    const first = await createReport(buildPayload('f'.repeat(64), REPORTER_A));

    vi.setSystemTime(new Date(Date.parse(BASE_TIME_ISO) + RECONFIRMATION_GAP_MS));
    const weak = await createReport(buildPayload('1'.repeat(64), REPORTER_A));

    vi.setSystemTime(new Date(Date.parse(BASE_TIME_ISO) + RECONFIRMATION_GAP_MS * 2));
    const strong = await createReport(buildPayload('2'.repeat(64), REPORTER_B));

    expect(first.signal_strength).toBe('none');
    expect(weak.signal_strength).toBe('weak_same_reporter');
    expect(weak.signal_summary.sameReporterReconfirmations).toBe(1);
    expect(weak.signal_summary.distinctReporterReconfirmations).toBe(0);
    expect(weak.signal_summary.uniqueReporters).toBe(1);
    expect(strong.signal_strength).toBe('strong_distinct_reporters');
    expect(strong.signal_summary.distinctReporterReconfirmations).toBe(1);
    expect(strong.signal_summary.uniqueReporters).toBe(2);
  });

  it('suppresses same-day same-reporter repeats from reconfirmation counters', async () => {
    const first = await createReport(buildPayload('3'.repeat(64), REPORTER_A));
    const second = await createReport(buildPayload('4'.repeat(64), REPORTER_A));

    expect(second.bike_group_id).toBe(first.bike_group_id);
    expect(second.signal_strength).toBe('none');
    expect(second.signal_summary.sameReporterReconfirmations).toBe(0);
    expect(second.signal_summary.distinctReporterReconfirmations).toBe(0);
    expect(second.signal_summary.hasQualifyingReconfirmation).toBe(false);

    const reports = await listOperatorReports({});
    const secondReport = reports.find((report) => report.publicId === second.public_id);
    expect(secondReport).toBeDefined();

    const events = await listReportEvents(secondReport!.id);
    expect(events.some((event) => event.type === 'signal_reconfirmation_ignored_same_day')).toBe(true);
  });

  it('creates separate bike groups when location or tags do not match same-bike rules', async () => {
    const first = await createReport(buildPayload('5'.repeat(64), REPORTER_A));
    const farAwayLocation = { lat: 52.3776, lng: 4.9141, accuracy_m: 7 };

    const second = await createReport(
      buildPayload('6'.repeat(64), REPORTER_B, {
        location: farAwayLocation
      })
    );

    const third = await createReport(
      buildPayload('7'.repeat(64), REPORTER_C, {
        location: { lat: 52.36761, lng: 4.90411, accuracy_m: 7 },
        tags: ['no_chain']
      })
    );

    expect(second.bike_group_id).not.toBe(first.bike_group_id);
    expect(third.bike_group_id).not.toBe(first.bike_group_id);
  });

  it('filters operator reports by signal strength', async () => {
    await createReport(buildPayload('8'.repeat(64), REPORTER_A));
    vi.setSystemTime(new Date(Date.parse(BASE_TIME_ISO) + RECONFIRMATION_GAP_MS));
    await createReport(buildPayload('9'.repeat(64), REPORTER_B));

    await createReport(
      buildPayload('a'.repeat(64), REPORTER_C, {
        location: { lat: 52.3776, lng: 4.9141, accuracy_m: 7 }
      })
    );

    const strongReports = await listOperatorReports({ signal_strength: 'strong_distinct_reporters' });
    const noneReports = await listOperatorReports({ signal_strength: 'none' });

    expect(strongReports.length).toBeGreaterThan(0);
    expect(strongReports.every((report) => report.signal_strength === 'strong_distinct_reporters')).toBe(true);
    expect(noneReports.length).toBeGreaterThan(0);
    expect(noneReports.every((report) => report.signal_strength === 'none')).toBe(true);
  });

  it('rejects invalid lifecycle transitions', async () => {
    const created = await createReport(buildPayload('b'.repeat(64), REPORTER_A));
    const reports = await listOperatorReports({});
    const report = reports.find((entry) => entry.publicId === created.public_id);

    expect(report).toBeDefined();

    await expect(
      updateReportStatus(report!.id, 'resolved', {
        email: 'operator@zwerffiets.local',
        role: 'operator'
      })
    ).rejects.toBeInstanceOf(ApiError);
  });

  it('returns status only when tracking token matches report', async () => {
    const created = await createReport(buildPayload('c'.repeat(64), REPORTER_A));
    const token = await createTrackingToken(created.public_id, '1h');

    const status = await getReportPublicStatus(created.public_id, token);
    expect(status.status).toBe('new');
  });

  it('generates export artifacts and downloads expected format', async () => {
    await createReport(buildPayload('0'.repeat(64), REPORTER_A));

    const batch = await generateExportBatch(
      {
        period_type: 'weekly',
        period_start: '2020-01-01T00:00:00.000Z',
        period_end: '2030-01-01T00:00:00.000Z'
      },
      { email: 'operator@zwerffiets.local', role: 'operator' }
    );

    const csvAsset = await getExportAsset(batch.id, 'csv');
    const geoAsset = await getExportAsset(batch.id, 'geojson');
    const pdfAsset = await getExportAsset(batch.id, 'pdf');

    expect(csvAsset.contentType).toContain('text/csv');
    expect(String(csvAsset.body)).toContain('report_id');
    expect(String(geoAsset.body)).toContain('FeatureCollection');
    expect(pdfAsset.contentType).toBe('application/pdf');
    expect((pdfAsset.body as Uint8Array).byteLength).toBeGreaterThan(0);
  });
});
