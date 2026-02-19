<script lang="ts">
  import { t, uiLanguage } from '$lib/i18n';
</script>

<svelte:head>
  <title>{t($uiLanguage, 'about_page_title')} – ZwerfFiets</title>
</svelte:head>

<div class="content-page">
  <div class="content-container">

    <section class="content-section content-hero">
      <p class="section-label">{t($uiLanguage, 'about_mission_label')}</p>
      <h1>{t($uiLanguage, 'about_mission_title')}</h1>
      <p>{t($uiLanguage, 'about_mission_p1')}</p>
      <p>{t($uiLanguage, 'about_mission_p2')}</p>
    </section>

    <section class="content-section">
      <p class="section-label">{t($uiLanguage, 'about_how_label')}</p>
      <h2>{t($uiLanguage, 'about_how_title')}</h2>
      <div class="steps-cards">
        <div class="step-card">
          <div class="step-number">1</div>
          <div class="step-body">
            <h3>{t($uiLanguage, 'about_how_step1_title')}</h3>
            <p>{t($uiLanguage, 'about_how_step1_desc')}</p>
          </div>
        </div>
        <div class="step-card">
          <div class="step-number">2</div>
          <div class="step-body">
            <h3>{t($uiLanguage, 'about_how_step2_title')}</h3>
            <p>{t($uiLanguage, 'about_how_step2_desc')}</p>
          </div>
        </div>
        <div class="step-card">
          <div class="step-number">3</div>
          <div class="step-body">
            <h3>{t($uiLanguage, 'about_how_step3_title')}</h3>
            <p>{t($uiLanguage, 'about_how_step3_desc')}</p>
          </div>
        </div>
      </div>
    </section>

    <section class="content-section content-row">
      <div>
        <p class="section-label">{t($uiLanguage, 'about_os_label')}</p>
        <h2>{t($uiLanguage, 'about_os_title')}</h2>
        <p>{t($uiLanguage, 'about_os_text')}</p>
        <a class="btn-link" href="https://github.com/zwerffiets/zwerffiets">{t($uiLanguage, 'about_contact_github')}</a>
      </div>
      <div>
        <p class="section-label">{t($uiLanguage, 'about_contact_label')}</p>
        <h2>{t($uiLanguage, 'about_contact_title')}</h2>
        <p>{t($uiLanguage, 'about_contact_text')}</p>
      </div>
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
    gap: 3.5rem;
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
    font-size: 1.5rem;
    color: var(--fg);
    margin: 0;
  }

  .content-section p {
    color: var(--fg-muted);
    line-height: 1.7;
    margin: 0;
  }

  .section-label {
    margin: 0;
    font-size: 0.78rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--primary);
  }

  .steps-cards {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    margin-top: 0.25rem;
  }

  .step-card {
    display: flex;
    gap: 1.25rem;
    align-items: flex-start;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 1.25rem 1.5rem;
  }

  .step-number {
    flex-shrink: 0;
    width: 2rem;
    height: 2rem;
    border-radius: 50%;
    background: var(--primary);
    color: var(--primary-fg);
    display: grid;
    place-items: center;
    font-weight: 700;
    font-size: 0.9rem;
  }

  .step-body {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }

  .step-body h3 {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
    color: var(--fg);
  }

  .step-body p {
    margin: 0;
    font-size: 0.9rem;
    color: var(--fg-muted);
    line-height: 1.6;
  }

  .content-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 2.5rem;
    align-items: start;
  }

  .btn-link {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    margin-top: 0.5rem;
    background: var(--primary);
    color: var(--primary-fg);
    border-radius: var(--radius-sm);
    padding: 0.55rem 1.1rem;
    text-decoration: none;
    font-weight: 600;
    font-size: 0.875rem;
    transition: background 120ms ease;
  }

  .btn-link:hover {
    background: #3d6649;
  }

  @media (max-width: 640px) {
    .content-row {
      grid-template-columns: 1fr;
    }

    .content-hero h1 {
      font-size: 1.6rem;
    }
  }
</style>
