import { describe, expect, it } from 'vitest';
import {
  buildNormalizationCandidates,
  fitDimensionsWithinMaxEdge,
  type PhotoNormalizationConfig
} from '$lib/client/photo-normalize';

describe('photo normalization', () => {
  it('scales down long edge to configured max dimension', () => {
    const size = fitDimensionsWithinMaxEdge(4000, 3000, 1000);

    expect(size.width).toBe(1000);
    expect(size.height).toBe(750);
  });

  it('keeps original size when already under max edge', () => {
    const size = fitDimensionsWithinMaxEdge(900, 700, 1000);

    expect(size.width).toBe(900);
    expect(size.height).toBe(700);
  });

  it('builds quality-first encoding candidates before reducing dimensions', () => {
    const config: PhotoNormalizationConfig = {
      maxBytes: 500 * 1024,
      maxDimensionPx: 1000,
      initialQuality: 0.85,
      minQuality: 0.69,
      qualityStep: 0.08,
      dimensionStep: 0.8,
      maxAttempts: 5
    };

    const candidates = buildNormalizationCandidates(2400, 1800, config);

    expect(candidates).toEqual([
      { width: 1000, height: 750, quality: 0.85 },
      { width: 1000, height: 750, quality: 0.77 },
      { width: 1000, height: 750, quality: 0.69 },
      { width: 800, height: 600, quality: 0.85 },
      { width: 800, height: 600, quality: 0.77 }
    ]);
  });
});
