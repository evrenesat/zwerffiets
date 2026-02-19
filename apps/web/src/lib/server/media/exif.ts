const MARKER_PREFIX = 0xff;
const START_OF_IMAGE_MARKER = 0xd8;
const START_OF_SCAN_MARKER = 0xda;
const APP1_MARKER = 0xe1;

const isJpeg = (bytes: Uint8Array): boolean => {
  return bytes.length >= 2 && bytes[0] === MARKER_PREFIX && bytes[1] === START_OF_IMAGE_MARKER;
};

const readSegmentLength = (bytes: Uint8Array, offset: number): number => {
  return (bytes[offset] << 8) | bytes[offset + 1];
};

export const stripExifMetadata = (bytes: Uint8Array, mimeType: string): Uint8Array => {
  if (mimeType !== 'image/jpeg' || !isJpeg(bytes)) {
    return bytes;
  }

  const chunks: Uint8Array[] = [];
  chunks.push(bytes.subarray(0, 2));

  let cursor = 2;

  while (cursor + 4 <= bytes.length) {
    if (bytes[cursor] !== MARKER_PREFIX) {
      break;
    }

    const marker = bytes[cursor + 1];
    if (marker === START_OF_SCAN_MARKER) {
      chunks.push(bytes.subarray(cursor));
      break;
    }

    const length = readSegmentLength(bytes, cursor + 2);
    if (length < 2 || cursor + 2 + length > bytes.length) {
      break;
    }

    const segmentEnd = cursor + 2 + length;
    if (marker !== APP1_MARKER) {
      chunks.push(bytes.subarray(cursor, segmentEnd));
    }

    cursor = segmentEnd;
  }

  const totalLength = chunks.reduce((sum, chunk) => sum + chunk.length, 0);
  const output = new Uint8Array(totalLength);

  let writeOffset = 0;
  for (const chunk of chunks) {
    output.set(chunk, writeOffset);
    writeOffset += chunk.length;
  }

  return output;
};
