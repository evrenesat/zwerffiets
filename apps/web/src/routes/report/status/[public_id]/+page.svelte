<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { statusLabel, t, uiLanguage } from '$lib/i18n';
  import '$lib/styles/report-status.css';

  let status: string | null = null;
  let createdAt = '';
  let updatedAt = '';
  let error = '';

  onMount(async () => {
    try {
      const token = $page.url.searchParams.get('token');
      const publicId = $page.params.public_id;
      const headers: Record<string, string> = {};
      if (token) {
        headers.Authorization = `Bearer ${token}`;
        window.history.replaceState({}, '', `/report/status/${publicId}`);
      }

      const response = await fetch(`/api/v1/reports/${publicId}/status`, { headers });
      if (response.status === 401) {
        window.location.href = '/login';
        return;
      }
      if (!response.ok) {
        throw new Error('Status lookup failed');
      }

      const payload = await response.json();
      status = payload.status;
      createdAt = payload.createdAt;
      updatedAt = payload.updatedAt;
    } catch {
      error = t($uiLanguage, 'report_status_error_lookup_failed');
    }
  });
</script>

<section class="card">
  <h1>{t($uiLanguage, 'report_status_title')}</h1>
  {#if error}
    <p>{error}</p>
  {:else if !status}
    <p>{t($uiLanguage, 'report_status_loading')}</p>
  {:else}
    <p><strong>{t($uiLanguage, 'report_status_current')}:</strong> {statusLabel($uiLanguage, status)}</p>
    <p><strong>{t($uiLanguage, 'report_status_created')}:</strong> {createdAt}</p>
    <p><strong>{t($uiLanguage, 'report_status_updated')}:</strong> {updatedAt}</p>
  {/if}
</section>
