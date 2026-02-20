<script lang="ts">
  import { onMount } from "svelte";
  import { t, uiLanguage } from "$lib/i18n";
  import type { BlogPost } from "$lib/types";
  import "$lib/styles/blog.css";

  let posts = $state<BlogPost[]>([]);
  let total = $state(0);
  let loading = $state(true);

  onMount(async () => {
    try {
      const res = await fetch("/api/v1/blog?limit=10&offset=0");
      if (res.ok) {
        const data = await res.json();
        posts = data.posts || [];
        total = data.total || 0;
      }
    } catch (e) {
      console.error("Failed to load blog posts", e);
    } finally {
      loading = false;
    }
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
            <!-- Basic excerpt: first 200 chars of text content -->
            {@html post.content_html
              .replace(/<[^>]*>/g, "")
              .substring(0, 200)}...
          </div>
          <a href="/blog/{post.slug}" class="blog-read-more"> Read more → </a>
        </article>
      {:else}
        <p class="empty-state">No blog posts found yet.</p>
      {/each}
    {/if}
  </div>

  {#if total > 10}
    <nav class="pagination">
      <!-- Simple pagination could be added here -->
    </nav>
  {/if}
</div>
