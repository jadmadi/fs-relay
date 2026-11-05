// Progressive-enhancement UI helpers for fs-relay html-prototypes.
// Without JS, every page renders all text, tables, and SVG diagrams in full.
(function () {
  'use strict';

  var doc = document;
  doc.documentElement.classList.add('js');

  var ICON_COPY =
    '<svg viewBox="0 0 24 24" aria-hidden="true">' +
    '<rect x="9" y="9" width="13" height="13" rx="2"/>' +
    '<path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>' +
    '</svg><span>Copy</span>';

  var ICON_CHECK =
    '<svg viewBox="0 0 24 24" aria-hidden="true">' +
    '<polyline points="20 6 9 17 4 12"/>' +
    '</svg><span>Copied</span>';

  // ---- 1. Mobile nav active tab centering & scroll mask ----
  var nav = doc.querySelector('nav');
  if (nav) {
    var here = nav.querySelector('a.here');
    var centerHere = function () {
      if (here && nav.scrollWidth > nav.clientWidth) {
        var targetLeft = here.offsetLeft - (nav.clientWidth - here.offsetWidth) / 2;
        nav.scrollLeft = Math.max(0, targetLeft);
      }
    };
    var updateNavMask = function () {
      var atEnd = nav.scrollLeft + nav.clientWidth >= nav.scrollWidth - 6;
      nav.classList.toggle('scrolled-end', atEnd);
    };
    nav.addEventListener('scroll', updateNavMask, { passive: true });
    window.addEventListener('resize', function () {
      centerHere();
      updateNavMask();
    }, { passive: true });
    centerHere();
    updateNavMask();
  }

  // ---- 2. One-click copy buttons on <pre> blocks ----
  function copyText(text, done) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done, function () {
        fallbackCopy(text, done);
      });
      return;
    }
    fallbackCopy(text, done);
  }

  function fallbackCopy(text, done) {
    var ta = doc.createElement('textarea');
    ta.value = text;
    ta.setAttribute('readonly', '');
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    doc.body.appendChild(ta);
    ta.select();
    try { doc.execCommand('copy'); } catch (_) {}
    doc.body.removeChild(ta);
    done();
  }

  var pres = Array.prototype.slice.call(doc.querySelectorAll('main pre'));
  pres.forEach(function (pre) {
    var rawText = (pre.textContent || '').trim();
    if (!rawText) return;
    pre.classList.add('has-copy');
    var btn = doc.createElement('button');
    btn.type = 'button';
    btn.className = 'copy-btn';
    btn.setAttribute('aria-label', 'Copy code snippet');
    btn.innerHTML = ICON_COPY;
    var timer = 0;
    btn.addEventListener('click', function () {
      copyText(rawText, function () {
        btn.classList.add('copied');
        btn.innerHTML = ICON_CHECK;
        clearTimeout(timer);
        timer = setTimeout(function () {
          btn.classList.remove('copied');
          btn.innerHTML = ICON_COPY;
        }, 1800);
      });
    });
    pre.appendChild(btn);
  });

  // ---- 3. Interactive smoke checklists (run.html) ----
  var checklists = Array.prototype.slice.call(
    doc.querySelectorAll('ol[data-checklist]'));
  checklists.forEach(function (ol) {
    var key = 'fsrelay-check-' + ol.getAttribute('data-checklist');
    var saved = {};
    try {
      saved = JSON.parse(sessionStorage.getItem(key) || '{}') || {};
    } catch (_) {}

    ol.classList.add('checklist-interactive');
    var items = Array.prototype.slice.call(ol.querySelectorAll('li'));
    var badge = doc.querySelector('[data-checklist-badge="' +
      ol.getAttribute('data-checklist') + '"]');

    function syncBadge() {
      if (!badge) return;
      var doneCount = items.filter(function (li) {
        return li.classList.contains('done');
      }).length;
      badge.textContent = doneCount + ' / ' + items.length + ' verified';
    }

    items.forEach(function (li, idx) {
      var cb = doc.createElement('input');
      cb.type = 'checkbox';
      cb.checked = !!saved[idx];
      cb.setAttribute('aria-label', 'Step ' + (idx + 1));
      var wrap = doc.createElement('span');
      while (li.firstChild) {
        wrap.appendChild(li.firstChild);
      }
      li.appendChild(cb);
      li.appendChild(wrap);
      if (cb.checked) li.classList.add('done');

      function toggle(fromBox) {
        if (!fromBox) cb.checked = !cb.checked;
        li.classList.toggle('done', cb.checked);
        saved[idx] = cb.checked;
        try { sessionStorage.setItem(key, JSON.stringify(saved)); } catch (_) {}
        syncBadge();
      }

      cb.addEventListener('click', function (ev) {
        ev.stopPropagation();
        toggle(true);
      });
      li.addEventListener('click', function () {
        toggle(false);
      });
    });

    syncBadge();
  });

  // ---- 4. Benchmark chart sorting and comparison filter (compare.html) ----
  var charts = Array.prototype.slice.call(doc.querySelectorAll('.chart'));
  if (charts.length > 1) {
    charts.forEach(function (chart) {
      var rows = Array.prototype.slice.call(chart.querySelectorAll('.row'));
      rows.forEach(function (row, idx) {
        row.setAttribute('data-orig-idx', String(idx));
        var valEl = row.querySelector('.val');
        var m = valEl && valEl.textContent.match(/(\d+(?:\.\d+)?)\s*s/);
        row.setAttribute('data-sec', m ? m[1] : '9999');
        var who = (row.querySelector('.who') || {}).textContent || '';
        row.setAttribute('data-who', who.trim().toLowerCase());
      });
    });

    function sortCharts(mode) {
      charts.forEach(function (chart) {
        var rows = Array.prototype.slice.call(chart.querySelectorAll('.row'));
        rows.sort(function (a, b) {
          if (mode === 'fastest') {
            return parseFloat(a.getAttribute('data-sec')) -
              parseFloat(b.getAttribute('data-sec'));
          }
          return parseInt(a.getAttribute('data-orig-idx'), 10) -
            parseInt(b.getAttribute('data-orig-idx'), 10);
        });
        rows.forEach(function (r) { chart.appendChild(r); });
      });
    }

    function filterCharts(target) {
      charts.forEach(function (chart) {
        var rows = Array.prototype.slice.call(chart.querySelectorAll('.row'));
        rows.forEach(function (row) {
          if (target === 'all') {
            row.classList.remove('is-dimmed');
            return;
          }
          var who = row.getAttribute('data-who') || '';
          var keep = who.indexOf('fs-relay') !== -1 || who.indexOf(target) !== -1;
          row.classList.toggle('is-dimmed', !keep);
        });
      });
    }

    var ctrlHost = doc.getElementById('compare-controls');
    if (ctrlHost) {
      var sortBtns = Array.prototype.slice.call(
        ctrlHost.querySelectorAll('[data-sort]'));
      sortBtns.forEach(function (btn) {
        btn.addEventListener('click', function () {
          var mode = btn.getAttribute('data-sort');
          sortBtns.forEach(function (b) {
            b.setAttribute('aria-pressed', String(b === btn));
          });
          sortCharts(mode);
        });
      });

      var filterBtns = Array.prototype.slice.call(
        ctrlHost.querySelectorAll('[data-filter]'));
      filterBtns.forEach(function (btn) {
        btn.addEventListener('click', function () {
          var target = btn.getAttribute('data-filter');
          filterBtns.forEach(function (b) {
            b.setAttribute('aria-pressed', String(b === btn));
          });
          filterCharts(target);
        });
      });
    }
  }

  // ---- 5. Interactive diagram step-through explorers ----
  var explorers = Array.prototype.slice.call(
    doc.querySelectorAll('figure.diagram[data-explorer]'));
  explorers.forEach(function (fig) {
    var steps = Array.prototype.slice.call(
      fig.querySelectorAll('.explorer-steps [data-step]'));
    var status = fig.querySelector('.explorer-status');
    var groups = Array.prototype.slice.call(
      fig.querySelectorAll('svg [data-role]'));

    function applyStep(stepId, desc) {
      steps.forEach(function (btn) {
        var active = btn.getAttribute('data-step') === stepId;
        btn.setAttribute('aria-pressed', String(active));
      });
      groups.forEach(function (el) {
        var roles = (el.getAttribute('data-role') || '').split(/\s+/);
        var match = stepId === 'all' || roles.indexOf(stepId) !== -1;
        el.classList.toggle('is-dim', !match);
        el.classList.toggle('is-focus', stepId !== 'all' && match &&
          (el.classList.contains('box') || el.classList.contains('box-hi') ||
           el.classList.contains('chip') || el.classList.contains('chip-hi') ||
           el.classList.contains('wire')));
      });
      if (status && desc) status.textContent = desc;
    }

    steps.forEach(function (btn) {
      btn.addEventListener('click', function () {
        applyStep(
          btn.getAttribute('data-step'),
          btn.getAttribute('data-desc')
        );
      });
    });
  });

  // ---- 6. Raw path interactive byte-size router (raw.html) ----
  var rawFig = doc.querySelector('figure.diagram[data-raw-router]');
  if (rawFig) {
    var sizeInput = rawFig.querySelector('#raw-size-slider');
    var sizeLabel = rawFig.querySelector('#raw-size-val');
    var opBtn = rawFig.querySelector('#raw-op-btn');
    var pipeBtn = rawFig.querySelector('#raw-pipe-btn');
    var statusEl = rawFig.querySelector('.explorer-status');
    var nodes = Array.prototype.slice.call(
      rawFig.querySelectorAll('svg [data-route]'));

    var isWrite = false;
    var fitsPipe = true;

    function updateRawRouter() {
      var kb = parseInt(sizeInput ? sizeInput.value : '256', 10);
      if (sizeLabel) {
        sizeLabel.textContent = kb >= 1024 ? '1 MiB' : kb + ' KiB';
      }
      var inWindow = kb >= 128 && kb <= 960;
      var activeRoute = 'env';
      var msg = '';
      if (!inWindow) {
        activeRoute = 'env';
        msg = kb + ' KiB is outside [Min 128 KiB, Max 960 KiB] → stays on FlatBuffers envelope.';
      } else if (isWrite) {
        activeRoute = 'raw-write';
        msg = kb + ' KiB write is inside [Min, Max] → writev header + bytes on raw socket, server splices socket → pipe → file.';
      } else if (fitsPipe) {
        activeRoute = 'raw-pipe';
        msg = kb + ' KiB read fits in pipe → server splices file → pipe → socket; mount splices socket → pipe → /dev/fuse (zero copy).';
      } else {
        activeRoute = 'raw-buf';
        msg = kb + ' KiB read exceeds pipe capacity → raw read into FUSE buffer.';
      }
      nodes.forEach(function (el) {
        var tags = (el.getAttribute('data-route') || '').split(/\s+/);
        var match = tags.indexOf('always') !== -1 || tags.indexOf(activeRoute) !== -1;
        el.classList.toggle('is-dim', !match);
        el.classList.toggle('is-focus', tags.indexOf(activeRoute) !== -1 &&
          tags.indexOf('outcome') !== -1);
      });
      if (statusEl) statusEl.textContent = msg;
    }

    if (sizeInput) sizeInput.addEventListener('input', updateRawRouter);
    if (opBtn) {
      opBtn.addEventListener('click', function () {
        isWrite = !isWrite;
        opBtn.textContent = isWrite ? 'Op: write' : 'Op: read';
        opBtn.setAttribute('aria-pressed', String(isWrite));
        updateRawRouter();
      });
    }
    if (pipeBtn) {
      pipeBtn.addEventListener('click', function () {
        fitsPipe = !fitsPipe;
        pipeBtn.textContent = fitsPipe ? 'Pipe: fits' : 'Pipe: full';
        pipeBtn.setAttribute('aria-pressed', String(!fitsPipe));
        updateRawRouter();
      });
    }
    updateRawRouter();
  }
})();
