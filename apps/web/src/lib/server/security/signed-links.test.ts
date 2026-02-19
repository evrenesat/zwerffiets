import { describe, expect, it } from 'vitest';
import { createTrackingToken, verifyTrackingToken } from '$server/security/signed-links';

describe('tracking token signing', () => {
  it('round-trips public report ID in signed token', async () => {
    const token = await createTrackingToken('ABC12345', '1h');
    const claims = await verifyTrackingToken(token);

    expect(claims.public_id).toBe('ABC12345');
  });
});
