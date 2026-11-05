// Explainer cinema timeline. Every visual and camera move is a pure
// function of the clock, so pause, seek, scrub, speed, and chapters agree.
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
  function boxHi(el, yes) {
    if (!el) return;
    el.setAttribute('class', yes ? 'box-hi' : 'box');
  }
  function rowHi(el, yes) {
    if (el) el.classList.toggle('row-hi', !!yes);
  }
  function clamp01(x) { return x < 0 ? 0 : x > 1 ? 1 : x; }
  function easeOutCubic(x) {
    var u = clamp01(x);
    return 1 - Math.pow(1 - u, 3);
  }
  function prog(t, at, span) {
    if (reduced) return t >= at ? 1 : 0;
    return easeOutCubic((t - at) / span);
  }
  function setCam(id, tx, ty, sc) {
    var g = $(id);
    if (!g) return;
    if (reduced) {
      g.style.transform = 'none';
      return;
    }
    g.style.transform = 'translate(' + tx + 'px,' + ty + 'px) scale(' + sc + ')';
  }

  // ---- optional Web Audio API synth (muted by default) ----
  var audioOn = false;
  var audioCtx = null;
  var lastKeyTick = 0;

  function ensureAudio() {
    if (!audioCtx) {
      var Ctx = window.AudioContext || window.webkitAudioContext;
      if (Ctx) audioCtx = new Ctx();
    }
    if (audioCtx && audioCtx.state === 'suspended') {
      try { audioCtx.resume(); } catch (_) {}
    }
  }

  function playTone(freq, dur, gainVal, type) {
    if (!audioOn || !playing || scrubbing || reduced) return;
    ensureAudio();
    if (!audioCtx) return;
    try {
      var now = audioCtx.currentTime;
      var osc = audioCtx.createOscillator();
      var g = audioCtx.createGain();
      osc.type = type || 'sine';
      osc.frequency.setValueAtTime(freq, now);
      g.gain.setValueAtTime(0.0001, now);
      g.gain.exponentialRampToValueAtTime(gainVal || 0.025, now + 0.01);
      g.gain.exponentialRampToValueAtTime(0.0001, now + dur);
      osc.connect(g);
      g.connect(audioCtx.destination);
      osc.start(now);
      osc.stop(now + dur + 0.01);
    } catch (_) {}
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

  // ---- per-scene choreography ----
  function tickS1(t) {
    on($('s1-legacy'), t < 8 || t >= 12);
    on($('s1-tangle'), t >= 3.2);
    on($('s1-clean'), t >= 8);
    on($('s1-hero'), t >= 11.8);
    var mult = $('s1-mult');
    if (mult) {
      var p = prog(t, 12, 1.4);
      var n = Math.max(1, Math.round(1 + 9 * p));
      var txt = '≈' + n + '× faster';
      if (mult.textContent !== txt) mult.textContent = txt;
    }
    if (t < 7.8) setCam('cam1', 42, 6, 1.08);
    else if (t < 12) setCam('cam1', -46, 6, 1.08);
    else setCam('cam1', 0, 0, 1);
  }

  var CPS = 18;
  var S2LINES = [
    { id: 's2l1', at: 0.2, y: 102, text: '$ just build' },
    { id: 's2l2', at: 5.0, y: 166, text: '$ ./fs-relay -data data -token local-demo' },
    { id: 's2l3', at: 10.0, y: 230, text: '$ ./fs-relay-bind -ns demo -token local-demo /mnt/x' },
    { id: 's2l4', at: 15.0, y: 298, text: 'mounted on /mnt/x, namespace demo' }
  ];

  function tickS2(t) {
    var totalChars = 0;
    var ns = S2LINES.map(function (L) {
      var n = reduced ? (t >= L.at ? L.text.length : 0) : Math.floor((t - L.at) * CPS);
      var clamped = Math.max(0, Math.min(L.text.length, n));
      totalChars += clamped;
      return clamped;
    });
    if (totalChars !== lastKeyTick) {
      if (totalChars > lastKeyTick) playTone(780, 0.018, 0.012, 'triangle');
      lastKeyTick = totalChars;
    }
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
        len = activeLine.getComputedTextLength ? activeLine.getComputedTextLength() : ns[last] * 8.1;
      } catch (_) {
        len = ns[last] * 8.1;
      }
      caret.setAttribute('x', String(42 + len));
      caret.setAttribute('y', String(S2LINES[last].y - 13));
    }
    on($('s2o1'), t >= 1.8);
    on($('s2o2'), t >= 7.8);
    on($('s2o3'), t >= 13.4);
    on($('s2s1'), t >= 1.5);
    on($('s2s2'), t >= 7.2);
    on($('s2s3'), t >= 12.8);
    on($('s2s4'), t >= 16.0);
    boxHi($('s2b1'), t >= 1.5 && t < 7.2);
    boxHi($('s2b2'), t >= 7.2 && t < 12.8);
    boxHi($('s2b3'), t >= 12.8 && t < 16.0);
    if (t < 5) setCam('cam2', 12, 18, 1.04);
    else if (t < 15) setCam('cam2', 0, 0, 1.03);
    else setCam('cam2', -10, -10, 1.04);
  }

  function tickS3(t) {
    on($('s3-mounts'), true);
    on($('s3-disk'), t >= 15.5);
    boxHi($('s3bA'), t < 6);
    boxHi($('s3bB'), t < 6);
    boxHi($('s3bDisk'), t >= 16);
    rowHi($('s3r-http'), t >= 6 && t < 11);
    rowHi($('s3r-ev'), t >= 11 && t < 16);
    rowHi($('s3r-lk'), t >= 11 && t < 16);
    rowHi($('s3r-raw'), t >= 16);
    var st = $('s3-status');
    var msg = t < 6
      ? '1 persistent socket per mount → multiplexed RPC'
      : t < 11
      ? 'call id=42 lookup("db") ↑ · result ino=9 ↓'
      : t < 16
      ? 'async push: evict & lock release share the socket'
      : 'payload ≥ 64 KiB → dedicated zero-copy raw socket';
    if (st && st.textContent !== msg) st.textContent = msg;
    if (t < 6) setCam('cam3', 32, 8, 1.05);
    else if (t < 16) setCam('cam3', 0, 4, 1.06);
    else setCam('cam3', -34, 4, 1.05);
  }

  function tickS4(t) {
    on($('s4-left'), t < 10 || t >= 14);
    on($('s4-right'), t >= 10);
    on($('s4zl'), t < 10 || t >= 14);
    on($('s4zr'), t >= 10);
    on($('s4-both'), t >= 14);
    var lbl = $('s4-ws-lbl');
    var txt = (t >= 6 && t < 10) ? 'call id=42 ↔ result id=42' : 'call id=N';
    if (lbl && lbl.textContent !== txt) lbl.textContent = txt;
    if (t < 10) setCam('cam4', 55, 8, 1.08);
    else if (t < 14) setCam('cam4', -55, 8, 1.08);
    else setCam('cam4', 0, 0, 1);
  }

  var S5BARS = [
    { b: 's5b0', r: 's5r0', v: 's5v0', at: 0.4, num: 0.14, suffix: ' s' },
    { b: 's5b5', r: 's5r5', v: 's5v5', at: 0.8, num: 203.56, suffix: ' s' },
    { b: 's5b4', r: 's5r4', v: 's5v4', at: 5.2, num: 107.89, suffix: ' s' },
    { b: 's5b3', r: 's5r3', v: 's5v3', at: 6.2, num: 25.76, suffix: ' s' },
    { b: 's5b1', r: 's5r1', v: 's5v1', at: 10.5, num: 15.47, suffix: ' s · 13× s3fs' },
    { b: 's5b2', r: 's5r2', v: 's5v2', at: 11.4, num: 18.11, suffix: ' s' }
  ];

  function tickS5(t) {
    S5BARS.forEach(function (B) {
      var p = prog(t, B.at, 1.1);
      var row = $(B.r), bar = $(B.b), val = $(B.v);
      on(row, p > 0);
      if (bar) bar.style.transform = 'scaleX(' + Math.max(0.02, p) + ')';
      if (val && p > 0) {
        var cur = (B.num * p).toFixed(2) + (p >= 0.99 ? B.suffix : ' s');
        if (val.textContent !== cur) val.textContent = cur;
      }
    });
    on($('s5verdict'), t >= 13.5);
    if (t >= 10.2 && t < 16) setCam('cam5', 0, 18, 1.05);
    else setCam('cam5', 0, 0, 1);
  }

  function tickS6(t) {
    on($('s6p1'), t >= 5.5 && t < 15);
    on($('s6p2'), t >= 6.4 && t < 15);
    on($('s6cache'), t >= 6.8 && t < 14.5);
    on($('s6a'), t >= 11);
    on($('s6b'), t >= 11);
    on($('s6rejoin'), t >= 15);
    boxHi($('s6mA'), t >= 5.5 && t < 11);
    boxHi($('s6mB'), t >= 6.8 && t < 15);
    var bst = $('s6b-state');
    var btxt = t < 6.8 ? 'cache: warm' : t < 15 ? 'cache: evicted!' : 'cache: healed';
    if (bst && bst.textContent !== btxt) bst.textContent = btxt;
    if (t < 5.5) setCam('cam6', 0, 10, 1.03);
    else if (t < 11) setCam('cam6', -28, 6, 1.06);
    else setCam('cam6', 0, 0, 1);
  }

  function tickS7(t) {
    on($('s7k1'), t >= 0);
    on($('s7k2'), t >= 2.2);
    on($('s7k3'), t >= 4.0);
    on($('s7a'), t < 10);
    on($('s7b'), t >= 5 && t < 15);
    on($('s7r'), t >= 10 && t < 15);
    on($('s7g'), t >= 15);
    on($('s7pulse'), t >= 10);
    boxHi($('s7mA'), t < 10);
    boxHi($('s7mB'), t >= 15);
    var h = $('s7holder');
    var s = t < 10 ? 'holder: mount A (F_WRLCK)' : t < 15 ? 'releasing → waking B' : 'holder: mount B (granted)';
    if (h && h.textContent !== s) h.textContent = s;
    var g = $('s7g');
    if (g && t >= 15) {
      var rows = Math.round(200 * prog(t, 15.2, 2.2));
      var gtxt = 'B retries → granted · ' + rows + ' rows, integrity ok';
      if (g.textContent !== gtxt) g.textContent = gtxt;
    }
    if (t < 5) setCam('cam7', 0, -6, 1.05);
    else if (t < 15) setCam('cam7', 0, 8, 1.04);
    else setCam('cam7', 0, -8, 1.04);
  }

  function tickS8(t) {
    on($('s8h1'), t >= 0);
    on($('s8h2'), t >= 5.0);
    on($('s8h3'), t >= 6.4);
    on($('s8h4'), t >= 7.8);
    on($('s8h5'), t >= 9.2);
    boxHi($('s8b1'), t >= 5.0 && t < 6.4);
    boxHi($('s8b2'), t >= 6.4 && t < 7.8);
    boxHi($('s8b3'), t >= 7.8 && t < 9.2);
    boxHi($('s8b4'), t >= 9.2 && t < 11.0);
    var st = $('s8-stat');
    var msg = t < 5
      ? 'payload in [64 KiB, Max] → bypass FlatBuffers envelope'
      : t < 11
      ? 'kernel splice(2): file → pipe → socket → pipe → FUSE'
      : '0 user-space copies · transient error retries 1×';
    if (st && st.textContent !== msg) st.textContent = msg;
    on($('s8close'), t >= 15);
    if (t >= 5 && t < 14) setCam('cam8', 0, 12, 1.05);
    else setCam('cam8', 0, 0, 1);
  }

  var TICKS = {
    sc1: tickS1,
    sc2: tickS2,
    sc3: tickS3,
    sc4: tickS4,
    sc5: tickS5,
    sc6: tickS6,
    sc7: tickS7,
    sc8: tickS8
  };

  // ---- transport & cinema HUD ----
  var player = $('player');
  var stage = $('stage');
  var playOverlay = $('play-overlay');
  var hudTitle = $('hud-title');
  var hudTc = $('hud-tc');
  var caption = $('caption');
  var fill = $('fill');
  var segTrack = $('seg-track');
  var scrubTip = $('scrub-tip');
  var clock = $('clock');
  var progress = $('progress');
  var playBtn = $('btn-play');
  var speedBtn = $('btn-speed');
  var audioBtn = $('btn-audio');
  var fsBtn = $('btn-fs');
  var chaptersEl = $('chapters');

  var SPEEDS = [1, 1.25, 1.5, 2];
  var speedIdx = 0;
  var t = 0;
  var playing = false;
  var scrubbing = false;
  var lastTs = 0;
  var activeIdx = -1;

  scenes.forEach(function (s, i) {
    if (segTrack && i < scenes.length - 1) {
      var seg = doc.createElement('div');
      seg.className = 'seg-tick';
      seg.style.width = (100 * s.dur / total) + '%';
      segTrack.appendChild(seg);
    }
    var b = doc.createElement('button');
    b.type = 'button';
    b.textContent = (i + 1) + ' · ' + s.title;
    b.setAttribute('aria-label', 'Go to scene ' + (i + 1) + ': ' + s.title);
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

  function pad2(n) { return n < 10 ? '0' + n : String(n); }

  function sceneAt(time) {
    for (var i = scenes.length - 1; i >= 0; i--) {
      if (time >= scenes[i].start) return i;
    }
    return 0;
  }

  function activate(i) {
    if (i === activeIdx) return;
    if (activeIdx !== -1) playTone(330, 0.11, 0.03, 'sine');
    scenes.forEach(function (s, j) {
      s.root.classList.toggle('active', j === i);
    });
    activeIdx = i;
    var btns = chaptersEl.children;
    for (var k = 0; k < btns.length; k++) {
      btns[k].classList.toggle('here', k === i);
    }
    if (hudTitle) {
      hudTitle.textContent = pad2(i + 1) + ' / ' + pad2(scenes.length) + ' · ' + scenes[i].title;
    }
  }

  function render() {
    var i = sceneAt(Math.min(t, total - 0.001));
    activate(i);
    var local = t - scenes[i].start;
    var tick = TICKS[scenes[i].root.id];
    if (tick) tick(local);
    if (hudTc) {
      hudTc.textContent = 'SCENE ' + (local < 10 ? '0' : '') + local.toFixed(1) + 's';
    }
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
    if (lastTs) {
      var dt = ((ts - lastTs) / 1000) * SPEEDS[speedIdx];
      t = Math.min(total, t + dt);
    }
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
  if (playOverlay) {
    playOverlay.addEventListener('click', function (ev) {
      ev.stopPropagation();
      setPlaying(true);
    });
  }
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

  if (speedBtn) {
    speedBtn.addEventListener('click', function () {
      speedIdx = (speedIdx + 1) % SPEEDS.length;
      speedBtn.textContent = SPEEDS[speedIdx] + '×';
    });
  }

  if (audioBtn) {
    audioBtn.addEventListener('click', function () {
      audioOn = !audioOn;
      audioBtn.setAttribute('aria-pressed', audioOn ? 'true' : 'false');
      audioBtn.textContent = audioOn ? 'Audio: On' : 'Audio: Off';
      if (audioOn) {
        ensureAudio();
        playTone(440, 0.08, 0.025, 'sine');
      }
    });
  }

  if (fsBtn && player) {
    fsBtn.addEventListener('click', function () {
      if (!doc.fullscreenElement) {
        if (player.requestFullscreen) player.requestFullscreen().catch(function () {});
      } else if (doc.exitFullscreen) {
        doc.exitFullscreen().catch(function () {});
      }
    });
    doc.addEventListener('fullscreenchange', function () {
      fsBtn.textContent = doc.fullscreenElement ? 'Exit Fullscreen' : 'Fullscreen';
    });
  }

  function updateHoverTip(clientX) {
    var r = progress.getBoundingClientRect();
    if (!r.width) return 0;
    var ratio = clamp01((clientX - r.left) / r.width);
    var hoverT = total * ratio;
    var idx = sceneAt(Math.min(hoverT, total - 0.001));
    progress.style.setProperty('--hover-pct', (ratio * 100) + '%');
    if (scrubTip) {
      scrubTip.textContent = fmt(hoverT) + ' · ' + (idx + 1) + '. ' + scenes[idx].title;
    }
    return hoverT;
  }

  function seekAtClientX(clientX) {
    t = updateHoverTip(clientX);
    render();
  }

  progress.addEventListener('pointermove', function (ev) {
    updateHoverTip(ev.clientX);
    if (scrubbing) seekAtClientX(ev.clientX);
  });
  progress.addEventListener('pointerdown', function (ev) {
    scrubbing = true;
    progress.classList.add('scrubbing');
    if (progress.setPointerCapture) {
      try { progress.setPointerCapture(ev.pointerId); } catch (_) {}
    }
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
    } else if (ev.key === 'm') {
      if (audioBtn) audioBtn.click();
    } else if (ev.key === 'f') {
      if (fsBtn) fsBtn.click();
    }
  });

  doc.addEventListener('visibilitychange', function () {
    if (doc.hidden) lastTs = 0;
  });

  var hash = window.location.hash.match(/^#t=(\d+(?:\.\d+)?)$/);
  if (hash) t = Math.max(0, Math.min(total, parseFloat(hash[1])));

  doc.body.classList.add('paused');
  render();
})();
