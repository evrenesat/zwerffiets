import { createHash } from 'node:crypto';

export const buildFingerprint = (
  ip: string,
  userAgent: string | null,
  acceptLanguage: string | null
): string => {
  const normalized = `${ip}|${userAgent ?? 'na'}|${acceptLanguage ?? 'na'}`;
  return createHash('sha256').update(normalized).digest('hex');
};
