<script lang="ts">
  import { onMount } from "svelte";
  import { PUBLIC_BASE_URL } from "$env/static/public";
  import { t, uiLanguage } from "$lib/i18n";
  import BrandMark from "$lib/components/BrandMark.svelte";
  import "$lib/styles/landing.css";

  interface ShowcaseItem {
    slot: number;
    subtitle: string;
    focalX: number;
    focalY: number;
    scalePercent: number;
    photoUrl: string;
  }

  let showcaseItems: Record<number, ShowcaseItem> = {};

  onMount(async () => {
    try {
      const res = await fetch("/api/v1/showcase");
      if (res.ok) {
        const data = await res.json();
        const items: ShowcaseItem[] = data.items || [];
        showcaseItems = items.reduce(
          (acc: Record<number, ShowcaseItem>, item: ShowcaseItem) => {
            acc[item.slot] = item;
            return acc;
          },
          {},
        );
      }
    } catch (e) {
      console.error("Failed to load showcase items", e);
    }
  });

  const defaultImages: Record<
    number,
    { src: string; altKey: string; captionKey: string }
  > = {
    1: {
      src: "/images/stray-bike-1.jpg",
      altKey: "landing_showcase_fig1_alt",
      captionKey: "landing_showcase_fig1_caption",
    },
    2: {
      src: "/images/stray-bike-2.jpg",
      altKey: "landing_showcase_fig2_alt",
      captionKey: "landing_showcase_fig2_caption",
    },
    3: {
      src: "/images/stray-bike-3.jpg",
      altKey: "landing_showcase_fig3_alt",
      captionKey: "landing_showcase_fig3_caption",
    },
    4: {
      src: "/images/stray-bike-4.jpg",
      altKey: "landing_showcase_fig4_alt",
      captionKey: "landing_showcase_fig4_caption",
    },
  };
</script>

<svelte:head>
  <meta property="og:title" content={t($uiLanguage, "landing_title")} />
  <meta
    property="og:description"
    content={t($uiLanguage, "landing_description")}
  />
  <meta
    property="og:url"
    content={PUBLIC_BASE_URL || "https://zwerffiets.org"}
  />
  <meta
    property="og:image"
    content="{PUBLIC_BASE_URL || 'https://zwerffiets.org'}/logo-zwerffiets.png"
  />
  <meta property="og:type" content="website" />
</svelte:head>

