import { describe, expect, it } from 'vitest';
import { hasSharedTags, scoreDuplicateCandidate, scoreSignalGroupCandidate } from '$server/dedupe';

describe('dedupe scoring', () => {
  it('returns a candidate score for nearby reports', () => {
    const candidate = scoreDuplicateCandidate(
      {
        location: { lat: 52.3676, lng: 4.9041, accuracy_m: 8 },
        tags: ['flat_tires', 'rusted']
      },
      {
        id: 'report-2',
        location: { lat: 52.36762, lng: 4.90412, accuracy_m: 6 },
        tags: ['flat_tires'],
        createdAt: new Date().toISOString()
      },
      new Date().toISOString()
    );

    expect(candidate).not.toBeNull();
    expect(candidate?.reportId).toBe('report-2');
    expect(candidate?.score).toBeGreaterThan(0.5);
  });

  it('returns null when reports are outside dedupe radius', () => {
    const candidate = scoreDuplicateCandidate(
      {
        location: { lat: 52.3676, lng: 4.9041, accuracy_m: 8 },
        tags: ['flat_tires']
      },
      {
        id: 'report-3',
        location: { lat: 52.3776, lng: 4.9141, accuracy_m: 6 },
        tags: ['flat_tires'],
        createdAt: new Date().toISOString()
      },
      new Date().toISOString()
    );

    expect(candidate).toBeNull();
  });

  it('scores signal candidates when within 10m and sharing at least one tag', () => {
    const score = scoreSignalGroupCandidate(
      {
        location: { lat: 52.3676, lng: 4.9041, accuracy_m: 8 },
        tags: ['flat_tires', 'rusted']
      },
      {
        location: { lat: 52.36761, lng: 4.90411, accuracy_m: 7 },
        tags: ['flat_tires'],
        createdAt: new Date().toISOString()
      },
      new Date().toISOString()
    );

    expect(score).not.toBeNull();
    expect(score).toBeGreaterThan(0.5);
  });

  it('returns null for signal candidates without shared tags', () => {
    const score = scoreSignalGroupCandidate(
      {
        location: { lat: 52.3676, lng: 4.9041, accuracy_m: 8 },
        tags: ['flat_tires']
      },
      {
        location: { lat: 52.36761, lng: 4.90411, accuracy_m: 7 },
        tags: ['no_chain'],
        createdAt: new Date().toISOString()
      },
      new Date().toISOString()
    );

    expect(hasSharedTags(['flat_tires'], ['no_chain'])).toBe(false);
    expect(score).toBeNull();
  });
});
