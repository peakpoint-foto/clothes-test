# Cloudflare deploy (Workers Static Assets)

The Vietnamese SPA (`index.html`, `data.js`, `assets/`) deploys as a **Workers Static Assets** site. The GoFrame try-on API under `backend/` does **not** run on Cloudflare Workers.

## Deploy

```bash
npm install
npm run deploy   # prepare:public + wrangler deploy
```

Requires Wrangler auth (`npx wrangler login` or `CLOUDFLARE_API_TOKEN` with Workers Scripts Write).

Config: [`wrangler.jsonc`](../wrangler.jsonc) — Worker name `kvpose`, `assets.not_found_handling = single-page-application`.

### Direct assets upload helper (optional)

`scripts/cf-upload-assets.mjs` uploads buckets from an `assets-upload-session` JWT. It **generates** the hash→path map by walking staged `public/` (after `npm run prepare:public`). You can also pass an explicit map:

```bash
npm run prepare:public
node scripts/cf-upload-assets.mjs session.json              # auto map from public/
node scripts/cf-upload-assets.mjs session.json my-map.json  # optional explicit map
```

## Try-on backend

Host `backend/` on a normal server/Docker (see `backend/DEPLOY.md`). In the UI **Settings → Backend API URL**, set that HTTPS origin. Optional: model API keys stay in browser settings (or server env for the Go service). No Cloudflare Worker secrets are required for the static gallery.
