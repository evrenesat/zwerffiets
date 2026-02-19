import { z } from 'zod';
import {
  ALLOWED_IMAGE_TYPES,
  MAX_NOTE_LENGTH,
  MAX_PHOTO_COUNT,
  MAX_TAG_COUNT,
  MAX_UPLOAD_BYTES,
  MIN_PHOTO_COUNT,
  MIN_TAG_COUNT,
  REPORT_STATUSES
} from '$lib/constants';
import type { SignalStrength } from '$lib/types';

const DEFAULT_MAX_LOCATION_ACCURACY_M = 3000;

const SIGNAL_STRENGTHS: SignalStrength[] = [
  'none',
  'weak_same_reporter',
  'strong_distinct_reporters'
];

const locationSchema = z.object({
  lat: z.number().min(-90).max(90),
  lng: z.number().min(-180).max(180),
  accuracy_m: z.number().min(0).max(DEFAULT_MAX_LOCATION_ACCURACY_M)
});

const basePhotoSchema = z.object({
  name: z.string().min(1).max(120),
  mimeType: z.enum(ALLOWED_IMAGE_TYPES),
  bytes: z.instanceof(Uint8Array).refine((value) => value.length <= MAX_UPLOAD_BYTES)
});

export const createReportSchema = z.object({
  photos: z.array(basePhotoSchema).min(MIN_PHOTO_COUNT).max(MAX_PHOTO_COUNT),
  location: locationSchema,
  tags: z.array(z.string().min(1).max(64)).min(MIN_TAG_COUNT).max(MAX_TAG_COUNT),
  note: z.string().max(MAX_NOTE_LENGTH).nullable(),
  clientTs: z.string().datetime().nullable(),
  source: z.literal('web'),
  ip: z.string().min(3).max(120),
  fingerprintHash: z.string().length(64),
  reporterHash: z.string().length(64)
});

export const reportStatusUpdateSchema = z.object({
  status: z.enum(REPORT_STATUSES)
});

export const mergeReportsSchema = z.object({
  canonical_report_id: z.string().min(1),
  duplicate_report_ids: z.array(z.string().min(1)).min(1)
});

export const generateExportSchema = z.object({
  period_type: z.enum(['weekly', 'monthly']),
  period_start: z.string().datetime().nullable().optional(),
  period_end: z.string().datetime().nullable().optional()
});

export const listReportsQuerySchema = z.object({
  status: z.enum(REPORT_STATUSES).optional(),
  tag: z.string().optional(),
  from: z.string().datetime().optional(),
  to: z.string().datetime().optional(),
  signal_strength: z.enum(SIGNAL_STRENGTHS as [SignalStrength, ...SignalStrength[]]).optional(),
  has_qualifying_reconfirmation: z.enum(['true', 'false']).optional(),
  strong_only: z.enum(['true', 'false']).optional(),
  sort: z.enum(['newest', 'signal']).optional()
});
