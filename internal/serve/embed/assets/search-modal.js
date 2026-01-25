(function() {
  if (window.NotepubSearchModal) return;

  var state = {
    inited: false,
    endpoint: '/v1/search',
    dialog: null,
    input: null,
    results: null,
    status: null,
    allLink: null,
    backdrop: null,
    items: [],
    selected: -1,
    debounceId: null
  };

  function init(opts) {
    if (state.inited) return;
    state.inited = true;
    if (opts && opts.endpoint) state.endpoint = opts.endpoint;
    state.dialog = opts && opts.mount ? opts.mount : document.getElementById('np-search-dialog');
    if (!state.dialog) return;

    state.input = state.dialog.querySelector('input[name="q"]');
    state.results = state.dialog.querySelector('.np-search-results');
    state.status = state.dialog.querySelector('.np-search-status');
    state.allLink = state.dialog.querySelector('.np-search-all');
    state.backdrop = document.querySelector('.np-search-backdrop');

    var closeBtn = state.dialog.querySelector('[data-action="close"]');
    if (closeBtn) closeBtn.addEventListener('click', close);
    if (state.backdrop) state.backdrop.addEventListener('click', close);
    state.dialog.addEventListener('cancel', function(e) {
      e.preventDefault();
      close();
    });
    state.dialog.addEventListener('click', function(e) {
      if (e.target === state.dialog) close();
    });

    if (state.input) {
      state.input.addEventListener('input', onInput);
      state.input.addEventListener('keydown', onKeyDown);
    }
  }

  function open() {
    if (!state.dialog) return;
    if (state.dialog.showModal) {
      state.dialog.showModal();
    } else {
      state.dialog.classList.add('is-open');
    }
    if (state.backdrop) state.backdrop.hidden = false;
    if (state.input) state.input.focus();
  }

  function close() {
    if (!state.dialog) return;
    if (state.dialog.open && state.dialog.close) {
      state.dialog.close();
    } else {
      state.dialog.classList.remove('is-open');
    }
    if (state.backdrop) state.backdrop.hidden = true;
  }

  function onInput() {
    if (!state.input) return;
    var q = state.input.value.trim();
    if (state.allLink) state.allLink.href = q ? '/search?q=' + encodeURIComponent(q) : '/search';
    if (q.length < 2) {
      clearResults();
      setStatus('');
      return;
    }
    if (state.debounceId) window.clearTimeout(state.debounceId);
    state.debounceId = window.setTimeout(function() {
      search(q);
    }, 200);
  }

  function search(q) {
    setStatus('Loading...');
    fetch(state.endpoint + '?q=' + encodeURIComponent(q) + '&limit=8', {
      headers: { 'Accept': 'application/json' }
    })
      .then(function(res) { return res.ok ? res.json() : Promise.reject(res); })
      .then(function(data) {
        state.items = Array.isArray(data.items) ? data.items : [];
        state.selected = -1;
        renderResults();
        setStatus(state.items.length ? '' : 'No results');
      })
      .catch(function() {
        setStatus('Error loading results');
        clearResults();
      });
  }

  function onKeyDown(e) {
    if (!state.items.length) return;
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      state.selected = Math.min(state.items.length - 1, state.selected + 1);
      renderResults();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      state.selected = Math.max(0, state.selected - 1);
      renderResults();
    } else if (e.key === 'Enter') {
      if (state.selected >= 0 && state.items[state.selected]) {
        e.preventDefault();
        window.location.href = state.items[state.selected].path;
      }
    } else if (e.key === 'Escape') {
      e.preventDefault();
      close();
    }
  }

  function renderResults() {
    if (!state.results) return;
    state.results.innerHTML = '';
    state.items.forEach(function(item, idx) {
      var row = document.createElement('div');
      row.className = 'np-search-item' + (idx === state.selected ? ' is-selected' : '');
      row.setAttribute('role', 'option');
      row.setAttribute('aria-selected', idx === state.selected ? 'true' : 'false');

      var title = document.createElement('div');
      title.className = 'np-search-item-title';
      title.textContent = item.title || item.path;

      var snippet = document.createElement('div');
      snippet.className = 'np-search-item-snippet';
      snippet.textContent = item.snippet || '';

      row.appendChild(title);
      row.appendChild(snippet);
      row.addEventListener('click', function() {
        window.location.href = item.path;
      });
      state.results.appendChild(row);
    });
  }

  function clearResults() {
    state.items = [];
    state.selected = -1;
    if (state.results) state.results.innerHTML = '';
  }

  function setStatus(text) {
    if (state.status) state.status.textContent = text;
  }

  window.NotepubSearchModal = {
    init: init,
    open: open,
    close: close
  };
})();
