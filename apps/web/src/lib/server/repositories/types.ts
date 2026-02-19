import type {
  BikeGroup,
  CreateReportPayload,
  DedupeGroup,
  ExportBatch,
  OperatorReportFilters,
  Report,
  ReportEvent,
  ReportLocation,
  ReportPhoto,
  ReportStatus,
  Tag
} from '$lib/types';

export interface Repository {
  getTags(): Promise<Tag[]>;
  createReport(payload: CreateReportPayload & { bikeGroupId: string }): Promise<Report>;
  saveReportPhotos(reportId: string, photos: CreateReportPayload['photos']): Promise<ReportPhoto[]>;
  getReportByPublicId(publicId: string): Promise<Report | null>;
  getReportById(id: string): Promise<Report | null>;
  listReports(filters?: OperatorReportFilters): Promise<Report[]>;
  listReportsSince(sinceIso: string): Promise<Report[]>;
  listReportsByBikeGroupId(bikeGroupId: string): Promise<Report[]>;
  listOpenReportsSince(sinceIso: string): Promise<Report[]>;
  updateReportStatus(
    reportId: string,
    nextStatus: ReportStatus,
    actor: string
  ): Promise<Report | null>;
  mergeReports(
    canonicalReportId: string,
    duplicateReportIds: string[],
    actor: string
  ): Promise<DedupeGroup>;
  addEvent(event: Omit<ReportEvent, 'id' | 'createdAt'>): Promise<ReportEvent>;
  listEvents(reportId: string): Promise<ReportEvent[]>;
  createExportBatch(
    batch: Omit<ExportBatch, 'id' | 'generatedAt'>
  ): Promise<ExportBatch>;
  getExportBatch(exportId: string): Promise<ExportBatch | null>;
  listExportBatches(): Promise<ExportBatch[]>;
  listPhotos(reportId: string): Promise<ReportPhoto[]>;
  setFlaggedForReview(reportId: string, flagged: boolean): Promise<void>;
  createBikeGroup(anchor: ReportLocation): Promise<BikeGroup>;
  getBikeGroupById(groupId: string): Promise<BikeGroup | null>;
  updateBikeGroup(group: BikeGroup): Promise<BikeGroup>;
}
