<script lang="ts">
  import { t, uiLanguage } from '$lib/i18n';
</script>

<svelte:head>
  <title>{t($uiLanguage, 'privacy_page_title')} – ZwerfFiets</title>
</svelte:head>

<div class="content-page">
  <div class="content-container">

    <section class="content-section content-hero">
      <p class="section-label">{t($uiLanguage, 'privacy_intro_label')}</p>
      <h1>{t($uiLanguage, 'privacy_intro_title')}</h1>
      <p>{t($uiLanguage, 'privacy_intro_text')}</p>
    </section>

    <section class="content-section">
      <p class="section-label">{t($uiLanguage, 'privacy_data_label')}</p>
      <h2>{t($uiLanguage, 'privacy_data_title')}</h2>
      <div class="privacy-items">
        <div class="privacy-item">
          <h3>{t($uiLanguage, 'privacy_anon_title')}</h3>
          <p>{t($uiLanguage, 'privacy_anon_text')}</p>
        </div>
        <div class="privacy-item">
          <h3>{t($uiLanguage, 'privacy_location_title')}</h3>
          <p>{t($uiLanguage, 'privacy_location_text')}</p>
        </div>
        <div class="privacy-item">
          <h3>{t($uiLanguage, 'privacy_photos_title')}</h3>
          <p>{t($uiLanguage, 'privacy_photos_text')}</p>
        </div>
        <div class="privacy-item">
          <h3>{t($uiLanguage, 'privacy_email_title')}</h3>
          <p>{t($uiLanguage, 'privacy_email_text')}</p>
        </div>
      </div>
    </section>

    <section class="content-section">
      <p class="section-label">{t($uiLanguage, 'privacy_retention_label')}</p>
      <h2>{t($uiLanguage, 'privacy_retention_title')}</h2>
      <p>{t($uiLanguage, 'privacy_retention_text')}</p>
    </section>

    <section class="content-section">
      <p class="section-label">{t($uiLanguage, 'privacy_gdpr_label')}</p>
      <h2>{t($uiLanguage, 'privacy_gdpr_title')}</h2>
      <p>{t($uiLanguage, 'privacy_gdpr_text')}</p>
    </section>

    <section class="content-section">
      <p class="section-label">{t($uiLanguage, 'privacy_contact_label')}</p>
      <h2>{t($uiLanguage, 'privacy_contact_title')}</h2>
      <p>{t($uiLanguage, 'privacy_contact_text')}</p>
    </section>

  </div>
</div>

<style>
  .content-page {
    padding: 3rem 0 5rem;
  }

  .content-container {
    max-width: 52rem;
    margin: 0 auto;
    padding-inline: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 3rem;
  }

  .content-section {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .content-hero h1 {
    font-family: Georgia, 'Palatino Linotype', serif;
    font-size: 2rem;
    line-height: 1.25;
    color: var(--fg);
    margin: 0;
  }

  .content-hero p {
    color: var(--fg-muted);
    line-height: 1.7;
    margin: 0;
    max-width: 60ch;
  }

  .content-section h2 {
    font-family: Georgia, 'Palatino Linotype', serif;
    font-size: 1.4rem;
    color: var(--fg);
    margin: 0;
  }

  .content-section > p {
    color: var(--fg-muted);
    line-height: 1.7;
    margin: 0;
    max-width: 68ch;
  }

  .section-label {
    margin: 0;
    font-size: 0.78rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--primary);
  }

  .privacy-items {
    display: flex;
    flex-direction: column;
    gap: 0;
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    overflow: hidden;
    background: var(--bg-card);
    margin-top: 0.25rem;
  }

  .privacy-item {
    padding: 1.25rem 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .privacy-item + .privacy-item {
    border-top: 1px solid var(--border);
  }

  .privacy-item h3 {
    margin: 0;
    font-size: 0.95rem;
    font-weight: 600;
    color: var(--fg);
  }

  .privacy-item p {
    margin: 0;
    font-size: 0.9rem;
    color: var(--fg-muted);
    line-height: 1.65;
    max-width: 70ch;
  }

  @media (max-width: 640px) {
    .content-hero h1 {
      font-size: 1.6rem;
    }
  }
</style>
