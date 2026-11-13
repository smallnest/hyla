import { defineConfig } from 'vite';
import { tanstackStart } from '@tanstack/solid-start/plugin/vite';
import solid from 'vite-plugin-solid';
import { cloudflare } from '@cloudflare/vite-plugin';
import { markdown } from './plugins/markdown';

export default defineConfig({
  plugins: [
    cloudflare({ viteEnvironment: { name: 'ssr' } }),
    // Compiles the Markdown in ../docs, which src/lib/docs.ts imports.
    markdown(),
    tanstackStart({
      // Every page is prerendered, in each language, so the build is a folder
      // of files a plain web server can serve: no Worker, no runtime. A link's
      // #hash is not another page.
      prerender: {
        enabled: true,
        crawlLinks: true,
        autoSubfolderIndex: false,
        filter: (page) => !/#/.test(page.path),
      },
    }),
    solid({ ssr: true }),
  ],
});
