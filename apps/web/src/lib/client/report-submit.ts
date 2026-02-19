import { ApiRequestError } from '$lib/client/http';
import type { TranslationKey } from '$lib/i18n/translations';

export interface ReportSubmitFailure {
  queueOffline: boolean;
  messageKey: TranslationKey;
}

export const resolveReportSubmitFailure = (error: unknown): ReportSubmitFailure => {
  if (error instanceof ApiRequestError) {
    if (error.code === 'invalid_location') {
      return {
        queueOffline: false,
        messageKey: 'report_error_location_accuracy'
      };
    }
    if (error.code === 'rate_limited') {
      return {
        queueOffline: false,
        messageKey: 'report_error_rate_limited'
      };
    }

    return {
      queueOffline: false,
      messageKey: 'report_error_submit_failed'
    };
  }

  return {
    queueOffline: true,
    messageKey: 'report_error_offline'
  };
};
