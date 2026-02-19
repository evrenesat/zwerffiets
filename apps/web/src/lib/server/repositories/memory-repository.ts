import { randomUUID } from 'node:crypto';
import { OPEN_REPORT_STATUSES } from '$lib/constants';
import type {
  BikeGroup,
  CreateReportPayload,
  DedupeGroup,
  ExportBatch,
  OperatorReportFilters,
  Report,
  ReportEvent,
  ReportPhoto,
  ReportStatus,
  Tag
} from '$lib/types';
import { DEFAULT_TAGS } from '$server/repositories/default-tags';
import type { Repository } from '$server/repositories/types';

const sortByCreatedAtDescending = <T extends { createdAt: string }>(items: T[]): T[] => {
  return [...items].sort((left, right) => right.createdAt.localeCompare(left.createdAt));
};

export class MemoryRepository implements Repository {
  private readonly tags: Tag[];
  private reports: Report[] = [];
  private photos: ReportPhoto[] = [];
  private events: ReportEvent[] = [];
  private dedupeGroups: DedupeGroup[] = [];
  private exportBatches: ExportBatch[] = [];
  private bikeGroups: BikeGroup[] = [];

  constructor() {
    this.tags = DEFAULT_TAGS.map((tag) => ({ ...tag, id: randomUUID() }));
  }

  async getTags(): Promise<Tag[]> {
    return [...this.tags];
  }

  async createReport(payload: CreateReportPayload & { bikeGroupId: string }): Promise<Report> {
    const nowIso = new Date().toISOString();
    const report: Report = {
      id: randomUUID(),
      publicId: randomUUID().slice(0, 8).toUpperCase(),
      createdAt: nowIso,
      updatedAt: nowIso,
      status: 'new',
      location: payload.location,
      tags: payload.tags,
      note: payload.note,
      source: payload.source,
      dedupeGroupId: null,
      bikeGroupId: payload.bikeGroupId,
      fingerprintHash: payload.fingerprintHash,
      reporterHash: payload.reporterHash,
      flaggedForReview: false
    };

    this.reports.push(report);

    return report;
  }

  async saveReportPhotos(reportId: string, photos: CreateReportPayload['photos']): Promise<ReportPhoto[]> {
    const createdAt = new Date().toISOString();
    const reportPhotos = photos.map((photo) => ({
      id: randomUUID(),
      reportId,
      createdAt,
      mimeType: photo.mimeType,
      filename: photo.name,
      bytes: photo.bytes
    }));

    this.photos = [...this.photos, ...reportPhotos];
    return reportPhotos;
  }

  async getReportByPublicId(publicId: string): Promise<Report | null> {
    return this.reports.find((report) => report.publicId === publicId) ?? null;
  }

  async getReportById(id: string): Promise<Report | null> {
    return this.reports.find((report) => report.id === id) ?? null;
  }

  async listReports(filters?: OperatorReportFilters): Promise<Report[]> {
    const filtered = this.reports.filter((report) => {
      if (filters?.status && report.status !== filters.status) {
        return false;
      }

      if (filters?.tag && !report.tags.includes(filters.tag)) {
        return false;
      }

      if (filters?.from && report.createdAt < filters.from) {
        return false;
      }

      if (filters?.to && report.createdAt > filters.to) {
        return false;
      }

      return true;
    });

    return sortByCreatedAtDescending(filtered);
  }

  async listReportsSince(sinceIso: string): Promise<Report[]> {
    return this.reports.filter((report) => report.createdAt >= sinceIso);
  }

  async listReportsByBikeGroupId(bikeGroupId: string): Promise<Report[]> {
    return this.reports
      .filter((report) => report.bikeGroupId === bikeGroupId)
      .sort((left, right) => left.createdAt.localeCompare(right.createdAt));
  }

  async listOpenReportsSince(sinceIso: string): Promise<Report[]> {
    return this.reports.filter(
      (report) =>
        OPEN_REPORT_STATUSES.includes(report.status as (typeof OPEN_REPORT_STATUSES)[number]) &&
        report.createdAt >= sinceIso
    );
  }

