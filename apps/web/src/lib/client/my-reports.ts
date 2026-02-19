import { statusLabel, t } from '$lib/i18n';
import type { UiLanguage } from '$lib/i18n/translations';
export { USER_LOGOUT_ENDPOINT, USER_SESSION_ENDPOINT } from '$lib/client/user-auth';
export const USER_REPORTS_ENDPOINT = '/api/v1/user/reports';

const DATE_LOCALE_BY_LANGUAGE: Record<UiLanguage, string> = {
  nl: 'nl-NL',
  en: 'en-US'
};

const REPORT_DATE_FORMAT: Intl.DateTimeFormatOptions = {
  year: 'numeric',
  month: 'long',
  day: 'numeric',
  hour: '2-digit',
  minute: '2-digit'
};

export const myReportsDateLocale = (language: UiLanguage): string => {
  return DATE_LOCALE_BY_LANGUAGE[language];
};

export const formatMyReportDate = (isoString: string, language: UiLanguage): string => {
  return new Date(isoString).toLocaleDateString(myReportsDateLocale(language), REPORT_DATE_FORMAT);
};

export const myReportStatusLabel = (language: UiLanguage, status: string): string => {
  return statusLabel(language, status);
};

export const myReportsLoadFailedMessage = (language: UiLanguage): string => {
  return t(language, 'my_reports_load_failed');
};

export const myReportStatusPath = (publicId: string): string => {
  return `/report/status/${publicId}`;
};
