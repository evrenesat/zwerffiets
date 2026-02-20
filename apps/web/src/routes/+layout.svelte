<script lang="ts">
  import { browser } from "$app/environment";
  import { page } from "$app/stores";
  import { onMount } from "svelte";
  import { PUBLIC_BASE_URL, PUBLIC_FB_APP_ID } from "$env/static/public";
  import { derived } from "svelte/store";
  import { t, uiLanguage, loadDynamicContent } from "$lib/i18n";
  import { layoutNavVariant } from "$lib/client/layout-nav";
  import {
    isUserSessionOk,
    USER_LOGOUT_ENDPOINT,
    USER_SESSION_ENDPOINT,
  } from "$lib/client/user-auth";
  import BrandMark from "$lib/components/BrandMark.svelte";
  import "$lib/styles/layout.css";

  let { children } = $props();

  const canonicalBase = (PUBLIC_BASE_URL || "https://zwerffiets.org").replace(
    /\/$/,
    "",
  );
  const buildId = __BUILD_ID__;
  let isUserAuthenticated = $state(false);

  const navVariant = derived(page, ($page) => {
    return layoutNavVariant($page.url.pathname);
  });

  const registerServiceWorker = async (): Promise<void> => {
    if (!browser || !("serviceWorker" in navigator)) {
      return;
    }

    if (!import.meta.env.PROD) {
      const registrations = await navigator.serviceWorker.getRegistrations();
      await Promise.all(
        registrations.map((registration) => registration.unregister()),
      );
      return;
    }

    const registration = await navigator.serviceWorker.register(
      `/service-worker.js?v=${buildId}`,
    );
    await registration.update();

    navigator.serviceWorker.addEventListener("controllerchange", () => {
      window.location.reload();
    });

    registration.addEventListener("updatefound", () => {
      const worker = registration.installing;
      if (!worker) {
        return;
      }

      worker.addEventListener("statechange", () => {
        if (
          worker.state === "installed" &&
          navigator.serviceWorker.controller
        ) {
          worker.postMessage({ type: "SKIP_WAITING" });
        }
      });
    });
  };

  const refreshUserSessionState = async (): Promise<void> => {
    try {
      const response = await fetch(USER_SESSION_ENDPOINT);
      isUserAuthenticated = isUserSessionOk(response.status);
    } catch {
      isUserAuthenticated = false;
    }
  };

  const handleFooterLogout = async (): Promise<void> => {
    await fetch(USER_LOGOUT_ENDPOINT, { method: "POST" });
    isUserAuthenticated = false;
    window.location.href = "/";
  };

  onMount(() => {
    // Remove the blocking class if it exists, to reveal the hydrated content
    document.documentElement.classList.remove("lang-switching");
    void registerServiceWorker();
    void refreshUserSessionState();
    void loadDynamicContent();
  });
</script>

<svelte:head>
  <link rel="icon" href={`/favicon.png?v=${buildId}`} />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <link rel="manifest" href={`/manifest.webmanifest?v=${buildId}`} />
  <meta name="theme-color" content="#ffffff" />
  <link rel="canonical" href={`${canonicalBase}${$page.url.pathname}`} />
  <meta property="og:image" content="{canonicalBase}/logo-zwerffiets.png" />
  <meta property="fb:app_id" content={PUBLIC_FB_APP_ID} />
</svelte:head>

<div class="app-shell">
  <header class="topbar">
    <a class="brand" href="/" aria-label={t($uiLanguage, "app_title")}>
      <BrandMark compact light={true} />
    </a>
    <nav class="topbar-nav">
      {#if $navVariant === "landing"}
        <a class="topbar-link-blog" href="/blog">{t($uiLanguage, "nav_blog")}</a
        >
        <a class="topbar-link-landing" href="#probleem"
          >{t($uiLanguage, "nav_problem")}</a
        >
        <a class="topbar-link-landing" href="#features"
          >{t($uiLanguage, "nav_how_it_works")}</a
        >
        <a class="nav-cta" href="/report">{t($uiLanguage, "nav_report")}</a>
      {:else if $navVariant === "report"}
        <a class="topbar-link-blog" href="/blog">{t($uiLanguage, "nav_blog")}</a
        >
        <a href="/my-reports">{t($uiLanguage, "nav_my_reports")}</a>
      {:else}
        <a class="topbar-link-blog" href="/blog">{t($uiLanguage, "nav_blog")}</a
        >
        <a href="/report">{t($uiLanguage, "nav_report")}</a>
        <a href="/my-reports">{t($uiLanguage, "nav_my_reports")}</a>
      {/if}
    </nav>
    <div class="language-select">
      <select
        bind:value={$uiLanguage}
        aria-label={t($uiLanguage, "language_selector_label")}
      >
        <option value="nl">NL</option>
        <option value="en">EN</option>
      </select>
    </div>
  </header>

  <main>{@render children()}</main>

  <footer class="site-footer">
    <div class="footer-inner">
      <div class="footer-logo">
        <a
          href="https://bikekitchennl.com/"
          target="_blank"
          rel="noopener noreferrer"
        >
          <img src="/images/bike-kitchen.png" alt="Bike Kitchen" />
        </a>
      </div>
      <p>ZwerfFiets – {t($uiLanguage, "footer_tagline")}</p>
      <nav class="footer-nav">
        <a href="/blog">{t($uiLanguage, "nav_blog")}</a>
        <a href="/privacy">{t($uiLanguage, "footer_privacy")}</a>
        <a href="/about">{t($uiLanguage, "footer_about")}</a>
        <a href="https://github.com/zwerffiets/zwerffiets">GitHub</a>
        {#if isUserAuthenticated}
          <button class="footer-logout-button" onclick={handleFooterLogout}>
            {t($uiLanguage, "footer_logout")}
          </button>
        {:else}
          <a href="/login">{t($uiLanguage, "footer_login")}</a>
        {/if}
      </nav>
    </div>
  </footer>
</div>
