// web/src/history.js
import { S } from './state.js';
import { openFile } from './tabs.js';

export function pushHistory(path, line) {
  const top = S.hist[S.histIdx];
  if (top && top.path === path && Math.abs(top.line - line) < 2) {
    updateHash(path, line);
    return;
  }
  S.hist = S.hist.slice(0, S.histIdx + 1);
  S.hist.push({ path, line });
  if (S.hist.length > 120) S.hist.shift();
  S.histIdx = S.hist.length - 1;
  updateHash(path, line);
}

// Deep-link surface: #<slash-path>:<line>. A host app reads this hash to
// deep-link the viewer at a specific file/line; the hash always mirrors the
// active tab so a click on a changed file can target it directly.
export function updateHash(path, line) {
  try {
    const next = '#' + path + (line ? ':' + line : '');
    if (location.hash !== next) {
      history.replaceState(null, '', next);
    }
  } catch {}
}

export function readHash() {
  try {
    const raw = decodeURIComponent(location.hash.slice(1));
    if (!raw) return null;
    const sep = raw.lastIndexOf(':');
    if (sep > 0) {
      const line = parseInt(raw.slice(sep + 1), 10);
      return { path: raw.slice(0, sep), line: Number.isFinite(line) && line > 0 ? line : null };
    }
    return { path: raw, line: null };
  } catch {
    return null;
  }
}

export function go(delta) {
  const i = S.histIdx + delta;
  if (i < 0 || i >= S.hist.length) return;
  S.histIdx = i;
  const h = S.hist[i];
  openFile(h.path, { line: h.line, push: false });
}
