interface SubmitStateInput {
  hasLocation: boolean;
  photoCount: number;
  selectedTagCount: number;
  submitting: boolean;
}

export const canSubmitReport = (input: SubmitStateInput): boolean => {
  if (input.submitting) {
    return false;
  }

  if (!input.hasLocation) {
    return false;
  }

  return input.photoCount > 0 && input.selectedTagCount > 0;
};
