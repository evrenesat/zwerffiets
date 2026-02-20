import { DateTime } from 'luxon';
import {
  FINGERPRINT_BURST_THRESHOLD,
  PHOTO_RETENTION_DAYS,
  REPORT_RATE_LIMIT_REQUESTS,
  REPORT_RATE_LIMIT_WINDOW_MS,
  SIGNAL_RECONFIRMATION_GAP_DAYS,
  STATUS_TRANSITIONS,
  STRONG_SIGNAL_MIN_UNIQUE_REPORTERS,
  TRACKING_LINK_TTL_DAYS
} from '$lib/constants';
import type {
  BikeGroup,
  CreateReportPayload,
  ExportBatch,
  OperatorReportFilters,
  OperatorReportView,
  OperatorSession,
  Report,
  ReportCreateResponse,
  ReportSignalSummary,
  ReportStatus,
  ReporterMatchKind,
  SignalDetails,
  SignalStrength,
  SignalTimelineEntry
} from '$lib/types';
import {
  dedupeLookbackStartIso,
  scoreDuplicateCandidate,
  scoreSignalGroupCandidate,
  signalLookbackStartIso
} from '$server/dedupe';
import { buildPublicUrl } from '$server/env';
import { ApiError } from '$server/errors';
import { stripExifMetadata } from '$server/media/exif';
import { getRepository } from '$server/repositories';
import { checkRateLimit } from '$server/security/rate-limit';
import { createTrackingToken, verifyTrackingToken } from '$server/security/signed-links';
import {
  createReportSchema,
  generateExportSchema,
  mergeReportsSchema,
  reportStatusUpdateSchema
} from '$server/validation';
import { EXPORT_TIMEZONE } from '$lib/constants';
import { buildExportArtifacts } from '$server/services/export-artifacts';
import { getExportRecipients, getMailer } from '$server/services/mailer';

interface FingerprintBucket {
  startsAtMs: number;
  count: number;
}

interface ReconfirmationComputation {
  summary: ReportSignalSummary;
  signalStrength: SignalStrength;
  classificationByReportId: Record<number, 'initial' | 'ignored_same_day' | 'counted_same_reporter' | 'counted_distinct_reporter' | 'non_qualifying'
  >;
}

const SIGNAL_STRENGTH_PRIORITY: Record<SignalStrength, number> = {
  none: 0,
  weak_same_reporter: 1,
  strong_distinct_reporters: 2
};

const fingerprintWindows = new Map<string, FingerprintBucket>();

const applyFingerprintHeuristic = (fingerprintHash: string, nowMs: number): boolean => {
  const existing = fingerprintWindows.get(fingerprintHash);

  if (!existing || nowMs - existing.startsAtMs > REPORT_RATE_LIMIT_WINDOW_MS) {
    fingerprintWindows.set(fingerprintHash, { startsAtMs: nowMs, count: 1 });
    return false;
  }

  existing.count += 1;
  fingerprintWindows.set(fingerprintHash, existing);
  return existing.count >= FINGERPRINT_BURST_THRESHOLD;
};

const ensureTransitionAllowed = (from: ReportStatus, to: ReportStatus): void => {
  const allowed = STATUS_TRANSITIONS[from]?.includes(to);
  if (!allowed) {
    throw new ApiError(400, 'invalid_status_transition', `Cannot transition from ${from} to ${to}`);
  }
};

const getReportWindow = (
  periodType: 'weekly' | 'monthly',
  requestedStart?: string | null,
  requestedEnd?: string | null
): { periodStart: string; periodEnd: string } => {
  if (requestedStart && requestedEnd) {
    return { periodStart: requestedStart, periodEnd: requestedEnd };
  }

  const now = DateTime.now().setZone(EXPORT_TIMEZONE);

  if (periodType === 'weekly') {
    const previousWeek = now.minus({ weeks: 1 });
    const periodStart = previousWeek.startOf('week').toUTC().toISO();
    const periodEnd = previousWeek.endOf('week').toUTC().toISO();

    if (!periodStart || !periodEnd) {
      throw new ApiError(500, 'period_resolution_failed', 'Unable to compute weekly period');
    }

    return { periodStart, periodEnd };
  }

  const previousMonth = now.minus({ months: 1 });
  const periodStart = previousMonth.startOf('month').toUTC().toISO();
  const periodEnd = previousMonth.endOf('month').toUTC().toISO();

  if (!periodStart || !periodEnd) {
    throw new ApiError(500, 'period_resolution_failed', 'Unable to compute monthly period');
  }

  return { periodStart, periodEnd };
};

