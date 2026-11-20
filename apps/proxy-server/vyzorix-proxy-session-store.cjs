/**
 * vyzorix-proxy-session-store.cjs
 *
 * Server-side session store.
 *
 * Each session holds the per-session signing key, auth tokens, CSRF token,
 * and organization context — none of which are exposed to the browser. The
 * browser only holds an opaque httpOnly cookie (_vyz_proxy_sid) that maps
 * to a session ID in this store.
 *
 * Sessions expire after sessionTtlMs (default 24h) and are cleaned up lazily.
 */

'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
var cfg = require('./vyzorix-proxy-config.cjs');
var logger = require('./vyzorix-proxy-logger.cjs');
var config = cfg.config;
var debug = logger.debug;
var info = logger.info;
var warn = logger.warn;

/**
 * The session fields worth keeping. Everything a session holds is server-side
 * only, and all of it is needed to resume after a restart — a session restored
 * without its signing key or tokens would be signed out in practice.
 */
var PERSISTED_FIELDS = [
  'id',
  'createdAt',
  'lastAccess',
  'signingKey',
  'accessToken',
  'refreshToken',
  'tokenExpiresAt',
  'csrfToken',
  'csrfCookie',
  'organizationId',
  'userId',
  'apiSessionCookie',
  // Set on logout. A defunct record keeps its id and timestamp for audit and is
  // stripped of every credential, so it can never authenticate again.
  'defunctAt',
];

function newSessionId() {
  return crypto.randomBytes(32).toString('hex');
}

function createSession() {
  var now = Date.now();
  return {
    id: newSessionId(),
    createdAt: now,
    lastAccess: now,
    signingKey: null,
    accessToken: null,
    refreshToken: null,
    tokenExpiresAt: null,
    csrfToken: null,
    csrfCookie: null,
    organizationId: null,
    userId: null,
    apiSessionCookie: null,
    defunctAt: null,
  };
}

/**
 * How long a defunct record is kept before it is pruned. Long enough to answer
 * "when did this session end", short enough that the file cannot grow without
 * bound: every write rewrites the whole array.
 */
var DEFUNCT_RETENTION_MS = (function () {
  var raw = process.env.PROXY_DEFUNCT_RETENTION_MS;
  var n = raw ? parseInt(raw, 10) : NaN;
  return Number.isNaN(n) ? 24 * 60 * 60 * 1000 : n;
})();

/**
 * A defunct session is one that was logged out. Its record survives for audit,
 * but it must never be treated as live.
 */
function isDefunct(sess) {
  return Boolean(sess && sess.defunctAt);
}

var sessions = new Map();

/**
 * Persistence.
 *
 * Sessions live in memory and are mirrored to a JSON file. A restart is common
 * during development and a redeploy is routine in production, and without this
 * every one of them signs every user out.
 *
 * The write is atomic (temp file + rename) so a crash mid-write cannot leave a
 * truncated file that fails to parse on the next start, and it is debounced
 * because a session is touched on every request.
 */
var PERSIST_DEBOUNCE_MS = 1000;
var persistTimer = null;
var persistPending = false;

function snapshot() {
  var now = Date.now();
  var out = [];
  sessions.forEach(function (sess) {
    if (isDefunct(sess)) {
      // Keep the record for audit, but only for the retention window: the whole
      // array is rewritten on every persist, so an unbounded set would make
      // each write progressively more expensive.
      if (now - sess.defunctAt > DEFUNCT_RETENTION_MS) return;
    } else if (now - sess.lastAccess > config.sessionTtlMs) {
      return;
    }

    var record = {};
    PERSISTED_FIELDS.forEach(function (field) {
      record[field] = sess[field];
    });
    out.push(record);
  });
  return out;
}

function persistNow() {
  persistPending = false;
  if (persistTimer) {
    clearTimeout(persistTimer);
    persistTimer = null;
  }

  var file = config.sessionFile;
  if (!file) return;

  try {
    fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 });
    var tmp = file + '.tmp';
    // mode 0600: the file holds signing keys and tokens, never world-readable.
    fs.writeFileSync(tmp, JSON.stringify(snapshot()), { encoding: 'utf8', mode: 0o600 });
    fs.renameSync(tmp, file);
  } catch (err) {
    warn('Could not persist sessions', { file: file, error: err.message });
  }
}

function schedulePersist() {
  persistPending = true;
  if (persistTimer) return;
  persistTimer = setTimeout(persistNow, PERSIST_DEBOUNCE_MS);
  if (persistTimer.unref) persistTimer.unref();
}

/**
 * Load persisted sessions, dropping any that have already expired.
 */
function loadSessions() {
  var file = config.sessionFile;
  if (!file || !fs.existsSync(file)) return;

  try {
    var raw = fs.readFileSync(file, 'utf8');
    if (!raw.trim()) return;
    var parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return;

    var now = Date.now();
    var loaded = 0;
    parsed.forEach(function (record) {
      if (!record || typeof record.id !== 'string') return;
      if (record.defunctAt) {
        if (now - record.defunctAt > DEFUNCT_RETENTION_MS) return;
      } else if (now - (record.lastAccess || 0) > config.sessionTtlMs) {
        return;
      }
      var sess = createSession();
      PERSISTED_FIELDS.forEach(function (field) {
        if (field in record) sess[field] = record[field];
      });
      sess.id = record.id;
      sessions.set(sess.id, sess);
      loaded++;
    });

    if (loaded > 0) {
      info('Restored sessions from disk', { count: loaded, file: file });
    }
  } catch (err) {
    // A corrupt file must not stop the proxy from starting: warn and carry on
    // with an empty store, which only costs the users a re-login.
    warn('Could not read the session file, starting empty', { file: file, error: err.message });
  }
}

