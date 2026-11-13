# Hyla's website

The site at `hyla.rpcx.io`, built with Vite and TanStack Start. The docs are
the Markdown in [`../docs`](../docs), compiled into the build; the pages are
prerendered, so the build is a folder of files a plain web server can serve.

## Develop

```sh
bun install
bun run dev          # http://localhost:3000, hot reload
```

## Build

```sh
bun run build        # dist/client (the site) + dist/server (unused here)
bun run preview      # serve the build: http://localhost:4173
```

`dist/client` is the site: `index.html`, `docs/…`, `assets/…` (content-hashed),
and everything in `public/`. `dist/server` is kept because the Cloudflare
plugin builds it; nothing here needs it.

## Deploy

```sh
./scripts/deploy-static.sh                # builds, then rsyncs to rpcx.io:/opt/hyla
./scripts/deploy-static.sh --dry-run      # what would change
./scripts/deploy-static.sh me@host:/path  # somewhere else
```

It mirrors `dist/client` into the destination with `rsync --delete`: the
destination must be the site's own directory, not a shared web root.

Serve it with [`nginx.conf.example`](./nginx.conf.example): the important part
is `try_files $uri $uri.html $uri/index.html`, because a page lives as
`docs/remote.html` while its address is `/docs/remote`.

## The download page

There is no server, so `/download` finds the latest release in the browser: it
asks `api.github.com` for the tag, keeps it in `localStorage` for six hours,
and until it answers every link goes to `releases/latest`, which GitHub
redirects to the current release. A blocked API, an ad blocker or a rate limit
leaves the page working, one click further from the file.

## Where it is published

`VITE_SITE_URL` is the address the pages say they live at: the canonical link
and the `hreflang` alternates use it. It defaults to `https://hyla.rpcx.io`;
a build for kero.sh sets it to `https://kero.sh`.

## kero.sh

`wrangler.jsonc` still describes the Cloudflare Worker that serves `kero.sh`.
It is not needed for `hyla.rpcx.io`: `bun run deploy` is the Cloudflare path,
`scripts/deploy-static.sh` the other.