const getTrackingExpiry = (): string => `${TRACKING_LINK_TTL_DAYS}d`;

const sameUtcDay = (leftIso: string, rightIso: string): boolean => {
  return DateTime.fromISO(leftIso).toUTC().toISODate() === DateTime.fromISO(rightIso).toUTC().toISODate();
};

const computeSignalStrength = (summary: ReportSignalSummary): SignalStrength => {
  if (
    summary.distinctReporterReconfirmations > 0 &&
    summary.uniqueReporters >= STRONG_SIGNAL_MIN_UNIQUE_REPORTERS
  ) {
    return 'strong_distinct_reporters';
  }

  if (summary.hasQualifyingReconfirmation) {
    return 'weak_same_reporter';
  }

  return 'none';
};

const computeReconfirmation = (reports: Report[]): ReconfirmationComputation => {
  const sortedReports = [...reports].sort((left, right) => left.createdAt.localeCompare(right.createdAt));
  const classificationByReportId: ReconfirmationComputation['classificationByReportId'] = {};

  let sameReporterReconfirmations = 0;
  let distinctReporterReconfirmations = 0;
  let firstQualifyingReconfirmationAt: string | null = null;
  let lastQualifyingReconfirmationAt: string | null = null;

  for (let index = 0; index < sortedReports.length; index += 1) {
    const current = sortedReports[index];

    if (index === 0) {
      classificationByReportId[current.id] = 'initial';
      continue;
    }

    const previousReports = sortedReports.slice(0, index);
    const previous = previousReports[previousReports.length - 1];
    const sameDayRepeatBySameReporter = previousReports.some((candidate) => {
      return candidate.reporterHash === current.reporterHash && sameUtcDay(candidate.createdAt, current.createdAt);
    });

    if (sameDayRepeatBySameReporter) {
      classificationByReportId[current.id] = 'ignored_same_day';
      continue;
    }

    const gapDays = DateTime.fromISO(current.createdAt)
      .diff(DateTime.fromISO(previous.createdAt), 'days')
      .days;

    if (gapDays < SIGNAL_RECONFIRMATION_GAP_DAYS) {
      classificationByReportId[current.id] = 'non_qualifying';
      continue;
    }

    const priorReporterHashes = new Set(sortedReports.slice(0, index).map((report) => report.reporterHash));
    const isSameReporter = priorReporterHashes.has(current.reporterHash);

    if (isSameReporter) {
      sameReporterReconfirmations += 1;
      classificationByReportId[current.id] = 'counted_same_reporter';
    } else {
      distinctReporterReconfirmations += 1;
      classificationByReportId[current.id] = 'counted_distinct_reporter';
    }

    if (!firstQualifyingReconfirmationAt) {
      firstQualifyingReconfirmationAt = current.createdAt;
    }
    lastQualifyingReconfirmationAt = current.createdAt;
  }

  const uniqueReporters = new Set(sortedReports.map((report) => report.reporterHash)).size;
  const summary: ReportSignalSummary = {
    totalReports: sortedReports.length,
    uniqueReporters,
    sameReporterReconfirmations,
    distinctReporterReconfirmations,
    firstQualifyingReconfirmationAt,
    lastQualifyingReconfirmationAt,
    lastReportAt: sortedReports[sortedReports.length - 1]?.createdAt ?? new Date().toISOString(),
    hasQualifyingReconfirmation:
      sameReporterReconfirmations + distinctReporterReconfirmations > 0
  };

  return {
    summary,
    signalStrength: computeSignalStrength(summary),
    classificationByReportId
  };
};

