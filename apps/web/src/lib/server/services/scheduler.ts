import { DateTime } from 'luxon';
import { schedulerEnabled } from '$server/env';
import { logger } from '$server/logger';
import { generateExportBatch } from '$server/services/report-service';

const WEEKLY_HOUR = 6;
const WEEKLY_MINUTE = 0;
const MONTHLY_HOUR = 6;
const MONTHLY_MINUTE = 15;

let schedulerStarted = false;

const shouldRunWeekly = (now: DateTime): boolean => {
  return now.weekday === 1 && now.hour === WEEKLY_HOUR && now.minute === WEEKLY_MINUTE;
};

const shouldRunMonthly = (now: DateTime): boolean => {
  return now.day === 1 && now.hour === MONTHLY_HOUR && now.minute === MONTHLY_MINUTE;
};

export const startExportScheduler = (): void => {
  if (schedulerStarted || !schedulerEnabled) {
    return;
  }

  schedulerStarted = true;
  logger.info('Starting export scheduler');

  setInterval(async () => {
    const now = DateTime.now().setZone('Europe/Amsterdam');

    try {
      if (shouldRunWeekly(now)) {
        await generateExportBatch({ period_type: 'weekly' }, { email: 'scheduler', role: 'operator' });
      }

      if (shouldRunMonthly(now)) {
        await generateExportBatch({ period_type: 'monthly' }, { email: 'scheduler', role: 'operator' });
      }
    } catch (error) {
      logger.error({ error }, 'Scheduled export generation failed');
    }
  }, 60_000);
};
