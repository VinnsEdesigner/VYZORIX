/**
 * vyzorix-proxy-ssr.cjs
 *
 * Server-side rendering for the web app.
 *
 * The web build emits two halves: `dist/client/assets` (the browser bundle) and
 * `dist/server/server.js`, a web-standard `fetch` handler that renders the
 * document. This module loads that handler once and turns a Node request into a
 * Response the proxy can write back.
 *
 * Rendering is optional and best-effort: if the build is absent or the handler
 * fails to load, `isReady()` stays false and the caller falls back to serving
 * static files. A missing SSR build must not stop the app from booting.
 */

'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

var cfg = require('./vyzorix-proxy-config.cjs');
var logger = require('./vyzorix-proxy-logger.cjs');
var webServer = require('./vyzorix-proxy-web-server.cjs');

var config = cfg.config;
var info = logger.info;
var warn = logger.warn;
var error = logger.error;

/** Methods that carry a body and must be forwarded as a stream. */
var BODY_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

var fetchHandler = null;
var ready = false;

/** The built SSR entry, if this deployment has one. */
function ssrEntryPath() {
  return path.join(config.webStaticDir, '..', 'server', 'server.js');
}

/**
 * Load the SSR entry once.
 *
 * Returns true when a handler is available. Safe to call repeatedly; the first
 * successful load is cached and a failure is not retried.
 */
async function init() {
  var entry = ssrEntryPath();

  if (!fs.existsSync(entry)) {
    warn('SSR entry not found; serving static files only', { entry: entry });
    return false;
  }

  try {
    var mod = await import(pathToFileURL(entry).href);
    var handler = (mod.default && mod.default.fetch) || mod.fetch;

    if (typeof handler !== 'function') {
      warn('SSR entry has no fetch handler; serving static files only', { entry: entry });
      return false;
    }

    fetchHandler = handler;
    ready = true;
    info('SSR handler loaded', { entry: entry });
    return true;
  } catch (err) {
    error('Failed to load the SSR entry; serving static files only', {
      entry: entry,
      error: err.message,
    });
    return false;
  }
}

/** Whether a render is available. */
function isReady() {
  return ready;
}

/**
 * Render a request to a Response.
 *
 * The Node request is converted to a web Request so the handler sees the same
 * shape it would in production. Body-carrying methods forward the stream.
 */
function render(req, res) {
  var host = req.headers.host || 'localhost';
  var protocol = req.headers['x-forwarded-proto'] || 'http';
  var url = protocol + '://' + host + (req.url || '/');

  var request = new Request(url, {
    method: req.method,
    headers: req.headers,
    body: BODY_METHODS.has(req.method) ? req : undefined,
    duplex: BODY_METHODS.has(req.method) ? 'half' : undefined,
  });

  return fetchHandler(request, process.env, {});
}

/**
 * Render a request and write the response to the Node response.
 *
 * The document security headers are applied on top of whatever the handler
 * returned, so a rendered page carries the same CSP as a statically served one.
 * Without them a rendered document has no 'wasm-unsafe-eval' and the local store
 * cannot start.
 *
 * A per-request nonce authorises the renderer's inline hydration script, whose
 * content varies per request and so cannot be covered by a static hash. The same
 * nonce goes into the header and into each inline tag, so the two cannot drift.
 */
async function serve(req, res) {
  var response = await render(req, res);
  var body = await response.text();

  var nonce = crypto.randomBytes(16).toString('base64');
  var html = webServer.injectScriptNonce(body, nonce);

  var headers = webServer.documentHeaders();
  headers['Content-Security-Policy'] = webServer.cspWithNonce(nonce);

  res.statusCode = response.status;

  // Security headers first, then the handler's own, so the handler can override.
  for (var name in headers) {
    res.setHeader(name, headers[name]);
  }

  response.headers.forEach(function (value, key) {
    // Set-Cookie may repeat; res.setHeader would collapse it to one.
    if (key.toLowerCase() === 'set-cookie') return;
    if (key.toLowerCase() === 'content-length') return;
    try {
      res.setHeader(key, value);
    } catch {
      // A header Node refuses to set is not fatal.
    }
  });

  var cookies = response.headers.getSetCookie ? response.headers.getSetCookie() : [];
  if (cookies.length) res.setHeader('set-cookie', cookies);

  res.setHeader('content-length', Buffer.byteLength(html));
  res.end(html);
}

module.exports = { init, isReady, render, serve, ssrEntryPath };