const applySummaryToBikeGroup = (group: BikeGroup, summary: ReportSignalSummary, signalStrength: SignalStrength): BikeGroup => {
  return {
    ...group,
    updatedAt: new Date().toISOString(),
    lastReportAt: summary.lastReportAt,
    totalReports: summary.totalReports,
    uniqueReporters: summary.uniqueReporters,
    sameReporterReconfirmations: summary.sameReporterReconfirmations,
    distinctReporterReconfirmations: summary.distinctReporterReconfirmations,
    firstQualifyingReconfirmationAt: summary.firstQualifyingReconfirmationAt,
    lastQualifyingReconfirmationAt: summary.lastQualifyingReconfirmationAt,
    signalStrength
  };
};

const bikeGroupToSignalSummary = (group: BikeGroup): ReportSignalSummary => {
  return {
    totalReports: group.totalReports,
    uniqueReporters: group.uniqueReporters,
    sameReporterReconfirmations: group.sameReporterReconfirmations,
    distinctReporterReconfirmations: group.distinctReporterReconfirmations,
    firstQualifyingReconfirmationAt: group.firstQualifyingReconfirmationAt,
    lastQualifyingReconfirmationAt: group.lastQualifyingReconfirmationAt,
    lastReportAt: group.lastReportAt,
    hasQualifyingReconfirmation:
      group.sameReporterReconfirmations + group.distinctReporterReconfirmations > 0
  };
};

const selectBikeGroupForReport = async (
  incoming: Pick<CreateReportPayload, 'location' | 'tags'>,
  nowIso: string
): Promise<BikeGroup | null> => {
  const repository = getRepository();
  const sinceIso = signalLookbackStartIso(nowIso);
  const candidates = (await repository.listReportsSince(sinceIso)).filter(
    (report) => report.status !== 'invalid'
  );

  const bestScoreByGroup = new Map<number, number>();

  for (const candidate of candidates) {
    const score = scoreSignalGroupCandidate(incoming, candidate, nowIso);
    if (score === null) {
      continue;
    }

    const currentBest = bestScoreByGroup.get(candidate.bikeGroupId) ?? -1;
    if (score > currentBest) {
      bestScoreByGroup.set(candidate.bikeGroupId, score);
    }
  }

  if (bestScoreByGroup.size === 0) {
    return null;
  }

  const bestGroup = [...bestScoreByGroup.entries()].sort((left, right) => right[1] - left[1])[0];
  return repository.getBikeGroupById(bestGroup[0]);
};

const buildSignalDetails = (reports: Report[], group: BikeGroup): SignalDetails => {
  const sortedReports = [...reports].sort((left, right) => left.createdAt.localeCompare(right.createdAt));
  const reconfirmation = computeReconfirmation(sortedReports);
  const reporterHashes = [...new Set(sortedReports.map((report) => report.reporterHash))];

  const labelByReporter = new Map<string, string>();
  reporterHashes.forEach((reporterHash, index) => {
    const alphabetLabel = String.fromCharCode(65 + (index % 26));
    const suffix = index >= 26 ? Math.floor(index / 26) + 1 : '';
    labelByReporter.set(reporterHash, `Reporter ${alphabetLabel}${suffix}`);
  });

  const timeline: SignalTimelineEntry[] = sortedReports.map((report) => {
    const classification = reconfirmation.classificationByReportId[report.id] ?? 'initial';

    let reporterMatchKind: ReporterMatchKind | null = null;
    if (classification === 'counted_same_reporter') {
      reporterMatchKind = 'same_reporter';
    }

    if (classification === 'counted_distinct_reporter') {
      reporterMatchKind = 'distinct_reporter';
    }

    return {
      reportId: report.id,
      publicId: report.publicId,
      createdAt: report.createdAt,
      reporterLabel: labelByReporter.get(report.reporterHash) ?? 'Reporter',
      reporterMatchKind,
      qualified:
        classification === 'counted_same_reporter' || classification === 'counted_distinct_reporter',
      ignoredSameDay: classification === 'ignored_same_day'
    };
  });

  return {
    bikeGroup: group,
    signalSummary: bikeGroupToSignalSummary(group),
    signalStrength: group.signalStrength,
    timeline
  };
};

