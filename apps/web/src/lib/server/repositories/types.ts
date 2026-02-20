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
  createReport(payload: CreateReportPayload & { bikeGroupId: number }): Promise<Report>;
  saveReportPhotos(reportId: number, photos: CreateReportPayload['photos']): Promise<ReportPhoto[]>;
  getReportByPublicId(publicId: string): Promise<Report | null>;
  getReportById(id: number): Promise<Report | null>;
  listReports(filters?: OperatorReportFilters): Promise<Report[]>;
  listReportsSince(sinceIso: string): Promise<Report[]>;
  listReportsByBikeGroupId(bikeGroupId: number): Promise<Report[]>;
  listOpenReportsSince(sinceIso: string): Promise<Report[]>;
  updateReportStatus(
    reportId: number,
    nextStatus: ReportStatus,
    actor: string
  ): Promise<Report | null>;
  mergeReports(
    canonicalReportId: number,
    duplicateReportIds: number[],
    actor: string
  ): Promise<DedupeGroup>;
  addEvent(event: Omit<ReportEvent, 'id' | 'createdAt'>): Promise<ReportEvent>;
  listEvents(reportId: number): Promise<ReportEvent[]>;
  createExportBatch(
    batch: Omit<ExportBatch, 'id' | 'generatedAt'>
  ): Promise<ExportBatch>;
  getExportBatch(exportId: number): Promise<ExportBatch | null>;
  listExportBatches(): Promise<ExportBatch[]>;
  listPhotos(reportId: number): Promise<ReportPhoto[]>;
  setFlaggedForReview(reportId: number, flagged: boolean): Promise<void>;
  createBikeGroup(anchor: ReportLocation): Promise<BikeGroup>;
  getBikeGroupById(groupId: number): Promise<BikeGroup | null>;
  updateBikeGroup(group: BikeGroup): Promise<BikeGroup>;
}
