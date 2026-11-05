// Applies the saved skin and colour mode before the page is drawn, so it
// never flashes in the wrong theme, and sets <html lang>. A plain script (not
// a module) loaded in <head>; it mirrors src/core/appearance.ts,
// src/lib/local-themes.ts and src/i18n/locales.ts — keep them in step.
(function () {
  var root = document.documentElement;
  var skin = 'carnet';
  var mode = 'system';
  // ?skin=<id> is for this page load only: it is never stored.
  var s = new URLSearchParams(location.search).get('skin');
  try {
    if (!s) s = localStorage.getItem('solstice.skin');
    if (s && /^[a-z0-9-]{1,38}$/.test(s)) skin = s;
    var m = localStorage.getItem('solstice.theme');
    if (m === 'light' || m === 'dark') mode = m;
  } catch {
    // Storage unavailable (private browsing): defaults.
  }
  var dark =
    mode === 'dark' ||
    (mode === 'system' &&
      !!window.matchMedia &&
      window.matchMedia('(prefers-color-scheme: dark)').matches);
  // The interface language, as src/i18n/locales.ts negotiates it, so
  // :lang() rules (Devanagari typography) hold from the first paint.
  var locales = ['en', 'fr', 'de', 'it', 'hi'];
  var lang = 'en';
  var saved = null;
  try {
    saved = localStorage.getItem('solstice.locale');
  } catch {
    // Storage unavailable: the browser's languages decide.
  }
  if (locales.indexOf(saved) >= 0) {
    lang = saved;
  } else {
    var nav = typeof navigator !== 'undefined' ? navigator : {};
    var wanted = nav.languages || [nav.language || ''];
    for (var i = 0; i < wanted.length; i++) {
      var base = String(wanted[i]).split(/[-_]/)[0].toLowerCase();
      if (locales.indexOf(base) >= 0) {
        lang = base;
        break;
      }
    }
  }
  root.lang = lang;
  root.setAttribute('data-skin', skin);
  if (mode !== 'system') root.setAttribute('data-theme', mode);
  root.setAttribute('data-scheme', dark ? 'dark' : 'light');
  // The page stays blank until the skin's stylesheet is in (src/lib/ui.svelte.ts);
  // the timer only guards against the application never starting, so it outlasts
  // the application's own timeout.
  root.setAttribute('data-skin-loading', '');
  setTimeout(function () {
    root.removeAttribute('data-skin-loading');
  }, 10000);
  // A theme kept in this browser: its stylesheet, null when it isn't here.
  function localCss(id) {
    return new Promise(function (resolve) {
      var req = indexedDB.open('solstice-themes', 1);
      req.onupgradeneeded = function () {
        req.result.createObjectStore('themes', { keyPath: 'id' });
      };
      req.onerror = function () {
        resolve(null);
      };
      req.onsuccess = function () {
        var db = req.result;
        var get = db.transaction('themes').objectStore('themes').get(id);
        get.onerror = function () {
          db.close();
          resolve(null);
        };
        get.onsuccess = function () {
          db.close();
          resolve(get.result ? get.result.css : null);
        };
      };
    }).catch(function () {
      return null;
    });
  }
  // Started here, so the stylesheet doesn't wait for the application's scripts.
  window.solsticeBootSkin = {
    id: skin,
    css:
      skin.indexOf('local-') === 0
        ? localCss(skin)
        : fetch('/api/themes/' + skin + '/theme.css', { credentials: 'same-origin' })
            .then(function (res) {
              return res.ok ? res.text() : null;
            })
            .catch(function () {
              return null;
            }),
  };
})();
