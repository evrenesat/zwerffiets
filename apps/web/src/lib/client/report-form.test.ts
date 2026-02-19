import { describe, expect, it } from 'vitest';
import { canSubmitReport } from '$lib/client/report-form';

describe('report form gating', () => {
  it('blocks submit when location is missing', () => {
    const allowed = canSubmitReport({
      hasLocation: false,
      photoCount: 1,
      selectedTagCount: 1,
      submitting: false
    });

    expect(allowed).toBe(false);
  });

  it('allows submit when location, photos, and tags are available', () => {
    const allowed = canSubmitReport({
      hasLocation: true,
      photoCount: 2,
      selectedTagCount: 3,
      submitting: false
    });

    expect(allowed).toBe(true);
  });
});