/**
 * Get or create a session for the given cookie header.
 * @param {string|undefined} cookieHeader
 * @returns {ProxySession}
 */
function getOrCreateSession(cookieHeader) {
  var existingId = extractCookie(cookieHeader, config.sessionCookieName);
  if (existingId) {
    var sess = sessions.get(existingId);
    // A defunct (logged-out) record must never be resumed: the cookie is still
    // in the browser until it is cleared, and handing the record back would
    // resurrect a session the user ended.
    if (sess && !isDefunct(sess) && !isExpired(sess)) {
      sess.lastAccess = Date.now();
      return sess;
    }
    if (sess) {
      debug('Session expired, removing', { id: existingId.slice(0, 12) });
      sessions.delete(existingId);
      schedulePersist();
    }
  }
  var newSess = createSession();
  sessions.set(newSess.id, newSess);
  debug('Created new proxy session', { id: newSess.id.slice(0, 12) });
  schedulePersist();
  return newSess;
}

/**
 * Get an existing session without creating one.
 * @param {string|undefined} cookieHeader
 * @returns {ProxySession|null}
 */
function getSession(cookieHeader) {
  var id = extractCookie(cookieHeader, config.sessionCookieName);
  if (!id) return null;
  var sess = sessions.get(id);
  if (!sess || isDefunct(sess) || isExpired(sess)) {
    if (sess && !isDefunct(sess)) {
      sessions.delete(id);
      schedulePersist();
    }
    return null;
  }
  sess.lastAccess = Date.now();
  // The last-access stamp is what the TTL is measured from, so it must survive a
  // restart; a debounced write keeps this off the request path.
  schedulePersist();
  return sess;
}

/**
 * End a session (called on logout).
 *
 * The record is marked defunct rather than deleted. Two reasons:
 *
 *   1. Audit. "This session ended at T" is answerable after the fact, which a
 *      deletion is not.
 *   2. Correctness. Every credential is stripped in the same step, so a defunct
 *      record can never authenticate, and every read path refuses it. Deleting
 *      the row was the intent, but nothing on the logout path called this, so
 *      logged-out sessions accumulated in the file as credential-less records.
 *
 * The record is pruned after DEFUNCT_RETENTION_MS.
 */
function destroySession(cookieHeader) {
  var id = extractCookie(cookieHeader, config.sessionCookieName);
  if (!id) return null;

  var sess = sessions.get(id);
  if (!sess) return null;

  if (isDefunct(sess)) {
    // Idempotent: a repeated logout must not move the timestamp.
    return sess;
  }

  var userId = sess.userId;
  sess.defunctAt = Date.now();
  // Strip every credential in the same step that marks the record dead, so the
  // two can never disagree.
  sess.signingKey = null;
  sess.accessToken = null;
  sess.refreshToken = null;
  sess.tokenExpiresAt = null;
  sess.csrfToken = null;
  sess.csrfCookie = null;
  sess.apiSessionCookie = null;

  info('Session marked defunct', { id: id.slice(0, 12), userId: userId });
  schedulePersist();
  return sess;
}

/**
 * Build the Set-Cookie header value for a session.
 */
function sessionCookieHeader(sess) {
  var flags = [
    config.sessionCookieName + '=' + sess.id,
    'Path=/',
    'HttpOnly',
    'SameSite=Lax',
    'Max-Age=' + Math.floor(config.sessionTtlMs / 1000),
  ];
  if (config.mode === 'production') flags.push('Secure');
  return flags.join('; ');
}

/**
 * Clear the session cookie header value.
 */
function clearSessionCookieHeader() {
  return config.sessionCookieName + '=; Path=/; HttpOnly; SameSite=Lax; Max-Age=0';
}

function isExpired(sess) {
  return Date.now() - sess.lastAccess > config.sessionTtlMs;
}

/**
 * Lazy cleanup of expired sessions.
 */
function cleanupExpiredSessions() {
  var now = Date.now();
  var removed = 0;
  sessions.forEach(function (sess, id) {
    if (now - sess.lastAccess > config.sessionTtlMs) {
      sessions.delete(id);
      removed++;
    }
  });
  if (removed > 0) {
    warn('Cleaned up expired sessions', { count: removed });
    schedulePersist();
  }
  return removed;
}

/**
 * Parse a cookie header and extract a named cookie value.
 */
function extractCookie(cookieHeader, name) {
  if (!cookieHeader) return null;
  var match = cookieHeader.match(new RegExp('(?:^|;\\s*)' + name + '=([^;]+)'));
  return match ? match[1] : null;
}

/**
 * Parse all Set-Cookie headers from an upstream response.
 * @returns {Map<string, string>}
 */
function parseSetCookies(setCookieHeaders) {
  var cookies = new Map();
  if (!setCookieHeaders) return cookies;
  if (!Array.isArray(setCookieHeaders)) setCookieHeaders = [setCookieHeaders];
  setCookieHeaders.forEach(function (header) {
    var match = header.match(/^([^=]+)=([^;]+)/);
    if (match) {
      cookies.set(match[1].trim(), match[2].trim());
    }
  });
  return cookies;
}

function activeSessionCount() {
  return sessions.size;
}

module.exports = {
  loadSessions,
  flushSessions: persistNow,
  getOrCreateSession,
  getSession,
  destroySession,
  isDefunct,
  DEFUNCT_RETENTION_MS,
  sessionCookieHeader,
  clearSessionCookieHeader,
  cleanupExpiredSessions,
  extractCookie,
  parseSetCookies,
  activeSessionCount,
};
