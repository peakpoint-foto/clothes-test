#!/usr/bin/env node
/**
 * Upload Workers static assets using a session JWT from assets-upload-session.
 * Usage: node scripts/cf-upload-assets.mjs <session.json>
 * session.json: { jwt, buckets: string[][] }
 */
import fs from 'fs';
import path from 'path';
import crypto from 'crypto';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '../public');
const accountId = process.env.CLOUDFLARE_ACCOUNT_ID || '4717cdb45968bd12046243c6da6d3b2d';
const sessionPath = process.argv[2];
if (!sessionPath) {
  console.error('Usage: node scripts/cf-upload-assets.mjs <session.json>');
  process.exit(1);
}
const session = JSON.parse(fs.readFileSync(sessionPath, 'utf8'));
const hashToPath = JSON.parse(fs.readFileSync('/tmp/kvpose-hash-to-path.json', 'utf8'));

function mimeFor(filePath) {
  const ext = path.extname(filePath).toLowerCase();
  return {
    '.html': 'text/html; charset=utf-8',
    '.js': 'application/javascript; charset=utf-8',
    '.css': 'text/css; charset=utf-8',
    '.webp': 'image/webp',
    '.png': 'image/png',
    '.jpg': 'image/jpeg',
    '.jpeg': 'image/jpeg',
    '.svg': 'image/svg+xml',
    '.json': 'application/json',
    '.woff2': 'font/woff2',
    '.woff': 'font/woff',
  }[ext] || 'application/octet-stream';
}

let completionJwt = session.jwt;
const buckets = session.buckets || [];
console.log(`Uploading ${buckets.length} bucket(s), ${buckets.reduce((n,b)=>n+b.length,0)} files...`);

for (let i = 0; i < buckets.length; i++) {
  const bucket = buckets[i];
  const form = new FormData();
  for (const hash of bucket) {
    const filePath = hashToPath[hash];
    if (!filePath) throw new Error(`Missing path for hash ${hash}`);
    const b64 = fs.readFileSync(filePath).toString('base64');
    form.append(hash, new Blob([b64], { type: mimeFor(filePath) }));
  }
  const url = `https://api.cloudflare.com/client/v4/accounts/${accountId}/workers/assets/upload?base64=true`;
  process.stdout.write(`  bucket ${i + 1}/${buckets.length} (${bucket.length} files)... `);
  const res = await fetch(url, {
    method: 'POST',
    headers: { Authorization: `Bearer ${session.jwt}` },
    body: form,
  });
  const text = await res.text();
  let json;
  try { json = JSON.parse(text); } catch { throw new Error(`Bad response ${res.status}: ${text.slice(0,300)}`); }
  if (!res.ok || json.success === false) {
    throw new Error(`Upload failed ${res.status}: ${JSON.stringify(json.errors || json)}`);
  }
  if (json.result?.jwt) completionJwt = json.result.jwt;
  console.log('ok');
}

fs.writeFileSync('/tmp/kvpose-completion.json', JSON.stringify({ jwt: completionJwt }, null, 2));
console.log('Wrote /tmp/kvpose-completion.json');
