<script lang="ts">
  import { onMount } from 'svelte';
  import { fetchJson } from '$lib/client/http';
  import { enqueueReport, flushQueue } from '$lib/client/offline-queue';
  import { normalizePhotoForUpload } from '$lib/client/photo-normalize';
  import { canSubmitReport } from '$lib/client/report-form';
  import { resolveReportSubmitFailure } from '$lib/client/report-submit';
  import { t, tagLabel, uiLanguage } from '$lib/i18n';
  import '$lib/styles/report-page.css';

  interface TagOption {
    id: string;
    code: string;
    label: string;
  }

  interface LocationPayload {
    lat: number;
    lng: number;
    accuracy_m: number;
  }

  type LocationState = 'requesting' | 'ready' | 'blocked';

  const MAX_PHOTO_COUNT = 3;

  const TAG_SVG_PATHS: Record<string, string> = {
    abandoned_long_time: '<circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>',
    blocking_sidewalk: '<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><line x1="12" x2="12" y1="9" y2="13"/><line x1="12" x2="12.01" y1="17" y2="17"/>',
    damaged_frame: '<path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>',
    flat_tires: '<path d="M8.5 14.5A2.5 2.5 0 0 0 11 12c0-1.38-.5-2-1-3-1.072-2.143-.224-4.054 2-6 .5 2.5 2 4.9 4 6.5 2 1.6 3 3.5 3 5.5a7 7 0 1 1-14 0c0-1.153.433-2.294 1-3a2.5 2.5 0 0 0 2.5 2.5z"/>',
    missing_parts: '<line x1="5" x2="19" y1="12" y2="12"/>',
    no_chain: '<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>',
    no_seat: '<circle cx="12" cy="12" r="10"/><line x1="4.93" x2="19.07" y1="4.93" y2="19.07"/>',
    other_visibility_issue: '<path d="M9.88 9.88a3 3 0 1 0 4.24 4.24"/><path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68"/><path d="M6.61 6.61A13.526 13.526 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61"/><line x1="2" x2="22" y1="2" y2="22"/>',
    rusted: '<path d="M7 16.3c2.2 0 4-1.83 4-4.05 0-1.16-.57-2.26-1.71-3.19S7.29 6.75 7 5.3c-.29 1.45-1.14 2.84-2.29 3.76S3 11.1 3 12.25c0 2.22 1.8 4.05 4 4.05z"/><path d="M12.56 6.6A10.97 10.97 0 0 0 14 3.02c.5 2.5 2 4.9 4 6.5s3 3.5 3 5.5a6.98 6.98 0 0 1-11.91 4.97"/>',
    wheel_missing: '<circle cx="12" cy="12" r="10"/><path d="m4.9 4.9 14.2 14.2"/>'
  };

  function tagSvgPaths(code: string): string {
    return TAG_SVG_PATHS[code] ?? '<circle cx="12" cy="12" r="10"/>';
  }

  let cameraInput: HTMLInputElement;
  let galleryInput: HTMLInputElement;
  let photos: File[] = [];
  let tagOptions: TagOption[] = [];
  let selectedTags: string[] = [];
  let note = '';
  let reporterEmail = '';
  let isLoggedIn = false;
  let location: LocationPayload | null = null;
  let locationState: LocationState = 'requesting';
  let locationMessage = t($uiLanguage, 'report_location_requesting');
  let submitting = false;
  let processingPhotos = false;
  let result: { public_id: string; tracking_url: string } | null = null;
  let errorMessage = '';

  const toDataUrl = async (file: File): Promise<string> => {
    return await new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => {
        if (typeof reader.result === 'string') {
          resolve(reader.result);
        } else {
          reject(new Error(t($uiLanguage, 'report_error_encode_image')));
        }
      };
      reader.onerror = () => reject(reader.error);
      reader.readAsDataURL(file);
    });
  };

  const loadTags = async (): Promise<void> => {
    const response = await fetch('/api/v1/tags');
    if (!response.ok) {
      return;
    }

    tagOptions = await response.json();
  };

  const requestLocation = (): void => {
    if (!('geolocation' in navigator)) {
      locationState = 'blocked';
      locationMessage = t($uiLanguage, 'report_location_not_supported');
      return;
    }

    locationState = 'requesting';
    locationMessage = t($uiLanguage, 'report_location_requesting');

    navigator.geolocation.getCurrentPosition(
      (position) => {
        location = {
          lat: Number(position.coords.latitude.toFixed(6)),
          lng: Number(position.coords.longitude.toFixed(6)),
          accuracy_m: Number(position.coords.accuracy.toFixed(1))
        };
        locationState = 'ready';
        locationMessage = t($uiLanguage, 'report_location_confirmed', {
          accuracy: location.accuracy_m
        });
      },
      () => {
        location = null;
        locationState = 'blocked';
        locationMessage = t($uiLanguage, 'report_location_required');
      },
      {
        enableHighAccuracy: true,
        timeout: 12000,
        maximumAge: 2000
      }
    );
  };

  const addPhotos = async (incoming: File[]): Promise<void> => {
    if (processingPhotos) {
      return;
    }

    const remaining = MAX_PHOTO_COUNT - photos.length;

    if (remaining <= 0) {
      errorMessage = t($uiLanguage, 'report_photo_limit');
      return;
    }

    const acceptedPhotos = incoming.slice(0, remaining);
    if (acceptedPhotos.length === 0) {
      return;
    }

    processingPhotos = true;
    errorMessage = '';
    try {
      const normalizedPhotos = await Promise.all(
        acceptedPhotos.map(async (photo) => await normalizePhotoForUpload(photo))
      );
      photos = [...photos, ...normalizedPhotos];
      if (incoming.length > acceptedPhotos.length) {
        errorMessage = t($uiLanguage, 'report_photo_limit');
      }
    } catch {
      errorMessage = t($uiLanguage, 'report_error_encode_image');
    } finally {
      processingPhotos = false;
    }
  };

  const onCameraChange = (event: Event): void => {
    const target = event.currentTarget as HTMLInputElement;
    const incoming = target.files ? Array.from(target.files) : [];
    void addPhotos(incoming);
    target.value = '';
  };

  const onGalleryChange = (event: Event): void => {
    const target = event.currentTarget as HTMLInputElement;
    const incoming = target.files ? Array.from(target.files) : [];
    void addPhotos(incoming);
    target.value = '';
  };

  const removePhoto = (index: number): void => {
    photos = photos.filter((_, currentIndex) => currentIndex !== index);
  };

  const toggleTag = (tagCode: string): void => {
    if (selectedTags.includes(tagCode)) {
      selectedTags = selectedTags.filter((entry) => entry !== tagCode);
      return;
    }

    selectedTags = [...selectedTags, tagCode];
  };

  const submitAllowed = (): boolean => {
    if (processingPhotos) {
      return false;
    }

    return canSubmitReport({
      hasLocation: locationState === 'ready',
      photoCount: photos.length,
      selectedTagCount: selectedTags.length,
      submitting
    });
  };

  const submit = async (): Promise<void> => {
    if (processingPhotos) {
      errorMessage = t($uiLanguage, 'report_error_encode_image');
      return;
    }

    if (!location || locationState !== 'ready') {
      errorMessage = t($uiLanguage, 'report_error_location_required');
      return;
    }

    submitting = true;
    errorMessage = '';
    result = null;

    const emailToSubmit = !isLoggedIn && reporterEmail.trim() ? reporterEmail.trim() : undefined;

    try {
      const formData = new FormData();
      formData.set('lat', String(location.lat));
      formData.set('lng', String(location.lng));
      formData.set('accuracy_m', String(location.accuracy_m));
      formData.set('tags', JSON.stringify(selectedTags));
      formData.set('note', note);
      formData.set('client_ts', new Date().toISOString());
      formData.set('ui_language', $uiLanguage);
      if (emailToSubmit) {
        formData.set('reporter_email', emailToSubmit);
      }

      for (const photo of photos) {
        formData.append('photos', photo, photo.name);
      }

      result = await fetchJson<{ public_id: string; tracking_url: string }>('/api/v1/reports', {
        method: 'POST',
        body: formData
      });
      photos = [];
      selectedTags = [];
      note = '';
      reporterEmail = '';
    } catch (error) {
      const failure = resolveReportSubmitFailure(error);
      errorMessage = t($uiLanguage, failure.messageKey);

      if (failure.queueOffline) {
        try {
          const queuedPhotos = await Promise.all(photos.map((photo) => toDataUrl(photo)));
          enqueueReport({
            location,
            tags: selectedTags,
            note: note || null,
            photos: queuedPhotos,
            client_ts: new Date().toISOString(),
            reporter_email: emailToSubmit,
            ui_language: $uiLanguage
          });
        } catch {
          errorMessage = t($uiLanguage, 'report_error_encode_image');
        }
      }
    } finally {
      submitting = false;
    }
  };

  onMount(() => {
    void loadTags();
    void flushQueue();
    requestLocation();

    fetch('/api/v1/auth/session')
      .then((res) => {
        isLoggedIn = res.ok;
      })
      .catch(() => {
        isLoggedIn = false;
      });

    const handleOnline = (): void => {
      void flushQueue();
    };

    window.addEventListener('online', handleOnline);
    return () => window.removeEventListener('online', handleOnline);
  });
