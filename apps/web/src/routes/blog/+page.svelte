<script lang="ts">
  import { onMount } from "svelte";
  import { t, uiLanguage } from "$lib/i18n";
  import type { BlogPost } from "$lib/types";
  import "$lib/styles/blog.css";

  let posts = $state<BlogPost[]>([]);
  let total = $state(0);
  let loading = $state(true);
  let currentPage = $state(1);
  const pageSize = 5;

  async function loadPage(page: number) {
    loading = true;
    try {
      const offset = (page - 1) * pageSize;
      const res = await fetch(
        `/api/v1/blog?limit=${pageSize}&offset=${offset}`,
      );
      if (res.ok) {
        const data = await res.json();
        posts = data.posts || [];
        total = data.total || 0;
        currentPage = page;
      }
    } catch (e) {
      console.error("Failed to load blog posts", e);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadPage(1);
  });
</script>

<svelte:head>
  <title>{t($uiLanguage, "nav_blog")} | ZwerfFiets</title>
</svelte:head>

<div class="blog-container">
  <header class="blog-header">
    <h1>{t($uiLanguage, "nav_blog")}</h1>
  </header>

  <div class="blog-grid">
    {#if loading}
      <p class="empty-state">Loading blog posts...</p>
    {:else}
      {#each posts as post}
        <article class="blog-card">
          <a href="/blog/{post.slug}">
            <h2>{post.title}</h2>
          </a>
          <div class="blog-meta">
            <span class="blog-author">{post.author_name}</span>
            <span class="blog-date">
              {#if post.published_at}
                {new Date(post.published_at).toLocaleDateString($uiLanguage)}
              {/if}
            </span>
          </div>
          <div class="blog-excerpt">
            {@html post.content_html}
          </div>
          <a href="/blog/{post.slug}" class="blog-read-more"> Read more → </a>
        </article>
      {:else}
        <p class="empty-state">No blog posts found yet.</p>
      {/each}
    {/if}
  </div>

  {#if total > pageSize}
    <nav
      class="pagination"
      style="display: flex; justify-content: space-between; margin-top: 2rem;"
    >
      <button
        class="button button-secondary"
        disabled={currentPage === 1}
        onclick={() => loadPage(currentPage - 1)}
      >
        ← {t($uiLanguage, "blog_prev_page")}
      </button>

      <span style="align-self: center;">
        {currentPage} / {Math.ceil(total / pageSize)}
      </span>

      <button
        class="button button-secondary"
        disabled={currentPage >= Math.ceil(total / pageSize)}
        onclick={() => loadPage(currentPage + 1)}
      >
        {t($uiLanguage, "blog_next_page")} →
      </button>
    </nav>
  {/if}
</div>
