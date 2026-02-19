export const MAX_PHOTO_COUNT = 3;
export const MIN_PHOTO_COUNT = 1;
export const MAX_NOTE_LENGTH = 500;
export const MAX_TAG_COUNT = 10;
export const MIN_TAG_COUNT = 1;

export const MAX_UPLOAD_BYTES = 10 * 1024 * 1024;
export const ALLOWED_IMAGE_TYPES = ['image/jpeg', 'image/webp'] as const;
export const PHOTO_UPLOAD_TARGET_MAX_BYTES = 500 * 1024;
export const PHOTO_UPLOAD_MAX_DIMENSION_PX = 1000;
export const PHOTO_UPLOAD_INITIAL_JPEG_QUALITY = 0.85;
export const PHOTO_UPLOAD_MIN_JPEG_QUALITY = 0.45;
export const PHOTO_UPLOAD_QUALITY_STEP = 0.08;
export const PHOTO_UPLOAD_DIMENSION_STEP = 0.85;
export const PHOTO_UPLOAD_MAX_ENCODING_ATTEMPTS = 12;

export const REPORT_STATUSES = ['new', 'triaged', 'forwarded', 'resolved', 'invalid'] as const;
export const OPEN_REPORT_STATUSES = ['new', 'triaged', 'forwarded'] as const;

export const OPERATOR_ROLES = ['operator'] as const;

export const STATUS_TRANSITIONS: Record<string, readonly string[]> = {
  new: ['triaged', 'invalid'],
  triaged: ['forwarded', 'resolved', 'invalid'],
  forwarded: ['resolved', 'invalid'],
  resolved: [],
  invalid: []
};

export const DEDUPE_RADIUS_METERS = 15;
export const DEDUPE_LOOKBACK_DAYS = 30;

export const EXPORT_TIMEZONE = 'Europe/Amsterdam';
export const PHOTO_RETENTION_DAYS = 365;

export const ANON_REPORTER_COOKIE_NAME = 'zwerffiets_anon_id';
export const ANON_REPORTER_COOKIE_MAX_AGE_SECONDS = 180 * 24 * 60 * 60;

export const SIGNAL_MATCH_RADIUS_METERS = 10;
export const SIGNAL_CANDIDATE_LOOKBACK_DAYS = 180;
export const SIGNAL_RECONFIRMATION_GAP_DAYS = 28;
export const STRONG_SIGNAL_MIN_UNIQUE_REPORTERS = 2;

export const REPORT_RATE_LIMIT_REQUESTS = 8;
export const REPORT_RATE_LIMIT_WINDOW_MS = 5 * 60 * 1000;

export const FINGERPRINT_BURST_THRESHOLD = 4;

export const TRACKING_LINK_TTL_DAYS = 90;

export const QUEUE_STORAGE_KEY = 'zwerffiets-offline-queue';
