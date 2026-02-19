import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

describe('repository selection', () => {
  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.unmock('$server/logger');
    vi.unmock('$server/repositories/memory-repository');
  });

  it('uses in-memory repository', async () => {
    class MemoryRepositoryMock {
      kind = 'memory';
    }

    const memoryConstructor = vi.fn(MemoryRepositoryMock);
    const info = vi.fn();
    vi.doMock('$server/logger', () => ({
      logger: {
        info
      }
    }));
    vi.doMock('$server/repositories/memory-repository', () => ({
      MemoryRepository: memoryConstructor
    }));

    const { getRepository } = await import('$server/repositories');
    const repository = getRepository() as unknown as { kind: string };

    expect(repository.kind).toBe('memory');
    expect(memoryConstructor).toHaveBeenCalledTimes(1);
    expect(info).toHaveBeenCalledTimes(1);
  });
});