const enrichReportsWithSignal = async (reports: Report[]): Promise<OperatorReportView[]> => {
  const repository = getRepository();

  return Promise.all(
    reports.map(async (report) => {
      const bikeGroup = await repository.getBikeGroupById(report.bikeGroupId);

      if (!bikeGroup) {
        throw new ApiError(500, 'bike_group_missing', `Bike group not found for report ${report.id}`);
      }

      return {
        ...report,
        bike_group_id: bikeGroup.id,
        signal_summary: bikeGroupToSignalSummary(bikeGroup),
        signal_strength: bikeGroup.signalStrength,
        preview_photo_url: null
      } satisfies OperatorReportView;
    })
  );
};

const applySignalFilters = (
  reports: OperatorReportView[],
  filters: OperatorReportFilters
): OperatorReportView[] => {
  return reports.filter((report) => {
    if (filters.signal_strength && report.signal_strength !== filters.signal_strength) {
      return false;
    }

    if (filters.strong_only && report.signal_strength !== 'strong_distinct_reporters') {
      return false;
    }

    if (
      typeof filters.has_qualifying_reconfirmation === 'boolean' &&
      report.signal_summary.hasQualifyingReconfirmation !== filters.has_qualifying_reconfirmation
    ) {
      return false;
    }

    return true;
  });
};

const sortReportsBySignal = (reports: OperatorReportView[]): OperatorReportView[] => {
  return [...reports].sort((left, right) => {
    const bySignal =
      SIGNAL_STRENGTH_PRIORITY[right.signal_strength] - SIGNAL_STRENGTH_PRIORITY[left.signal_strength];

    if (bySignal !== 0) {
      return bySignal;
    }

    return right.createdAt.localeCompare(left.createdAt);
  });
};

export const createReport = async (payload: CreateReportPayload): Promise<ReportCreateResponse> => {
  const validatedPayload = createReportSchema.parse(payload);

  const rateLimitResult = checkRateLimit(
    `report:${validatedPayload.ip}`,
    REPORT_RATE_LIMIT_REQUESTS,
    REPORT_RATE_LIMIT_WINDOW_MS
  );

  if (!rateLimitResult.allowed) {
    throw new ApiError(429, 'rate_limited', 'Too many reports from this IP. Please retry later.');
  }

  const repository = getRepository();
  const activeTagCodes = new Set((await repository.getTags()).filter((tag) => tag.isActive).map((tag) => tag.code));

  for (const tag of validatedPayload.tags) {
    if (!activeTagCodes.has(tag)) {
      throw new ApiError(400, 'invalid_tag', `Unknown or inactive tag: ${tag}`);
    }
  }

  const sanitizedPhotos = validatedPayload.photos.map((photo) => ({
    ...photo,
    bytes: stripExifMetadata(photo.bytes, photo.mimeType)
  }));

  const nowIso = new Date().toISOString();
  const matchedBikeGroup = await selectBikeGroupForReport(validatedPayload, nowIso);
  const bikeGroup = matchedBikeGroup ?? (await repository.createBikeGroup(validatedPayload.location));

  const report = await repository.createReport({
    ...validatedPayload,
    photos: sanitizedPhotos,
    bikeGroupId: bikeGroup.id
  });
  await repository.saveReportPhotos(report.id, sanitizedPhotos);

  await repository.addEvent({
    reportId: report.id,
    type: 'created',
    actor: 'citizen_anonymous',
    metadata: {
      source: validatedPayload.source,
      retention_days: PHOTO_RETENTION_DAYS,
      bike_group_id: bikeGroup.id
    }
  });

  const bikeGroupReports = await repository.listReportsByBikeGroupId(bikeGroup.id);
  const recomputation = computeReconfirmation(bikeGroupReports);
  const previousSignalStrength = bikeGroup.signalStrength;
  const updatedGroup = applySummaryToBikeGroup(bikeGroup, recomputation.summary, recomputation.signalStrength);
  await repository.updateBikeGroup(updatedGroup);

  const reportClassification = recomputation.classificationByReportId[report.id];

  if (reportClassification === 'ignored_same_day') {
    await repository.addEvent({
      reportId: report.id,
      type: 'signal_reconfirmation_ignored_same_day',
      actor: 'system',
      metadata: {
        bike_group_id: bikeGroup.id
      }
    });
  }

  if (reportClassification === 'counted_same_reporter' || reportClassification === 'counted_distinct_reporter') {
    await repository.addEvent({
      reportId: report.id,
      type: 'signal_reconfirmation_counted',
      actor: 'system',
      metadata: {
        bike_group_id: bikeGroup.id,
        reporter_match_kind:
          reportClassification === 'counted_same_reporter' ? 'same_reporter' : 'distinct_reporter'
      }
    });
  }

  if (previousSignalStrength !== recomputation.signalStrength) {
    await repository.addEvent({
      reportId: report.id,
      type: 'signal_strength_changed',
      actor: 'system',
      metadata: {
        previous_signal_strength: previousSignalStrength,
        signal_strength: recomputation.signalStrength,
        bike_group_id: bikeGroup.id
      }
    });
  }

  const lookbackStart = dedupeLookbackStartIso(nowIso);
  const openReports = (await repository.listOpenReportsSince(lookbackStart)).filter(
    (candidate) => candidate.id !== report.id
  );

  const dedupeCandidates = openReports
    .map((candidate) => scoreDuplicateCandidate(report, candidate, nowIso))
    .filter((candidate): candidate is NonNullable<typeof candidate> => candidate !== null)
    .sort((left, right) => right.score - left.score)
    .slice(0, 5)
    .map((candidate) => candidate.reportId);

  const flaggedForReview = applyFingerprintHeuristic(validatedPayload.fingerprintHash, Date.now());
  if (flaggedForReview) {
    await repository.setFlaggedForReview(report.id, true);
  }

  const token = await createTrackingToken(report.publicId, getTrackingExpiry());

  return {
    public_id: report.publicId,
    created_at: report.createdAt,
    status: report.status,
    tracking_url: buildPublicUrl(`/report/status/${report.publicId}?token=${token}`),
    dedupe_candidates: dedupeCandidates,
    flagged_for_review: flaggedForReview,
    bike_group_id: bikeGroup.id,
    signal_strength: updatedGroup.signalStrength,
    signal_summary: bikeGroupToSignalSummary(updatedGroup)
  };
};