  async updateReportStatus(
    reportId: string,
    nextStatus: ReportStatus,
    actor: string
  ): Promise<Report | null> {
    const index = this.reports.findIndex((report) => report.id === reportId);
    if (index === -1) {
      return null;
    }

    const updated: Report = {
      ...this.reports[index],
      status: nextStatus,
      updatedAt: new Date().toISOString()
    };
    this.reports[index] = updated;

    await this.addEvent({
      reportId,
      type: 'status_changed',
      actor,
      metadata: { status: nextStatus }
    });

    return updated;
  }

  async mergeReports(
    canonicalReportId: string,
    duplicateReportIds: string[],
    actor: string
  ): Promise<DedupeGroup> {
    const existingGroup = this.dedupeGroups.find((group) => group.canonicalReportId === canonicalReportId);
    const group: DedupeGroup =
      existingGroup ?? {
        id: randomUUID(),
        canonicalReportId,
        mergedReportIds: [],
        createdAt: new Date().toISOString(),
        createdBy: actor
      };

    const mergedSet = new Set([...group.mergedReportIds, ...duplicateReportIds]);
    group.mergedReportIds = [...mergedSet];

    if (!existingGroup) {
      this.dedupeGroups.push(group);
    }

    this.reports = this.reports.map((report) => {
      if (report.id === canonicalReportId || mergedSet.has(report.id)) {
        return { ...report, dedupeGroupId: group.id, updatedAt: new Date().toISOString() };
      }
      return report;
    });

    for (const mergedId of duplicateReportIds) {
      await this.addEvent({
        reportId: mergedId,
        type: 'merged',
        actor,
        metadata: { canonicalReportId, dedupeGroupId: group.id }
      });
    }

    return group;
  }

  async addEvent(event: Omit<ReportEvent, 'id' | 'createdAt'>): Promise<ReportEvent> {
    const entity: ReportEvent = {
      ...event,
      id: randomUUID(),
      createdAt: new Date().toISOString()
    };

    this.events.push(entity);
    return entity;
  }

  async listEvents(reportId: string): Promise<ReportEvent[]> {
    return this.events
      .filter((event) => event.reportId === reportId)
      .sort((left, right) => left.createdAt.localeCompare(right.createdAt));
  }

  async createExportBatch(batch: Omit<ExportBatch, 'id' | 'generatedAt'>): Promise<ExportBatch> {
    const entity: ExportBatch = {
      ...batch,
      id: randomUUID(),
      generatedAt: new Date().toISOString()
    };

    this.exportBatches.push(entity);
    return entity;
  }

  async getExportBatch(exportId: string): Promise<ExportBatch | null> {
    return this.exportBatches.find((batch) => batch.id === exportId) ?? null;
  }

  async listExportBatches(): Promise<ExportBatch[]> {
    return [...this.exportBatches].sort((left, right) =>
      right.generatedAt.localeCompare(left.generatedAt)
    );
  }

  async listPhotos(reportId: string): Promise<ReportPhoto[]> {
    return this.photos.filter((photo) => photo.reportId === reportId);
  }

  async setFlaggedForReview(reportId: string, flagged: boolean): Promise<void> {
    this.reports = this.reports.map((report) => {
      if (report.id !== reportId) {
        return report;
      }

      return {
        ...report,
        flaggedForReview: flagged,
        updatedAt: new Date().toISOString()
      };
    });
  }

  async createBikeGroup(anchor: Report['location']): Promise<BikeGroup> {
    const nowIso = new Date().toISOString();
    const group: BikeGroup = {
      id: randomUUID(),
      createdAt: nowIso,
      updatedAt: nowIso,
      anchorLat: anchor.lat,
      anchorLng: anchor.lng,
      lastReportAt: nowIso,
      totalReports: 0,
      uniqueReporters: 0,
      sameReporterReconfirmations: 0,
      distinctReporterReconfirmations: 0,
      firstQualifyingReconfirmationAt: null,
      lastQualifyingReconfirmationAt: null,
      signalStrength: 'none'
    };

    this.bikeGroups.push(group);
    return group;
  }

  async getBikeGroupById(groupId: string): Promise<BikeGroup | null> {
    return this.bikeGroups.find((group) => group.id === groupId) ?? null;
  }

  async updateBikeGroup(group: BikeGroup): Promise<BikeGroup> {
    const index = this.bikeGroups.findIndex((entry) => entry.id === group.id);

    if (index === -1) {
      this.bikeGroups.push(group);
      return group;
    }

    this.bikeGroups[index] = group;
    return group;
  }
}
