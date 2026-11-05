// Explainer timeline. Every visual is a pure function of
// the clock, so pause, seek, scrub, and chapters always agree.
(function () {
  'use strict';

  var doc = document;
  doc.documentElement.classList.add('js');

  var reduced = window.matchMedia &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  function $(id) { return doc.getElementById(id); }
  function on(el, yes) {
    if (el) el.classList.toggle('on', !!yes);
  }
  function clamp01(x) { return x < 0 ? 0 : x > 1 ? 1 : x; }
  function easeOutCubic(x) {
    var u = clamp01(x);
    return 1 - Math.pow(1 - u, 3);
  }
  // Reduced motion snaps phases; standard mode uses cubic ease-out.
  function prog(t, at, span) {
    if (reduced) return t >= at ? 1 : 0;
    return easeOutCubic((t - at) / span);
  }

  // ---- scenes (durations and captions live in the HTML) ----
  var sections = Array.prototype.slice.call(
    doc.querySelectorAll('.scene'));
  var scenes = sections.map(function (sec, i) {
    var caps = Array.prototype.slice.call(
      sec.querySelectorAll('.script li')).map(function (li) {
      return {
        t: parseFloat(li.getAttribute('data-t') || '0'),
        text: li.textContent
      };
    });
    return {
      root: sec,
      title: sec.getAttribute('data-title') || 'Scene ' + (i + 1),
      dur: parseFloat(sec.getAttribute('data-dur') || '10'),
      caps: caps,
      start: 0
    };
  });
  var total = 0;
  scenes.forEach(function (s) { s.start = total; total += s.dur; });

  // ---- per-scene ticks: each a function of local time ----
  var CPS = 16; // terminal characters per second
  var S2LINES = [
    { id: 's2l1', at: 0, y: 110, text: '$ just build' },
    { id: 's2l2', at: 5, y: 170, text: '$ ./fs-relay -data data -token local-demo' },
    { id: 's2l3', at: 10, y: 230, text: '$ ./fs-relay-bind -ns demo -token local-demo /mnt/x' },
    { id: 's2l4', at: 15, y: 290, text: 'mounted on /mnt/x, namespace demo' }
  ];

  function tickS2(t) {
    var ns = S2LINES.map(function (L) {
      var n;
      if (reduced) n = t >= L.at ? L.text.length : 0;
      else n = Math.floor((t - L.at) * CPS);
      return Math.max(0, Math.min(L.text.length, n));
    });
    var last = 0;
    ns.forEach(function (n, i) { if (n > 0) last = i; });
    S2LINES.forEach(function (L, i) {
      var el = $(L.id);
      if (!el) return;
      var s = L.text.slice(0, ns[i]);
      if (el.textContent !== s) el.textContent = s;
    });
    var caret = $('s2caret');
    var activeLine = $(S2LINES[last].id);
    if (caret && activeLine) {
      var len = 0;
      try {
        len = activeLine.getComputedTextLength ? activeLine.getComputedTextLength() : ns[last] * 8.4;
      } catch (_) {
        len = ns[last] * 8.4;
      }
      caret.setAttribute('x', String(42 + len));
      caret.setAttribute('y', String(S2LINES[last].y - 13));
    }
    on($('s2s1'), t >= 2.5);
    on($('s2s2'), t >= 8.5);
    on($('s2s3'), t >= 13.5);
    on($('s2s4'), t >= 16.5);
  }

  var S5BARS = [
    { b: 's5b0', r: 's5r0', at: 0.4 },  // local floor
    { b: 's5b1', r: 's5r1', at: 10.5 }, // fs-relay
    { b: 's5b2', r: 's5r2', at: 11.4 }, // fs-relay -post
    { b: 's5b3', r: 's5r3', at: 6.2 },  // kvfs
    { b: 's5b4', r: 's5r4', at: 5.2 },  // fuse-nfs
    { b: 's5b5', r: 's5r5', at: 0.8 }   // s3fs
  ];

  function tickS5(t) {
    S5BARS.forEach(function (B) {
      var p = prog(t, B.at, 0.9);
      var row = $(B.r), bar = $(B.b);
      on(row, p > 0);
      if (bar) bar.style.transform = 'scaleX(' + Math.max(0.02, p) + ')';
    });
    on($('s5verdict'), t >= 13.5);
  }

  var TICKS = {
    sc1: function (t) { on($('s1-clean'), t >= 8); },
    sc2: tickS2,
    sc4: function (t) {
      on($('s4zl'), t < 10);
      on($('s4zr'), t >= 10);
    },
    sc5: tickS5,
    sc6: function (t) {
      on($('s6cache'), t >= 6 && t < 10);
      on($('s6a'), t >= 11);
      on($('s6b'), t >= 11);
    },
    sc7: function (t) {
      on($('s7a'), t < 10);
      on($('s7b'), t >= 5 && t < 15);
      on($('s7r'), t >= 10 && t < 15);
      on($('s7g'), t >= 15);
      on($('s7pulse'), t >= 10);
      var h = $('s7holder');
      var s = t < 15 ? 'holder: A' : 'holder: B';
      if (h && h.textContent !== s) h.textContent = s;
    },
    sc8: function (t) { on($('s8close'), t >= 15); }
  };

  // ---- transport ----
  var stage = $('stage');
  var caption = $('caption');
  var fill = $('fill');
  var clock = $('clock');
  var progress = $('progress');
  var playBtn = $('btn-play');
  var chaptersEl = $('chapters');

  var t = 0;
  var playing = false;
  var lastTs = 0;
  var activeIdx = -1;

  scenes.forEach(function (s, i) {
    var b = doc.createElement('button');
    b.type = 'button';
    b.textContent = (i + 1) + ' · ' + s.title;
    b.setAttribute('aria-label', 'Go to scene ' + (i + 1) +
      ': ' + s.title);
    b.addEventListener('click', function () {
      t = s.start;
      render();
    });
    chaptersEl.appendChild(b);
  });
  progress.setAttribute('aria-valuemax', String(Math.round(total)));

  function fmt(s) {
    var m = Math.floor(s / 60);
    var r = Math.floor(s % 60);
    return m + ':' + (r < 10 ? '0' : '') + r;
  }

  function sceneAt(time) {
    for (var i = scenes.length - 1; i >= 0; i--) {
      if (time >= scenes[i].start) return i;
    }
    return 0;
  }

  function activate(i) {
    if (i === activeIdx) return;
    scenes.forEach(function (s, j) {
      s.root.classList.toggle('active', j === i);
    });
    activeIdx = i;
    var btns = chaptersEl.children;
    for (var k = 0; k < btns.length; k++) {
      btns[k].classList.toggle('here', k === i);
    }
  }

  function render() {
    var i = sceneAt(Math.min(t, total - 0.001));
    activate(i);
    var local = t - scenes[i].start;
    var tick = TICKS[scenes[i].root.id];
    if (tick) tick(local);
    var line = '';
    scenes[i].caps.forEach(function (c) {
      if (local >= c.t) line = c.text;
    });
    if (caption.textContent !== line) caption.textContent = line;
    var pct = (100 * t / total);
    fill.style.width = pct + '%';
    progress.style.setProperty('--seek-pct', pct + '%');
    progress.setAttribute('aria-valuenow', String(Math.round(t)));
    progress.setAttribute('aria-valuetext', fmt(t) + ' of ' + fmt(total));
    clock.textContent = fmt(t) + ' / ' + fmt(total);
  }

  function frame(ts) {
    if (!playing) return;
    if (lastTs) t = Math.min(total, t + (ts - lastTs) / 1000);
    lastTs = ts;
    render();
    if (t >= total) {
      setPlaying(false);
      return;
    }
    requestAnimationFrame(frame);
  }

  function setPlaying(p) {
    if (p && t >= total) t = 0;
    playing = p;
    lastTs = 0;
    doc.body.classList.toggle('paused', !p);
    playBtn.textContent = p ? 'Pause' : 'Play';
    playBtn.setAttribute('aria-label', p ? 'Pause' : 'Play');
    if (p) requestAnimationFrame(frame);
    else render();
  }

  playBtn.addEventListener('click', function () {
    setPlaying(!playing);
  });
  if (stage) {
    stage.addEventListener('click', function () {
      setPlaying(!playing);
    });
  }
  $('btn-restart').addEventListener('click', function () {
    t = 0;
    setPlaying(true);
  });
  $('btn-prev').addEventListener('click', function () {
    var i = sceneAt(t);
    var target = (t - scenes[i].start > 1.5) ? i : Math.max(0, i - 1);
    t = scenes[target].start;
    render();
  });
  $('btn-next').addEventListener('click', function () {
    var i = sceneAt(Math.min(t, total - 0.001));
    t = i + 1 < scenes.length ? scenes[i + 1].start : total;
    render();
  });

  // Pointer-capture scrubbing
  var scrubbing = false;
  function seekAtClientX(clientX) {
    var r = progress.getBoundingClientRect();
    if (!r.width) return;
    var x = clientX - r.left;
    t = Math.max(0, Math.min(total, total * x / r.width));
    render();
  }

  progress.addEventListener('pointerdown', function (ev) {
    scrubbing = true;
    progress.classList.add('scrubbing');
    if (progress.setPointerCapture) {
      try { progress.setPointerCapture(ev.pointerId); } catch (_) {}
    }
    seekAtClientX(ev.clientX);
  });
  progress.addEventListener('pointermove', function (ev) {
    if (!scrubbing) return;
    seekAtClientX(ev.clientX);
  });
  function stopScrub(ev) {
    if (!scrubbing) return;
    scrubbing = false;
    progress.classList.remove('scrubbing');
    if (progress.releasePointerCapture && ev && ev.pointerId != null) {
      try { progress.releasePointerCapture(ev.pointerId); } catch (_) {}
    }
  }
  progress.addEventListener('pointerup', stopScrub);
  progress.addEventListener('pointercancel', stopScrub);

  progress.addEventListener('keydown', function (ev) {
    if (ev.key === 'ArrowLeft') {
      t = Math.max(0, t - 5);
      render();
      ev.preventDefault();
    } else if (ev.key === 'ArrowRight') {
      t = Math.min(total, t + 5);
      render();
      ev.preventDefault();
    } else if (ev.key === 'Home') {
      t = 0;
      render();
      ev.preventDefault();
    } else if (ev.key === 'End') {
      t = total;
      render();
      ev.preventDefault();
    }
  });

  // Global keyboard shortcuts when not inside a button/link/input
  doc.addEventListener('keydown', function (ev) {
    var tag = ev.target && ev.target.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || ev.metaKey || ev.ctrlKey || ev.altKey) return;
    if (ev.key === 'k' || (ev.key === ' ' && tag !== 'BUTTON' && tag !== 'A')) {
      setPlaying(!playing);
      ev.preventDefault();
    } else if (ev.key === 'j') {
      var i = sceneAt(t);
      t = scenes[Math.max(0, i - 1)].start;
      render();
    } else if (ev.key === 'l') {
      var idx = sceneAt(Math.min(t, total - 0.001));
      t = idx + 1 < scenes.length ? scenes[idx + 1].start : total;
      render();
    }
  });

  // Pause timer when tab is hidden (Sonner rule 4)
  doc.addEventListener('visibilitychange', function () {
    if (doc.hidden) lastTs = 0;
  });

  // Deep link: #t=seconds starts paused at that moment.
  var hash = window.location.hash.match(/^#t=(\d+(?:\.\d+)?)$/);
  if (hash) t = Math.max(0, Math.min(total, parseFloat(hash[1])));

  // Initial frame, paused.
  doc.body.classList.add('paused');
  render();
})();
