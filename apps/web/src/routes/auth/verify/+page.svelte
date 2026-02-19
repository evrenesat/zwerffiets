<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';

  let loading = $state(true);
  let error = $state('');

  onMount(async () => {
    const token = $page.url.searchParams.get('token');

    if (!token) {
      error = 'Geen token gevonden.';
      loading = false;
      return;
    }

    try {
      const response = await fetch('/api/v1/auth/verify?token=' + token);
      
      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.message || 'Verificatie mislukt');
      }

      // Success! Redirect to reports
      window.location.href = '/my-reports';
    } catch (e: any) {
      error = e.message;
      loading = false;
    }
  });
</script>

<div class="max-w-md mx-auto mt-10 p-6 bg-white rounded-lg shadow-md text-center">
  {#if loading}
    <p class="text-gray-600">Bezig met verifiëren...</p>
  {:else if error}
    <div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded relative mb-4" role="alert">
      <strong class="font-bold">Fout!</strong>
      <span class="block sm:inline">{error}</span>
    </div>
    <a href="/login" class="text-indigo-600 hover:text-indigo-800 underline">Probeer opnieuw in te loggen</a>
  {/if}
</div>
