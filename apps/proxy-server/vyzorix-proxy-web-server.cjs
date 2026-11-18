/**
 * vyzorix-proxy-web-server.cjs
 *
 * Static file serving for the web app.
 *
 * Serves pre-built assets from apps/VyzoriX_web/dist/client/ with SPA fallback
 * (index.html for client-side routing). No vite involvement — the web app must
 * be built first (cd apps/VyzoriX_web && pnpm build).
 *
 * Works identically in dev and prod: the only difference is cache headers
 * and verbosity. In dev, no-cache is used everywhere; in prod, hashed assets
 * get immutable caching.
 */

'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const stream = require('node:stream');
var cfg = require('./vyzorix-proxy-config.cjs');
var logger = require('./vyzorix-proxy-logger.cjs');
var config = cfg.config;
var debug = logger.debug;
var warn = logger.warn;
var error = logger.error;

var MIME_TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript; charset=utf-8',
  '.mjs': 'application/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif': 'image/gif',
  '.ico': 'image/x-icon',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
  '.ttf': 'font/ttf',
  '.eot': 'application/vnd.ms-fontobject',
  '.webp': 'image/webp',
  '.webmanifest': 'application/manifest+json',
  '.map': 'application/json; charset=utf-8',
  '.txt': 'text/plain; charset=utf-8',
  '.xml': 'application/xml; charset=utf-8',
  '.wasm': 'application/wasm',
};

/**
 * Content-Security-Policy for the app document.
 *
 * The Go API sends its own CSP, but only on JSON responses; it never serves the
 * document, so the policy that permits WebAssembly has to live here.
 */
var DOCUMENT_CSP = [
  "default-src 'self'",
  // Required for WebAssembly.compile/instantiate. Narrower than 'unsafe-eval':
  // it permits WASM compilation and nothing else.
  "script-src 'self' 'wasm-unsafe-eval'",
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  "font-src 'self' data:",
  "connect-src 'self' ws: wss:",
  // The OPFS capability probe starts a throwaway worker from a Blob URL.
  "worker-src 'self' blob:",
  "object-src 'none'",
  "base-uri 'self'",
  "form-action 'self'",
  "frame-ancestors 'none'",
].join('; ');

/**
 * Headers for every HTML response (the document and the SPA fallback).
 *
 * COOP/COEP are deliberately absent: OPFS createSyncAccessHandle() does not
 * require cross-origin isolation (only SharedArrayBuffer does), and
 * COEP: require-corp would break cross-origin resources for no benefit.
 */
function documentHeaders() {
  return {
    'Content-Security-Policy': DOCUMENT_CSP,
    'X-Content-Type-Options': 'nosniff',
    'Referrer-Policy': 'strict-origin-when-cross-origin',
    'X-Frame-Options': 'DENY',
    // The HTML shell names the hashed asset files, so a cached copy keeps
    // pointing at a previous build after a redeploy and the browser silently runs
    // stale JavaScript. `no-store` forces a fresh fetch of the shell each time;
    // the assets themselves are content-hashed and can be cached forever.
    'Cache-Control': 'no-store, must-revalidate',
  };
}

/**
 * A CSP that also permits inline scripts carrying `nonce`.
 *
 * A server-rendered page carries an inline hydration script whose content varies
 * per request (it embeds match timestamps), so a static hash cannot cover it and
 * hashing per response is fragile — the header and the body must stay paired, and
 * any re-request breaks the pairing. A per-request nonce avoids that entirely: it
 * authorises the scripts the renderer just emitted, whatever their content.
 *
 * @param {string} nonce - base64 nonce, also injected into each inline script
 * @returns {string} the Content-Security-Policy header value
 */
function cspWithNonce(nonce) {
  return DOCUMENT_CSP.split('; ')
    .map(function (d) {
      return d.indexOf('script-src ') === 0 ? d + " 'nonce-" + nonce + "'" : d;
    })
    .join('; ');
}

/**
 * Add `nonce="…"` to every inline <script> in `html`.
 *
 * External scripts are left alone: 'self' already covers them, and a nonce on a
 * tag the policy does not require it on is noise.
 */
