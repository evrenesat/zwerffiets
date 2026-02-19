import { MemoryRepository } from '$server/repositories/memory-repository';
import type { Repository } from '$server/repositories/types';
import { logger } from '$server/logger';

let repository: Repository | null = null;

const createRepository = (): Repository => {
  logger.info('Using in-memory repository');
  return new MemoryRepository();
};

export const getRepository = (): Repository => {
  if (!repository) {
    repository = createRepository();
  }

  return repository;
};

export const setRepository = (nextRepository: Repository): void => {
  repository = nextRepository;
};
