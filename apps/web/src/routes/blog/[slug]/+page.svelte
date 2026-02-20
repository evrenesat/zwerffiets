<script lang="ts">
  import { onMount } from "svelte";
  import { page } from "$app/stores";
  import { t, uiLanguage } from "$lib/i18n";
  import type { BlogPost } from "$lib/types";
  import "$lib/styles/blog.css";

  let post = $state<BlogPost | null>(null);
  let loading = $state(true);
  let notFound = $state(false);

  onMount(async () => {
    try {
      const slug = $page.params.slug;
      const res = await fetch(`/api/v1/blog/${slug}`);
      if (res.ok) {
        post = await res.json();
      } else if (res.status === 404) {
        notFound = true;
      }
    } catch (e) {
      console.error("Failed to load blog post", e);
    } finally {
      loading = false;
    }
  });
</script>

<svelte:head>
  <title>{post ? post.title : "Blog"} | ZwerfFiets</title>
  {#if post}
    <meta name="description" content={post.title} />
  {/if}
</svelte:head>

<article class="blog-container post-detail">
  {#if loading}
    <p class="empty-state">Loading...</p>
  {:else if notFound || !post}
    <h1>404</h1>
    <p>Blog post not found</p>
    <a href="/blog" class="blog-back">← All posts</a>
  {:else}
    <header class="post-header">
      <a href="/blog" class="blog-back">← All posts</a>
      <h1>{post.title}</h1>
      <div class="blog-meta">
        <span class="blog-author">{post.author_name}</span>
        <span class="blog-date">
          {#if post.published_at}
            {new Date(post.published_at).toLocaleDateString($uiLanguage)}
          {/if}
        </span>
      </div>
    </header>

    <div class="post-content">
      {@html post.content_html}
    </div>
  {/if}
</article>
