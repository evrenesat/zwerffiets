<script lang="ts">
  import { onMount } from "svelte";
  import { preventDefault } from "svelte/legacy";
  import { isUserSessionOk, USER_MAGIC_LINK_ENDPOINT, USER_SESSION_ENDPOINT } from "$lib/client/user-auth";
  import "$lib/styles/login-page.css";

  let email = $state("");
  let loading = $state(false);
  let error = $state("");
  let success = $state(false);
  let checkingSession = $state(true);

  onMount(async () => {
    try {
      const response = await fetch(USER_SESSION_ENDPOINT);
      if (isUserSessionOk(response.status)) {
        window.location.href = "/my-reports";
        return;
      }
    } finally {
      checkingSession = false;
    }
  });

  async function handleSubmit() {
    loading = true;
    error = "";
    success = false;

    try {
      const response = await fetch(USER_MAGIC_LINK_ENDPOINT, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ email }),
      });

      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.message || "Something went wrong");
      }

      success = true;
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }
</script>

<div class="login-wrap">
  <h1>Inloggen</h1>

  {#if checkingSession}
    <div class="alert" role="status">Sessie controleren...</div>
  {:else if success}
    <div class="alert alert-success" role="alert">
      <strong>E-mail verstuurd!</strong>
      Check je mailbox voor de inloglink.
    </div>
  {:else}
    <form onsubmit={preventDefault(handleSubmit)}>
      {#if error}
        <div class="alert alert-error" role="alert">
          <strong>Foutmelding</strong>
          {error}
        </div>
      {/if}

      <div class="field">
        <label for="email">E-mailadres</label>
        <input
          type="email"
          id="email"
          bind:value={email}
          required
          placeholder="jouw@email.nl"
        />
      </div>

      <button type="submit" class="submit" disabled={loading}>
        {loading ? "Laden..." : "Verstuur inloglink"}
      </button>
    </form>
  {/if}
</div>
