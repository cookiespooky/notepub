(function() {
  if (window.NotepubSearchPage) return;

  var state = {
    form: null,
    input: null,
    list: null,
    summary: null,
    abort: null,
    debounceId: null
  };

  function init() {
    state.form = document.querySelector('.np-search-page-form');
    state.input = document.querySelector('.np-search-page-form input[name="q"]');
    state.list = document.querySelector('.np-search-page-results');
    state.summary = document.querySelector('.np-search-summary');
    if (!state.form || !state.input || !state.list) return;
    state.input.addEventListener('input', onInput);
  }

  function onInput() {
    var q = state.input.value.trim();
    if (state.debounceId) window.clearTimeout(state.debounceId);
    state.debounceId = window.setTimeout(function() {
      search(q);
    }, 200);
  }

  function search(q) {
    updateHistory(q);
    if (q.length < 2) {
      clearResults();
      updateSummary(q, 0);
      return;
    }
    if (state.abort) state.abort.abort();
    state.abort = new AbortController();
    fetch('/v1/search?q=' + encodeURIComponent(q) + '&limit=10', {
      signal: state.abort.signal,
      headers: { 'Accept': 'application/json' }
    })
      .then(function(res) { return res.ok ? res.json() : Promise.reject(res); })
      .then(function(data) {
        renderResults(Array.isArray(data.items) ? data.items : []);
        updateSummary(q, (data.items || []).length);
      })
      .catch(function(err) {
        if (err && err.name === 'AbortError') return;
      });
  }

  function renderResults(items) {
    state.list.innerHTML = '';
    items.forEach(function(item) {
      var li = document.createElement('li');
      var a = document.createElement('a');
      a.href = item.path;
      a.textContent = item.title || item.path;
      li.appendChild(a);
      if (item.snippet) {
        var p = document.createElement('p');
        p.textContent = item.snippet;
        li.appendChild(p);
      }
      state.list.appendChild(li);
    });
  }

  function clearResults() {
    state.list.innerHTML = '';
  }

  function updateSummary(q, count) {
    if (!state.summary) return;
    if (!q) {
      state.summary.textContent = '';
      return;
    }
    state.summary.textContent = 'Запрос: ' + q + ' (' + count + ')';
  }

  function updateHistory(q) {
    if (!window.history || !window.history.replaceState) return;
    var url = q ? '/search?q=' + encodeURIComponent(q) : '/search';
    window.history.replaceState({}, '', url);
  }

  window.NotepubSearchPage = { init: init };
})();