</script>

<div class="report-form-wrap">
  <h1>{t($uiLanguage, 'report_title')}</h1>

  <section class={`location-banner ${locationState}`}>
    <h2>
      <span class="section-icon">
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 10c0 6-8 12-8 12s-8-6-8-12a8 8 0 0 1 16 0Z"/><circle cx="12" cy="10" r="3"/></svg>
      </span>
      {t($uiLanguage, 'report_location_permission')}
    </h2>
    <p>{locationMessage}</p>
    {#if locationState !== 'ready'}
      <button class="retry" type="button" onclick={requestLocation}>
        {t($uiLanguage, 'report_location_retry')}
      </button>
    {/if}
  </section>

  <section class="photo-panel">
    <h2>
      <span class="section-icon">
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z"/><circle cx="12" cy="13" r="3"/></svg>
      </span>
      {t($uiLanguage, 'report_photos_title')}
    </h2>
    <p class="hint">{t($uiLanguage, 'report_photos_hint')}</p>

    <div class="photo-actions">
      <button type="button" class="photo-button" onclick={() => cameraInput.click()}>
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z"/><circle cx="12" cy="13" r="3"/></svg>
        {t($uiLanguage, 'report_take_photo')}
      </button>
      <button type="button" class="photo-button alt" onclick={() => galleryInput.click()}>
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect width="18" height="18" x="3" y="3" rx="2" ry="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/></svg>
        {t($uiLanguage, 'report_choose_gallery')}
      </button>
      <input
        bind:this={cameraInput}
        class="hidden-input"
        type="file"
        accept="image/jpeg,image/webp,image/*"
        capture="environment"
        onchange={onCameraChange}
      />
      <input
        bind:this={galleryInput}
        class="hidden-input"
        type="file"
        accept="image/jpeg,image/webp,image/*"
        multiple
        onchange={onGalleryChange}
      />
    </div>

    <ul class="photo-list">
      {#each photos as photo, index}
        <li>
          <div>
            <strong>{photo.name}</strong>
            <small>{Math.round(photo.size / 1024)} KB</small>
          </div>
          <button type="button" class="remove" onclick={() => removePhoto(index)}>
            {t($uiLanguage, 'report_remove_photo')}
          </button>
        </li>
      {/each}
    </ul>
  </section>

  <section class="tags-panel">
    <h2>{t($uiLanguage, 'report_tags_title')}</h2>
    <div class="tags-grid">
      {#each tagOptions as tag}
        <button
          type="button"
          class:selected={selectedTags.includes(tag.code)}
          onclick={() => toggleTag(tag.code)}
          aria-pressed={selectedTags.includes(tag.code)}
        >
          <span class="tag-icon">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              {@html tagSvgPaths(tag.code)}
            </svg>
          </span>
          {tagLabel($uiLanguage, tag.code, tag.label)}
        </button>
      {/each}
    </div>
  </section>

  <div class="field">
    <label for="note">{t($uiLanguage, 'report_note_label')}</label>
    <textarea id="note" rows="4" bind:value={note} placeholder={t($uiLanguage, 'report_note_placeholder') || ''}></textarea>
  </div>

  {#if !isLoggedIn}
    <div class="field">
      <label for="reporter_email">{t($uiLanguage, 'report_email_label')}</label>
      <p class="hint">{t($uiLanguage, 'report_email_hint')}</p>
      <input
        id="reporter_email"
        type="email"
        placeholder={t($uiLanguage, 'report_email_placeholder')}
        bind:value={reporterEmail}
        autocomplete="email"
      />
    </div>
  {/if}

  <button class="submit" disabled={!submitAllowed()} onclick={submit}>
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m22 2-7 20-4-9-9-4 20-7z"/><path d="M22 2 11 13"/></svg>
    {submitting ? t($uiLanguage, 'report_submitting') : t($uiLanguage, 'report_submit')}
  </button>

  {#if errorMessage}
    <p class="error">{errorMessage}</p>
  {/if}

  {#if result}
    <div class="success">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" style="flex-shrink:0; color: var(--primary); margin-top: 0.1rem"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><path d="m9 11 3 3L22 4"/></svg>
      <div>
        <p>
          <strong>{t($uiLanguage, 'report_success_title')}</strong>
          {t($uiLanguage, 'report_success_reference')}: {result.public_id}
        </p>
        <a href={result.tracking_url}>{t($uiLanguage, 'report_success_track')}</a>
      </div>
    </div>
  {/if}
</div>