<div class="landing-page">
  <!-- Hero -->
  <section class="lp-hero">
    <div class="lp-container">
      <div class="hero-grid">
        <div class="card hero-intro">
          <h1>{t($uiLanguage, "landing_title")}</h1>
          <p>{t($uiLanguage, "landing_description")}</p>
          <div class="hero-actions">
            <a class="btn-primary" href="/report"
              >{t($uiLanguage, "nav_report")}</a
            >
            <a class="btn-outline" href="#probleem"
              >{t($uiLanguage, "landing_more_info")}</a
            >
          </div>
        </div>
        <div class="card hero-steps">
          <h2>{t($uiLanguage, "landing_how_title")}</h2>
          <ol class="steps-list">
            <li>{t($uiLanguage, "landing_step_1")}</li>
            <li>{t($uiLanguage, "landing_step_2")}</li>
            <li>{t($uiLanguage, "landing_step_3")}</li>
          </ol>
        </div>
      </div>
    </div>
  </section>

  <!-- Stats strip -->
  <section class="lp-stats">
    <div class="lp-container">
      <div class="stats-grid">
        <div class="stat-item">
          <span class="stat-value">1200+</span>
          <span class="stat-label"
            >{t($uiLanguage, "landing_stats_bikes_reported")}</span
          >
        </div>
        <div class="stat-item">
          <span class="stat-value">18</span>
          <span class="stat-label"
            >{t($uiLanguage, "landing_stats_municipalities")}</span
          >
        </div>
        <div class="stat-item">
          <span class="stat-value">&lt;1 min</span>
          <span class="stat-label"
            >{t($uiLanguage, "landing_stats_avg_report_time")}</span
          >
        </div>
        <div class="stat-item">
          <span class="stat-value">73%</span>
          <span class="stat-label"
            >{t($uiLanguage, "landing_stats_resolved_14d")}</span
          >
        </div>
      </div>
    </div>
  </section>

  <!-- Showcase -->
  <section id="probleem" class="lp-showcase">
    <div class="lp-container">
      <div class="showcase-intro">
        <p class="section-label">{t($uiLanguage, "landing_showcase_label")}</p>
        <h2>{t($uiLanguage, "landing_showcase_title")}</h2>
        <p class="showcase-desc">{t($uiLanguage, "landing_showcase_desc")}</p>
      </div>
      <div class="showcase-grid">
        {#each [1, 2, 3, 4] as slot}
          {#if showcaseItems[slot]}
            <figure class="showcase-figure">
              <img
                src={showcaseItems[slot].photoUrl}
                alt={showcaseItems[slot].subtitle}
                style="object-fit: cover; object-position: {showcaseItems[slot]
                  .focalX}% {showcaseItems[slot]
                  .focalY}%; transform-origin: {showcaseItems[slot]
                  .focalX}% {showcaseItems[slot]
                  .focalY}%; transform: scale({(showcaseItems[slot]
                  .scalePercent || 100) / 100});"
              />
              <figcaption>{showcaseItems[slot].subtitle}</figcaption>
            </figure>
          {:else}
            <figure class="showcase-figure">
              <img
                src={defaultImages[slot].src}
                alt={t($uiLanguage, defaultImages[slot].altKey)}
              />
              <figcaption>
                {t($uiLanguage, defaultImages[slot].captionKey)}
              </figcaption>
            </figure>
          {/if}
        {/each}
      </div>
      <div class="showcase-cta card">
        <div class="cta-content">
          <h3>{t($uiLanguage, "landing_showcase_cta_title")}</h3>
          <p>{t($uiLanguage, "landing_showcase_cta_desc")}</p>
        </div>
        <a class="btn-outline" href="/about"
          >{t($uiLanguage, "landing_showcase_cta_btn")}</a
        >
      </div>
    </div>
  </section>

  <!-- Problem / Solution -->
  <section class="lp-problem">
    <div class="lp-container">
      <div class="problem-grid">
        <div class="card problem-card">
          <p class="section-label">{t($uiLanguage, "landing_problem_label")}</p>
          <h2>{t($uiLanguage, "landing_problem_title")}</h2>
          <p class="problem-text">{t($uiLanguage, "landing_problem_text")}</p>
          <ul class="bullet-list red">
            <li>{t($uiLanguage, "landing_problem_li1")}</li>
            <li>{t($uiLanguage, "landing_problem_li2")}</li>
            <li>{t($uiLanguage, "landing_problem_li3")}</li>
            <li>{t($uiLanguage, "landing_problem_li4")}</li>
          </ul>
        </div>
        <div class="card solution-card">
          <p class="section-label">
            {t($uiLanguage, "landing_solution_label")}
          </p>
          <h2>{t($uiLanguage, "landing_solution_title")}</h2>
          <p class="problem-text">{t($uiLanguage, "landing_solution_text")}</p>
          <ul class="bullet-list primary">
            <li>{t($uiLanguage, "landing_solution_li1")}</li>
            <li>{t($uiLanguage, "landing_solution_li2")}</li>
            <li>{t($uiLanguage, "landing_solution_li3")}</li>
            <li>{t($uiLanguage, "landing_solution_li4")}</li>
          </ul>
        </div>
      </div>
    </div>
  </section>

  <!-- Features -->
  <section id="features" class="lp-features">
    <div class="lp-container">
      <p class="section-label">{t($uiLanguage, "landing_features_label")}</p>
      <h2>{t($uiLanguage, "landing_features_title")}</h2>
      <p class="features-desc">{t($uiLanguage, "landing_features_desc")}</p>
      <div class="features-grid">
        <div class="feature-card">
          <div class="feature-icon">
            <svg
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
              ><path
                d="M20 10c0 6-8 12-8 12s-8-6-8-12a8 8 0 0 1 16 0Z"
              /><circle cx="12" cy="10" r="3" /></svg
            >
          </div>
          <h3>{t($uiLanguage, "landing_feature1_title")}</h3>
          <p>{t($uiLanguage, "landing_feature1_desc")}</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">
            <svg
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
              ><path
                d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z"
              /><circle cx="12" cy="13" r="3" /></svg
            >
          </div>
          <h3>{t($uiLanguage, "landing_feature2_title")}</h3>
          <p>{t($uiLanguage, "landing_feature2_desc")}</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">
            <svg
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
              ><path
                d="M12 2H2v10l9.29 9.29c.94.94 2.48.94 3.42 0l6.58-6.58c.94-.94.94-2.48 0-3.42L12 2Z"
              /><path d="M7 7h.01" /></svg
            >
          </div>
          <h3>{t($uiLanguage, "landing_feature3_title")}</h3>
          <p>{t($uiLanguage, "landing_feature3_desc")}</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">
            <svg
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
              ><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" /></svg
            >
          </div>
          <h3>{t($uiLanguage, "landing_feature4_title")}</h3>
          <p>{t($uiLanguage, "landing_feature4_desc")}</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">
            <svg
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
              ><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" /><circle
                cx="9"
                cy="7"
                r="4"
              /><path d="M23 21v-2a4 4 0 0 0-3-3.87" /><path
                d="M16 3.13a4 4 0 0 1 0 7.75"
              /></svg
            >
          </div>
          <h3>{t($uiLanguage, "landing_feature5_title")}</h3>
          <p>{t($uiLanguage, "landing_feature5_desc")}</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">
            <svg
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
              ><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z" /><path
                d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"
              /></svg
            >
          </div>
          <h3>{t($uiLanguage, "landing_feature6_title")}</h3>
          <p>{t($uiLanguage, "landing_feature6_desc")}</p>
        </div>
      </div>
    </div>
  </section>

  <!-- CTA -->
  <section class="lp-cta">
    <div class="lp-container">
      <div class="cta-card card">
        <h2>{t($uiLanguage, "landing_cta_title")}</h2>
        <p>{t($uiLanguage, "landing_cta_desc")}</p>
        <div class="cta-actions">
          <a class="btn-primary" href="/report"
            >{t($uiLanguage, "nav_report")}</a
          >
          <a class="btn-outline" href="/about"
            >{t($uiLanguage, "landing_cta_more")}</a
          >
        </div>
      </div>
    </div>
  </section>
</div>
