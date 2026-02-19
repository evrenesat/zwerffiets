<script lang="ts">
  import { onMount } from "svelte";
  import { t, uiLanguage } from "$lib/i18n";
  import {
    formatMyReportDate,
    myReportStatusLabel,
    myReportsLoadFailedMessage,
    myReportStatusPath,
    USER_REPORTS_ENDPOINT,
    USER_SESSION_ENDPOINT,
  } from "$lib/client/my-reports";
  import { isUserSessionOk, isUserSessionUnauthorized } from "$lib/client/user-auth";
  import "$lib/styles/my-reports-page.css";

  interface Report {
    id: string;
    publicId: string;
    status: string;
    createdAt: string;
    address?: string;
  }

  let reports = $state<Report[]>([]);
  let loading = $state(true);
  let error = $state("");
  
  function myReportStatusClass(status: string): string {
    return `my-report-status my-report-status-${status}`;
  }

  onMount(async () => {
    try {
      const sessionResponse = await fetch(USER_SESSION_ENDPOINT);

      if (isUserSessionUnauthorized(sessionResponse.status)) {
        window.location.href = "/login";
        return;
      }

      if (!isUserSessionOk(sessionResponse.status)) {
        throw new Error(myReportsLoadFailedMessage($uiLanguage));
      }
      const response = await fetch(USER_REPORTS_ENDPOINT);

      if (response.status === 401) {
        window.location.href = "/login";
        return;
      }

      if (!response.ok) {
        throw new Error(myReportsLoadFailedMessage($uiLanguage));
      }

      reports = await response.json();
    } catch (e: unknown) {
      error = e instanceof Error ? e.message : myReportsLoadFailedMessage($uiLanguage);
    } finally {
      loading = false;
    }
  });
</script>

<section class="my-reports-page">
  <h1 class="my-reports-title">{t($uiLanguage, "my_reports_title")}</h1>

  {#if loading}
    <p class="my-reports-state">{t($uiLanguage, "my_reports_loading")}</p>
  {:else if error}
    <div class="my-reports-error" role="alert">
      <span>{error}</span>
    </div>
  {:else if reports.length === 0}
    <div class="my-reports-empty">
      <p>{t($uiLanguage, "my_reports_empty")}</p>
      <a href="/report" class="my-reports-new-link">
        {t($uiLanguage, "my_reports_new_report")} &rarr;
      </a>
    </div>
  {:else}
    <ul class="my-reports-list">
      {#each reports as report}
        <li class="my-report-item">
          <div class="my-report-top">
            <span class="my-report-id">#{report.publicId}</span>
            <span class={myReportStatusClass(report.status)}>
              {myReportStatusLabel($uiLanguage, report.status)}
            </span>
          </div>
          <p class="my-report-address">
            {report.address || t($uiLanguage, "my_reports_no_address")}
          </p>
          <p class="my-report-time">
            {formatMyReportDate(report.createdAt, $uiLanguage)}
          </p>
          <a href={myReportStatusPath(report.publicId)} class="my-report-link">
            {t($uiLanguage, "my_reports_check_status")}
          </a>
        </li>
      {/each}
    </ul>
  {/if}
</section>
