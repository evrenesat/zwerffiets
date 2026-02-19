export const NO_ACTIVE_PHOTO_INDEX = -1;
export const GALLERY_ACTION_CLOSE = 'close';
export const GALLERY_ACTION_NEXT = 'next';
export const GALLERY_ACTION_PREVIOUS = 'previous';

export type GalleryAction =
  | typeof GALLERY_ACTION_CLOSE
  | typeof GALLERY_ACTION_NEXT
  | typeof GALLERY_ACTION_PREVIOUS;

const KEY_ESCAPE = 'Escape';
const KEY_ARROW_LEFT = 'ArrowLeft';
const KEY_ARROW_RIGHT = 'ArrowRight';

const hasPhotos = (totalPhotos: number): boolean => totalPhotos > 0;

export const getNextPhotoIndex = (currentIndex: number, totalPhotos: number): number => {
  if (!hasPhotos(totalPhotos)) {
    return NO_ACTIVE_PHOTO_INDEX;
  }
  if (currentIndex === NO_ACTIVE_PHOTO_INDEX) {
    return 0;
  }

  return (currentIndex + 1) % totalPhotos;
};

export const getPreviousPhotoIndex = (currentIndex: number, totalPhotos: number): number => {
  if (!hasPhotos(totalPhotos)) {
    return NO_ACTIVE_PHOTO_INDEX;
  }
  if (currentIndex === NO_ACTIVE_PHOTO_INDEX) {
    return totalPhotos - 1;
  }

  return (currentIndex - 1 + totalPhotos) % totalPhotos;
};

export const galleryActionFromKey = (key: string): GalleryAction | null => {
  if (key === KEY_ESCAPE) {
    return GALLERY_ACTION_CLOSE;
  }
  if (key === KEY_ARROW_LEFT) {
    return GALLERY_ACTION_PREVIOUS;
  }
  if (key === KEY_ARROW_RIGHT) {
    return GALLERY_ACTION_NEXT;
  }

  return null;
};
