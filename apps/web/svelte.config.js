import adapter from '@sveltejs/adapter-static';

const BUILD_ID = process.env.BUILD_ID ?? 'dev';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  kit: {
    adapter: adapter({
      fallback: '200.html'
    }),
    appDir: `_app_${BUILD_ID}`,
    alias: {
      $server: 'src/lib/server'
    }
  }
};

export default config;
