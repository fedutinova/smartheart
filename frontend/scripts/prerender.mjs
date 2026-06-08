// Post-build prerender of public pages into static HTML.
//
// SmartHeart is a client-rendered SPA: page <title>/<meta> are set by
// useMetaTags() only after the JS bundle boots. Crawlers — Yandex in
// particular, which matters for a Russian-language medical service — see a
// near-empty shell. This script runs the *built* app in a headless browser
// for each public route and snapshots the fully-rendered HTML (content +
// meta) to dist/<route>/index.html, so the first byte already contains the
// real title, description and visible text.
//
// Caddy must resolve these: `try_files {path} {path}/index.html /index.html`.

import http from 'node:http';
import { createReadStream, existsSync, statSync, readFileSync, mkdirSync, writeFileSync } from 'node:fs';
import { join, dirname, extname } from 'node:path';
import { fileURLToPath } from 'node:url';
import puppeteer from 'puppeteer';

const DIST = join(dirname(fileURLToPath(import.meta.url)), '..', 'dist');

// Public, indexable routes (must mirror sitemap.xml).
const ROUTES = ['/', '/calculators', '/qtc', '/killip', '/cha2ds2-vasc', '/contacts'];

const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.mjs': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.webp': 'image/webp',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
  '.ttf': 'font/ttf',
  '.map': 'application/json; charset=utf-8',
  '.webmanifest': 'application/manifest+json',
};

if (!existsSync(join(DIST, 'index.html'))) {
  console.error('[prerender] dist/index.html not found — run `vite build` first.');
  process.exit(1);
}

// Serve the ORIGINAL shell for SPA fallback from memory, so overwriting
// dist/index.html mid-run never poisons later route renders.
const shellHtml = readFileSync(join(DIST, 'index.html'));

function startServer() {
  const server = http.createServer((req, res) => {
    const urlPath = decodeURIComponent((req.url || '/').split('?')[0]);
    const filePath = join(DIST, urlPath);

    // Real asset on disk → serve it. Otherwise SPA fallback to the shell.
    if (urlPath !== '/' && existsSync(filePath) && statSync(filePath).isFile()) {
      res.writeHead(200, { 'Content-Type': MIME[extname(filePath)] || 'application/octet-stream' });
      createReadStream(filePath).pipe(res);
      return;
    }
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
    res.end(shellHtml);
  });
  return new Promise((resolve) => server.listen(0, '127.0.0.1', () => resolve(server)));
}

async function run() {
  const server = await startServer();
  const { port } = server.address();
  const base = `http://127.0.0.1:${port}`;

  const browser = await puppeteer.launch({ headless: 'new', args: ['--no-sandbox', '--disable-setuid-sandbox'] });

  let failures = 0;
  try {
    for (const route of ROUTES) {
      const page = await browser.newPage();
      try {
        await page.goto(base + route, { waitUntil: 'domcontentloaded', timeout: 30_000 });
        // Wait past the isInitializing gate until the route's real content
        // (an <h1>) is in the DOM, then a short settle for useMetaTags effects.
        await page.waitForSelector('#root h1', { timeout: 30_000 });
        await new Promise((r) => setTimeout(r, 500));

        const html = await page.content();
        const outDir = route === '/' ? DIST : join(DIST, route);
        mkdirSync(outDir, { recursive: true });
        writeFileSync(join(outDir, 'index.html'), html);

        const title = await page.title();
        console.log(`[prerender] ${route.padEnd(16)} → ${join(outDir.replace(DIST, 'dist'), 'index.html')}  (${title})`);
      } catch (err) {
        failures++;
        console.error(`[prerender] FAILED ${route}: ${err.message}`);
      } finally {
        await page.close();
      }
    }
  } finally {
    await browser.close();
    server.close();
  }

  if (failures) {
    console.error(`[prerender] ${failures} route(s) failed.`);
    process.exit(1);
  }
  console.log(`[prerender] done — ${ROUTES.length} routes.`);
}

run().catch((err) => {
  console.error('[prerender] fatal:', err);
  process.exit(1);
});
