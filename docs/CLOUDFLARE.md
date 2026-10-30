# Cloudflare deploy (Workers Static Assets)

The Vietnamese SPA (`index.html`, `data.js`, `assets/`) deploys as a **Workers Static Assets** site. The GoFrame try-on API under `backend/` does **not** run on Cloudflare Workers.

## Deploy

```bash
npm install
npm run deploy   # prepare:public + wrangler deploy
```

Requires Wrangler auth (`npx wrangler login` or `CLOUDFLARE_API_TOKEN` with Workers Scripts Write).

Config: [`wrangler.jsonc`](../wrangler.jsonc) — Worker name `kvpose`, `assets.not_found_handling = single-page-application`.

## Try-on backend

Host `backend/` on a normal server/Docker (see `backend/DEPLOY.md`). In the UI **Settings → Backend API URL**, set that HTTPS origin. Optional: model API keys stay in browser settings (or server env for the Go service). No Cloudflare Worker secrets are required for the static gallery.
