import { DateTime } from 'luxon';
import {
  DEDUPE_LOOKBACK_DAYS,
  DEDUPE_RADIUS_METERS,
  SIGNAL_CANDIDATE_LOOKBACK_DAYS,
  SIGNAL_MATCH_RADIUS_METERS
} from '$lib/constants';
import type { Report } from '$lib/types';
import { haversineMeters } from '$server/utils/geo';

const DISTANCE_WEIGHT = 0.6;
const TAG_OVERLAP_WEIGHT = 0.25;
const RECENCY_WEIGHT = 0.15;

const clamp01 = (value: number): number => Math.min(1, Math.max(0, value));

const tagOverlapRatio = (source: string[], target: string[]): number => {
  const sourceSet = new Set(source);
  const targetSet = new Set(target);

  const intersectionSize = [...sourceSet].filter((tag) => targetSet.has(tag)).length;
  const unionSize = new Set([...sourceSet, ...targetSet]).size;

  if (unionSize === 0) {
    return 0;
  }

  return intersectionSize / unionSize;
};

export const hasSharedTags = (source: string[], target: string[]): boolean => {
  const targetSet = new Set(target);
  return source.some((tag) => targetSet.has(tag));
};

const recencyScore = (createdAtIso: string, nowIso: string): number => {
  const ageDays = DateTime.fromISO(nowIso).diff(DateTime.fromISO(createdAtIso), 'days').days;
  return clamp01(1 - ageDays / DEDUPE_LOOKBACK_DAYS);
};

export interface DedupeCandidate {
  reportId: string;
  score: number;
  distanceMeters: number;
}

export const scoreDuplicateCandidate = (
  incoming: Pick<Report, 'location' | 'tags'>,
  candidate: Pick<Report, 'id' | 'location' | 'tags' | 'createdAt'>,
  nowIso: string
): DedupeCandidate | null => {
  const distanceMeters = haversineMeters(
    incoming.location.lat,
    incoming.location.lng,
    candidate.location.lat,
    candidate.location.lng
  );

  if (distanceMeters > DEDUPE_RADIUS_METERS) {
    return null;
  }

  const distanceScore = clamp01(1 - distanceMeters / DEDUPE_RADIUS_METERS);
  const overlap = tagOverlapRatio(incoming.tags, candidate.tags);
  const recency = recencyScore(candidate.createdAt, nowIso);

  const score =
    distanceScore * DISTANCE_WEIGHT + overlap * TAG_OVERLAP_WEIGHT + recency * RECENCY_WEIGHT;

  return {
    reportId: candidate.id,
    score: Number(score.toFixed(4)),
    distanceMeters: Number(distanceMeters.toFixed(2))
  };
};

export const dedupeLookbackStartIso = (nowIso: string): string => {
  return DateTime.fromISO(nowIso).minus({ days: DEDUPE_LOOKBACK_DAYS }).toUTC().toISO() ?? nowIso;
};

export const signalLookbackStartIso = (nowIso: string): string => {
  return (
    DateTime.fromISO(nowIso).minus({ days: SIGNAL_CANDIDATE_LOOKBACK_DAYS }).toUTC().toISO() ?? nowIso
  );
};

export const scoreSignalGroupCandidate = (
  incoming: Pick<Report, 'location' | 'tags'>,
  candidate: Pick<Report, 'location' | 'tags' | 'createdAt'>,
  nowIso: string
): number | null => {
  if (!hasSharedTags(incoming.tags, candidate.tags)) {
    return null;
  }

  const distanceMeters = haversineMeters(
    incoming.location.lat,
    incoming.location.lng,
    candidate.location.lat,
    candidate.location.lng
  );

  if (distanceMeters > SIGNAL_MATCH_RADIUS_METERS) {
    return null;
  }

  const distanceScore = clamp01(1 - distanceMeters / SIGNAL_MATCH_RADIUS_METERS);
  const overlap = tagOverlapRatio(incoming.tags, candidate.tags);
  const recency = recencyScore(candidate.createdAt, nowIso);

  const score = distanceScore * DISTANCE_WEIGHT + overlap * TAG_OVERLAP_WEIGHT + recency * RECENCY_WEIGHT;
  return Number(score.toFixed(4));
};
