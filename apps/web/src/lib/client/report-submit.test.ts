import { describe, expect, it } from 'vitest';
import { ApiRequestError } from '$lib/client/http';
import { resolveReportSubmitFailure } from '$lib/client/report-submit';

describe('report submit failure resolver', () => {
  it('does not queue offline for invalid location API errors', () => {
    const failure = resolveReportSubmitFailure(
      new ApiRequestError(400, 'invalid_location', 'Location accuracy is invalid')
    );

    expect(failure.queueOffline).toBe(false);
    expect(failure.messageKey).toBe('report_error_location_accuracy');
  });

  it('does not queue offline for other API errors', () => {
    const failure = resolveReportSubmitFailure(
      new ApiRequestError(400, 'invalid_tags', 'Tags count is invalid')
    );

    expect(failure.queueOffline).toBe(false);
    expect(failure.messageKey).toBe('report_error_submit_failed');
  });

  it('surfaces rate limiting as a dedicated message', () => {
    const failure = resolveReportSubmitFailure(
      new ApiRequestError(429, 'rate_limited', 'Too many reports from this IP')
    );

    expect(failure.queueOffline).toBe(false);
    expect(failure.messageKey).toBe('report_error_rate_limited');
  });

  it('queues offline for network failures', () => {
    const failure = resolveReportSubmitFailure(new TypeError('Failed to fetch'));

    expect(failure.queueOffline).toBe(true);
    expect(failure.messageKey).toBe('report_error_offline');
  });
});