export const getReportPublicStatus = async (
  publicId: string,
  token: string
): Promise<Pick<Report, 'publicId' | 'status' | 'createdAt' | 'updatedAt'>> => {
  if (!token) {
    throw new ApiError(400, 'missing_token', 'Tracking token is required');
  }

  const claims = await verifyTrackingToken(token);
  if (claims.public_id !== publicId) {
    throw new ApiError(403, 'token_mismatch', 'Tracking token does not match report id');
  }

  const repository = getRepository();
  const report = await repository.getReportByPublicId(publicId);

  if (!report) {
    throw new ApiError(404, 'report_not_found', 'Report not found');
  }

  return {
    publicId: report.publicId,
    status: report.status,
    createdAt: report.createdAt,
    updatedAt: report.updatedAt
  };
};

export const listOperatorReports = async (filters: OperatorReportFilters): Promise<OperatorReportView[]> => {
  const repository = getRepository();
  const reports = await repository.listReports(filters);
  const enrichedReports = await enrichReportsWithSignal(reports);
  const filtered = applySignalFilters(enrichedReports, filters);

  if (filters.sort === 'signal') {
    return sortReportsBySignal(filtered);
  }

  return filtered;
};

export const updateReportStatus = async (
  reportId: number,
  nextStatus: ReportStatus,
  session: OperatorSession
): Promise<Report> => {
  const payload = reportStatusUpdateSchema.parse({ status: nextStatus });
  const repository = getRepository();
  const current = await repository.getReportById(reportId);

  if (!current) {
    throw new ApiError(404, 'report_not_found', 'Report not found');
  }

  ensureTransitionAllowed(current.status, payload.status);

  const updated = await repository.updateReportStatus(reportId, payload.status, session.email);
  if (!updated) {
    throw new ApiError(404, 'report_not_found', 'Report not found');
  }

  return updated;
};

