import { describe, expect, it } from 'vitest';
import {
  myReportStatusPath,
  myReportStatusLabel,
  myReportsDateLocale,
  myReportsLoadFailedMessage,
  USER_LOGOUT_ENDPOINT,
  USER_REPORTS_ENDPOINT,
  USER_SESSION_ENDPOINT
} from '$lib/client/my-reports';

describe('my-reports helpers', () => {
  it('uses the expected user auth/report endpoints', () => {
    expect(USER_SESSION_ENDPOINT).toBe('/api/v1/auth/session');
    expect(USER_REPORTS_ENDPOINT).toBe('/api/v1/user/reports');
    expect(USER_LOGOUT_ENDPOINT).toBe('/api/v1/auth/logout');
  });

  it('maps UI language to date locales', () => {
    expect(myReportsDateLocale('nl')).toBe('nl-NL');
    expect(myReportsDateLocale('en')).toBe('en-US');
  });

  it('resolves translated report status labels', () => {
    expect(myReportStatusLabel('nl', 'resolved')).toBe('Afgehandeld');
    expect(myReportStatusLabel('en', 'resolved')).toBe('Resolved');
  });

  it('resolves translated load-failure messages', () => {
    expect(myReportsLoadFailedMessage('nl')).toBe('Meldingen konden niet worden geladen.');
    expect(myReportsLoadFailedMessage('en')).toBe('Could not load reports.');
  });

  it('builds report status page paths', () => {
    expect(myReportStatusPath('ABC12345')).toBe('/report/status/ABC12345');
  });
});
