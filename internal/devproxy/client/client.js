// Live reload and the error overlay of umono theme dev. The dev proxy adds
// this script to the pages; it is not part of Umono.
(() => {
  const events = new EventSource('/_umono-dev/events');
  let host = null;
  let panel = null;

  // overlay returns the panel of <umono-dev-overlay>, at the end of <body>
  // and outside <umono-root>. Its closed shadow root keeps the theme's CSS
  // out; its only style is the stylesheet below.
  function overlay() {
    if (!host) {
      host = document.createElement('umono-dev-overlay');
      const root = host.attachShadow({ mode: 'closed' });
      const link = document.createElement('link');
      link.rel = 'stylesheet';
      link.href = '/_umono-dev/overlay.css';
      panel = document.createElement('div');
      panel.className = 'panel';
      root.append(link, panel);
    }
    if (!host.isConnected) {
      document.body.append(host);
    }
    return panel;
  }

  // section adds a heading and a list of items; text is never HTML.
  function section(title, kind, items) {
    if (items.length === 0) {
      return;
    }
    const heading = document.createElement('h2');
    heading.className = kind;
    heading.textContent = title;
    const list = document.createElement('ul');
    for (const item of items) {
      const li = document.createElement('li');
      li.textContent = item;
      list.append(li);
    }
    panel.append(heading, list);
  }

  // show shows the errors and warnings of state, or removes the overlay
  // when there are none.
  function show(state) {
    if (state.errors.length === 0 && state.warnings.length === 0) {
      if (host) {
        host.remove();
      }
      return;
    }
    overlay().replaceChildren();
    section('Errors', 'error', state.errors);
    section('Warnings', 'warning', state.warnings);
    const close = document.createElement('button');
    close.type = 'button';
    close.textContent = 'Close';
    close.addEventListener('click', () => host.remove());
    panel.append(close);
  }

  // swapCSS replaces the stylesheet of /_theme/dev/theme.css with a new
  // <link> of the same URL and a query, without reloading the page.
  function swapCSS() {
    for (const link of document.querySelectorAll('link[rel="stylesheet"]')) {
      const url = new URL(link.href, location.href);
      if (url.pathname !== '/_theme/dev/theme.css') {
        continue;
      }
      url.searchParams.set('t', String(Date.now()));
      const next = link.cloneNode();
      next.href = url.pathname + url.search;
      next.addEventListener('load', () => link.remove());
      link.after(next);
    }
  }

  events.addEventListener('state', (e) => show(JSON.parse(e.data)));
  // A connection error is an error event too, without data.
  events.addEventListener('error', (e) => {
    if (e.data) {
      show(JSON.parse(e.data));
    }
  });
  events.addEventListener('css', (e) => {
    swapCSS();
    show(JSON.parse(e.data));
  });
  events.addEventListener('reload', () => location.reload());
})();