export const mergeDuplicateReports = async (
  canonicalReportId: number,
  duplicateReportIds: number[],
  session: OperatorSession
) => {
  mergeReportsSchema.parse({
    canonical_report_id: canonicalReportId,
    duplicate_report_ids: duplicateReportIds
  });

  const repository = getRepository();
  const canonical = await repository.getReportById(canonicalReportId);
  if (!canonical) {
    throw new ApiError(404, 'canonical_not_found', 'Canonical report not found');
  }

  for (const duplicateId of duplicateReportIds) {
    const duplicate = await repository.getReportById(duplicateId);
    if (!duplicate) {
      throw new ApiError(404, 'duplicate_not_found', `Duplicate report not found: ${duplicateId}`);
    }
  }

  return repository.mergeReports(canonicalReportId, duplicateReportIds, session.email);
};

export const generateExportBatch = async (
  input: { period_type: 'weekly' | 'monthly'; period_start?: string | null; period_end?: string | null },
  session: OperatorSession
): Promise<ExportBatch> => {
  const payload = generateExportSchema.parse(input);
  const repository = getRepository();

  const period = getReportWindow(payload.period_type, payload.period_start, payload.period_end);
  const reports = (await repository.listReports()).filter((report) => {
    return report.createdAt >= period.periodStart && report.createdAt <= period.periodEnd;
  });

  const artifacts = await buildExportArtifacts(reports, period.periodStart, period.periodEnd);

  const exportBatch = await repository.createExportBatch({
    periodType: payload.period_type,
    periodStart: period.periodStart,
    periodEnd: period.periodEnd,
    generatedBy: session.email,
    rowCount: reports.length,
    artifacts
  });

  const mailer = getMailer();
  await mailer.send({
    recipients: getExportRecipients(),
    subject: `[ZwerfFiets] ${payload.period_type} export generated`,
    body: `Export ${exportBatch.id} generated for ${period.periodStart} - ${period.periodEnd}. Open ${buildPublicUrl('/bikeadmin/exports')} to download artifacts.`
  });

  return exportBatch;
};

export const getExportAsset = async (
  exportId: number,
  format: 'csv' | 'geojson' | 'pdf'
): Promise<{ contentType: string; body: string | Uint8Array; fileName: string }> => {
  const repository = getRepository();
  const exportBatch = await repository.getExportBatch(exportId);

  if (!exportBatch) {
    throw new ApiError(404, 'export_not_found', 'Export batch not found');
  }

  if (format === 'csv') {
    return {
      contentType: 'text/csv; charset=utf-8',
      body: exportBatch.artifacts.csv,
      fileName: `zwerffiets-${exportBatch.periodType}-${exportBatch.periodStart}.csv`
    };
  }

  if (format === 'geojson') {
    return {
      contentType: 'application/geo+json; charset=utf-8',
      body: exportBatch.artifacts.geojson,
      fileName: `zwerffiets-${exportBatch.periodType}-${exportBatch.periodStart}.geojson`
    };
  }

  return {
    contentType: 'application/pdf',
    body: exportBatch.artifacts.pdf,
    fileName: `zwerffiets-${exportBatch.periodType}-${exportBatch.periodStart}.pdf`
  };
};

export const listReportEvents = async (reportId: number) => {
  const repository = getRepository();
  return repository.listEvents(reportId);
};

export const getReportDetails = async (reportId: number): Promise<{
  report: Report;
  events: Awaited<ReturnType<typeof listReportEvents>>;
  signal_details: SignalDetails;
}> => {
  const repository = getRepository();
  const report = await repository.getReportById(reportId);
  if (!report) {
    throw new ApiError(404, 'report_not_found', 'Report not found');
  }

  const bikeGroup = await repository.getBikeGroupById(report.bikeGroupId);
  if (!bikeGroup) {
    throw new ApiError(404, 'bike_group_not_found', 'Bike group not found');
  }

  const groupReports = await repository.listReportsByBikeGroupId(report.bikeGroupId);

  return {
    report,
    events: await repository.listEvents(reportId),
    signal_details: buildSignalDetails(groupReports, bikeGroup)
  };
};

export const getAvailableTags = async () => {
  const repository = getRepository();
  return repository.getTags();
};