function injectScriptNonce(html, nonce) {
  return html.replace(/<script\b([^>]*)>/gi, function (whole, attrs) {
    if (/\bsrc\s*=/i.test(attrs)) return whole;
    if (/\bnonce\s*=/i.test(attrs)) return whole;
    return '<script nonce="' + nonce + '"' + attrs + '>';
  });
}

/**
 * Serve a static file or SPA fallback.
 * @param {string} pathname - URL pathname
 * @param {object} res - http.ServerResponse
 */
async function serveStaticFile(pathname, res) {
  var staticDir = config.webStaticDir;

  // Check if dist/client exists.
  try {
    await fs.promises.stat(staticDir);
  } catch {
    var msg =
      '<!DOCTYPE html><html><body>' +
      '<h1>Vyzorix Proxy Server</h1>' +
      '<p>Web app not built. Run:</p>' +
      '<pre>cd apps/VyzoriX_web && pnpm build</pre>' +
      '<p>Then restart the proxy.</p>' +
      '</body></html>';
    warn('Web app dist not found, serving placeholder', { dir: staticDir });
    res.writeHead(200, Object.assign({ 'Content-Type': 'text/html; charset=utf-8' }, documentHeaders()));
    res.end(msg);
    return;
  }

  // Normalize and prevent path traversal.
  var safePath = path.normalize(pathname).replace(/^(\.\.[/\\])+/, '');
  var filePath = path.join(staticDir, safePath);
  var ext = path.extname(safePath);

  // For extensionless paths, try the directory's index.html first.
  if (!ext) {
    var dirIndex = path.join(filePath, 'index.html');
    try {
      var dirStats = await fs.promises.stat(dirIndex);
      if (dirStats.isFile()) {
        filePath = dirIndex;
        ext = '.html';
      }
    } catch {
      // Not a directory with index.html — fall through to SPA root index.
      filePath = path.join(staticDir, 'index.html');
      ext = '.html';
    }
  }

  try {
    var stats = await fs.promises.stat(filePath);
    if (!stats.isFile()) {
      return serveSpaIndex(staticDir, res);
    }

    var contentType = MIME_TYPES[ext] || 'application/octet-stream';
    var cacheControl;
    if (config.mode === 'production' && safePath.includes('/assets/')) {
      cacheControl = 'public, max-age=31536000, immutable';
    } else {
      cacheControl = 'no-cache';
    }

    var headers = {
      'Content-Type': contentType,
      'Cache-Control': cacheControl,
      'Content-Length': stats.size,
    };

    // The document gets the security headers; hashed assets do not need them.
    if (contentType.indexOf('text/html') === 0) {
      Object.assign(headers, documentHeaders());
    }

    res.writeHead(200, headers);

    // Stream the file.
    var fileStream = fs.createReadStream(filePath);
    stream.pipeline(fileStream, res, function (err) {
      if (err) {
        error('File stream error', { path: filePath, error: err.message });
        if (!res.writableEnded) res.end();
      }
    });
    debug('Served static file', { path: safePath, type: contentType });
  } catch {
    // SPA fallback: serve index.html for client-side routing.
    return serveSpaIndex(staticDir, res);
  }
}

async function serveSpaIndex(staticDir, res) {
  var indexPath = path.join(staticDir, 'index.html');
  try {
    var stats = await fs.promises.stat(indexPath);
    res.writeHead(
      200,
      Object.assign(
        {
          'Content-Type': 'text/html; charset=utf-8',
          'Cache-Control': 'no-cache',
          'Content-Length': stats.size,
        },
        documentHeaders()
      )
    );
    var fileStream = fs.createReadStream(indexPath);
    stream.pipeline(fileStream, res, function (err) {
      if (err) {
        error('SPA index stream error', { error: err.message });
        if (!res.writableEnded) res.end();
      }
    });
  } catch {
    warn('index.html not found — web app may not be built');
    res.writeHead(200, Object.assign({ 'Content-Type': 'text/html; charset=utf-8' }, documentHeaders()));
    res.end(
      '<!DOCTYPE html><html><body>' +
      '<h1>Vyzorix Proxy Server</h1>' +
      '<p>Web app not built. Run: cd apps/VyzoriX_web && pnpm build</p>' +
      '</body></html>'
    );
  }
}

module.exports = {
  serveStaticFile,
  MIME_TYPES,
  DOCUMENT_CSP,
  documentHeaders,
  cspWithNonce,
  injectScriptNonce,
};
