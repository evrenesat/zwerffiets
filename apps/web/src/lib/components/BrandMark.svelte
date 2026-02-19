<script lang="ts">
interface Props {
  compact?: boolean;
  subtitle?: string;
  light?: boolean;
}

let { compact = false, subtitle = 'Meld verlaten fietsen in minder dan een minuut.', light = false }: Props = $props();
</script>

<div class:compact class="brandmark" aria-label="ZwerfFiets logo">
  <div class="emoji" aria-hidden="true">🚲</div>
  <div class="text">
    <strong style:color={light ? '#171717' : '#f7fbff'}>ZwerfFiets</strong>
    {#if !compact}
      <span style:color={light ? '#6b7280' : '#d8efff'}>{subtitle}</span>
    {/if}
  </div>
</div>

<style>
  .brandmark {
    display: inline-flex;
    align-items: center;
    gap: 0.7rem;
  }

  .emoji {
    width: 3rem;
    height: 3rem;
    border-radius: 0.9rem;
    display: grid;
    place-items: center;
    font-size: 1.8rem;
    background: #4a7c59;
    box-shadow: 0 4px 12px rgba(74, 124, 89, 0.3);
  }

  .text {
    display: grid;
    gap: 0.1rem;
  }

  strong {
    font-size: 1.1rem;
    letter-spacing: 0.01em;
  }

  span {
    font-size: 0.82rem;
  }

  .compact .emoji {
    width: 2.2rem;
    height: 2.2rem;
    font-size: 1.3rem;
    border-radius: 0.7rem;
  }

  .compact strong {
    font-size: 1rem;
  }
</style>
