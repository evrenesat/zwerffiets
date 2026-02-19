<script lang="ts">
  import logo from "$lib/assets/logo-zwerffiets.png";

  interface Props {
    compact?: boolean;
    subtitle?: string;
    light?: boolean;
  }

  let {
    compact = false,
    subtitle = "Meld verlaten fietsen in minder dan een minuut.",
    light = false,
  }: Props = $props();
</script>

<div class:compact class="brandmark" aria-label="ZwerfFiets logo">
  <img src={logo} alt="ZwerfFiets" class="logo-img" />
</div>

<style>
  .brandmark {
    display: inline-flex;
    align-items: center;
    gap: 0.7rem;
  }

  .logo-img {
    width: 8rem;
    object-fit: contain;
    /* Optional: add a slight drop shadow if needed, but the image might already have it */
    filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.1));
  }

  /* Text styles removed as text is no longer displayed */

  .compact .logo-img {
    width: 6.4rem;
    height: 6.4rem;
  }
</style>
