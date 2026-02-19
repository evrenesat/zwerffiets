interface RateBucket {
  startsAtMs: number;
  count: number;
}

const buckets = new Map<string, RateBucket>();

export interface RateLimitResult {
  allowed: boolean;
  remaining: number;
  resetAtMs: number;
}

export const checkRateLimit = (
  key: string,
  maxRequests: number,
  windowMs: number,
  nowMs = Date.now()
): RateLimitResult => {
  const existing = buckets.get(key);

  if (!existing || nowMs - existing.startsAtMs >= windowMs) {
    buckets.set(key, { startsAtMs: nowMs, count: 1 });
    return {
      allowed: true,
      remaining: maxRequests - 1,
      resetAtMs: nowMs + windowMs
    };
  }

  existing.count += 1;
  buckets.set(key, existing);

  const remaining = Math.max(0, maxRequests - existing.count);

  return {
    allowed: existing.count <= maxRequests,
    remaining,
    resetAtMs: existing.startsAtMs + windowMs
  };
};

export const resetRateLimitState = (): void => {
  buckets.clear();
};
