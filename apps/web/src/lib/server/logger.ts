import pino from 'pino';

export const logger = pino({
  name: 'zwerffiets-api',
  level: process.env.LOG_LEVEL ?? 'info'
});
