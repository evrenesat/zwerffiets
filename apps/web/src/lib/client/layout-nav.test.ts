import { describe, expect, it } from 'vitest';
import { layoutNavVariant } from '$lib/client/layout-nav';

describe('layoutNavVariant', () => {
  it('returns landing for homepage', () => {
    expect(layoutNavVariant('/')).toBe('landing');
  });

  it('returns report for report routes', () => {
    expect(layoutNavVariant('/report')).toBe('report');
    expect(layoutNavVariant('/report/status/ABC123')).toBe('report');
  });

  it('returns default for other routes', () => {
    expect(layoutNavVariant('/about')).toBe('default');
    expect(layoutNavVariant('/my-reports')).toBe('default');
  });
});
