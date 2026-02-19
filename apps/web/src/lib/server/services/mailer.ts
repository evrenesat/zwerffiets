import { env } from '$server/env';
import { logger } from '$server/logger';

export interface EmailPayload {
  subject: string;
  body: string;
  recipients: string[];
}

export interface Mailer {
  send(payload: EmailPayload): Promise<void>;
}

class LogMailer implements Mailer {
  async send(payload: EmailPayload): Promise<void> {
    logger.info({ email: payload }, 'Export email queued');
  }
}

let mailer: Mailer | null = null;

export const getMailer = (): Mailer => {
  if (!mailer) {
    mailer = new LogMailer();
  }

  return mailer;
};

export const getExportRecipients = (): string[] => {
  return env.EXPORT_EMAIL_TO.split(',').map((entry) => entry.trim()).filter(Boolean);
};
