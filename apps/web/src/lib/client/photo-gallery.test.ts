import { describe, expect, it } from 'vitest';
import {
  GALLERY_ACTION_CLOSE,
  GALLERY_ACTION_NEXT,
  GALLERY_ACTION_PREVIOUS,
  NO_ACTIVE_PHOTO_INDEX,
  galleryActionFromKey,
  getNextPhotoIndex,
  getPreviousPhotoIndex
} from './photo-gallery';

describe('photo-gallery helpers', () => {
  it('returns no active index when there are no photos', () => {
    expect(getNextPhotoIndex(0, 0)).toBe(NO_ACTIVE_PHOTO_INDEX);
    expect(getPreviousPhotoIndex(0, 0)).toBe(NO_ACTIVE_PHOTO_INDEX);
  });

  it('wraps around when moving next', () => {
    expect(getNextPhotoIndex(NO_ACTIVE_PHOTO_INDEX, 3)).toBe(0);
    expect(getNextPhotoIndex(0, 3)).toBe(1);
    expect(getNextPhotoIndex(2, 3)).toBe(0);
  });

  it('wraps around when moving previous', () => {
    expect(getPreviousPhotoIndex(NO_ACTIVE_PHOTO_INDEX, 3)).toBe(2);
    expect(getPreviousPhotoIndex(2, 3)).toBe(1);
    expect(getPreviousPhotoIndex(0, 3)).toBe(2);
  });

  it('maps keyboard keys to gallery actions', () => {
    expect(galleryActionFromKey('Escape')).toBe(GALLERY_ACTION_CLOSE);
    expect(galleryActionFromKey('ArrowLeft')).toBe(GALLERY_ACTION_PREVIOUS);
    expect(galleryActionFromKey('ArrowRight')).toBe(GALLERY_ACTION_NEXT);
    expect(galleryActionFromKey('Enter')).toBeNull();
  });
});
