import type { OPERATOR_ROLES, REPORT_STATUSES } from '$lib/constants';

export type ReportStatus = (typeof REPORT_STATUSES)[number];
export type OperatorRole = (typeof OPERATOR_ROLES)[number];

export type SignalStrength = 'none' | 'weak_same_reporter' | 'strong_distinct_reporters';
export type ReporterMatchKind = 'same_reporter' | 'distinct_reporter';

export interface ReportLocation {
  lat: number;
  lng: number;
  accuracy_m: number;
}

export interface PhotoUpload {
  name: string;
  mimeType: string;
  bytes: Uint8Array;
}

export interface Tag {
  id: number;
  code: string;
  label: string;
  isActive: boolean;
}

export interface ReportSignalSummary {
  totalReports: number;
  uniqueReporters: number;
  sameReporterReconfirmations: number;
  distinctReporterReconfirmations: number;
  firstQualifyingReconfirmationAt: string | null;
  lastQualifyingReconfirmationAt: string | null;
  lastReportAt: string;
  hasQualifyingReconfirmation: boolean;
}

export interface BikeGroup {
  id: number;
  createdAt: string;
  updatedAt: string;
  anchorLat: number;
  anchorLng: number;
  lastReportAt: string;
  totalReports: number;
  uniqueReporters: number;
  sameReporterReconfirmations: number;
  distinctReporterReconfirmations: number;
  firstQualifyingReconfirmationAt: string | null;
  lastQualifyingReconfirmationAt: string | null;
  signalStrength: SignalStrength;
}

export interface Report {
  id: number;
  publicId: string;
  createdAt: string;
  updatedAt: string;
  status: ReportStatus;
  location: ReportLocation;
  tags: string[];
  note: string | null;
  source: 'web' | 'partner_import';
  dedupeGroupId: number | null;
  bikeGroupId: number;
  fingerprintHash: string;
  reporterHash: string;
  flaggedForReview: boolean;
}

export interface OperatorReportView extends Report {
  bike_group_id: number;
  signal_summary: ReportSignalSummary;
  signal_strength: SignalStrength;
  preview_photo_url: string | null;
}

export interface OperatorReportPhotoView {
  id: number;
  url: string;
  mime_type: string;
  filename: string;
  size_bytes: number;
  created_at: string;
}

export interface SignalTimelineEntry {
  reportId: number;
  publicId: string;
  createdAt: string;
  reporterLabel: string;
  reporterMatchKind: ReporterMatchKind | null;
  qualified: boolean;
  ignoredSameDay: boolean;
}

export interface SignalDetails {
  bikeGroup: BikeGroup;
  signalSummary: ReportSignalSummary;
  signalStrength: SignalStrength;
  timeline: SignalTimelineEntry[];
}

export interface ReportPhoto {
  id: number;
  reportId: number;
  createdAt: string;
  mimeType: string;
  filename: string;
  bytes: Uint8Array;
}

export interface DedupeGroup {
  id: number;
  canonicalReportId: number;
  mergedReportIds: number[];
  createdAt: string;
  createdBy: string;
}

export interface ReportEvent {
  id: number;
  reportId: number;
  createdAt: string;
  type:
    | 'created'
    | 'status_changed'
    | 'merged'
    | 'exported'
    | 'signal_reconfirmation_counted'
    | 'signal_reconfirmation_ignored_same_day'
    | 'signal_strength_changed';
  actor: string;
  metadata: Record<string, unknown>;
}

export interface ExportArtifacts {
  csv: string;
  geojson: string;
  pdf: Uint8Array;
}

export interface ExportBatch {
  id: number;
  periodType: 'weekly' | 'monthly';
  periodStart: string;
  periodEnd: string;
  generatedAt: string;
  generatedBy: string;
  rowCount: number;
  artifacts: ExportArtifacts;
}

export interface CreateReportPayload {
  photos: PhotoUpload[];
  location: ReportLocation;
  tags: string[];
  note: string | null;
  clientTs: string | null;
  source: 'web';
  ip: string;
  fingerprintHash: string;
  reporterHash: string;
}

export interface ReportCreateResponse {
  public_id: string;
  created_at: string;
  status: ReportStatus;
  tracking_url: string;
  dedupe_candidates: number[];
  flagged_for_review: boolean;
  bike_group_id: number;
  signal_strength: SignalStrength;
  signal_summary: ReportSignalSummary;
}

export interface OperatorSession {
  email: string;
  role: OperatorRole;
}

export interface OperatorReportFilters {
  status?: ReportStatus;
  tag?: string;
  from?: string;
  to?: string;
  signal_strength?: SignalStrength;
  has_qualifying_reconfirmation?: boolean;
  strong_only?: boolean;
  sort?: 'newest' | 'signal';
}
export interface BlogPost {
  id: number;
  slug: string;
  title: string;
  content_html: string;
  author_id: number | null;
  author_name: string;
  is_published: boolean;
  published_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface PaginatedBlogPosts {
  posts: BlogPost[];
  total: number;
}
