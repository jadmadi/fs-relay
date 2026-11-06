// fs-relay 3D Protocol & Topology Lab (Three.js + Claude Interactive Diagram Pattern)
import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { STRUCTURES, EDGES, QUIZ, EVERYDAY_FLEET, GUIDE_STEPS } from './data.js';

// --- Web Audio API Synthesizer (muted by default) ---
let audioOn = false;
let audioCtx = null;

function playTone(freq = 520, dur = 0.06, type = 'sine', gainVal = 0.04) {
  if (!audioOn) return;
  if (!audioCtx) audioCtx = new (window.AudioContext || window.webkitAudioContext)();
  if (audioCtx.state === 'suspended') audioCtx.resume();
  const osc = audioCtx.createOscillator();
  const gain = audioCtx.createGain();
  osc.type = type;
  osc.frequency.setValueAtTime(freq, audioCtx.currentTime);
  gain.gain.setValueAtTime(gainVal, audioCtx.currentTime);
  gain.gain.exponentialRampToValueAtTime(0.0001, audioCtx.currentTime + dur);
  osc.connect(gain);
  gain.connect(audioCtx.destination);
  osc.start();
  osc.stop(audioCtx.currentTime + dur);
}

// --- Three.js Scene Initialization ---
const stageEl = document.getElementById('canvas-stage');
const canvasEl = document.getElementById('canvas3d');
const labelLayer = document.getElementById('label-layer');

const renderer = new THREE.WebGLRenderer({ canvas: canvasEl, antialias: true, alpha: true });
renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

const scene = new THREE.Scene();
const camera = new THREE.PerspectiveCamera(42, 1, 0.1, 200);
camera.position.set(0, 7.8, 22.0);

const controls = new OrbitControls(camera, canvasEl);
controls.enableDamping = true;
controls.dampingFactor = 0.06;
controls.maxPolarAngle = Math.PI * 0.82;
controls.minDistance = 6;
controls.maxDistance = 36;
controls.target.set(0, 0, 0);

const grid = new THREE.GridHelper(28, 28, 0x263540, 0x162129);
grid.position.y = -4.0;
scene.add(grid);

scene.add(new THREE.AmbientLight(0xdbe4eb, 1.35));
const dirLight = new THREE.DirectionalLight(0xffffff, 1.9);
dirLight.position.set(10, 18, 14);
scene.add(dirLight);
const rimLight = new THREE.DirectionalLight(0x38bdf8, 0.8);
rimLight.position.set(-12, -8, -10);
scene.add(rimLight);

// --- Build 3D Structures & Multi-Machine Silhouettes ---
const meshMap = new Map();
const pickables = [];
const tagMap = new Map();
const tubeGroup = new THREE.Group();
scene.add(tubeGroup);

function buildGeometry(s) {
  if (s.geom === 'cylinder') {
    const g = new THREE.CylinderGeometry(s.size[0], s.size[1], s.size[2], 28);
    if (s.id === 'raw-splice') g.rotateZ(Math.PI / 2);
    return g;
  }
  if (s.geom === 'torus') return new THREE.TorusGeometry(s.size[0], s.size[1], 18, 42);
  if (s.geom === 'octa') return new THREE.OctahedronGeometry(s.size[0], 0);
  if (s.geom === 'lockrings') return new THREE.TorusGeometry(s.size[0], s.size[1], 16, 48);
  if (s.id === 'mount-a') return new THREE.BoxGeometry(2.3, 0.22, 1.6); // Laptop base deck
  if (s.id === 'mount-b') return new THREE.BoxGeometry(2.2, 1.35, 0.22); // Desktop monitor panel
  return new THREE.BoxGeometry(s.size[0], s.size[1], s.size[2]);
}

function decorateMultiMachineNode(group, s) {
  if (s.id === 'mount-a') {
    // Hinged laptop screen
    const lid = new THREE.Mesh(
      new THREE.BoxGeometry(2.25, 1.35, 0.12),
      new THREE.MeshStandardMaterial({ color: 0xf43f5e, roughness: 0.25, metalness: 0.3 })
    );
    lid.position.set(0, 0.72, -0.72);
    lid.rotation.x = -0.22;
    group.add(lid);
  } else if (s.id === 'mount-b') {
    // Monitor stand + desktop workstation tower + cloud VM blade
    const stand = new THREE.Mesh(
      new THREE.BoxGeometry(0.9, 0.16, 0.9),
      new THREE.MeshStandardMaterial({ color: 0x38bdf8, roughness: 0.35 })
    );
    stand.position.set(0, -0.78, 0);
    const tower = new THREE.Mesh(
      new THREE.BoxGeometry(0.75, 1.55, 1.25),
      new THREE.MeshStandardMaterial({ color: 0x0ea5e9, roughness: 0.3, transparent: true, opacity: 0.88 })
    );
    tower.position.set(-1.65, -0.05, 0.2);
    const vmBlade = new THREE.Mesh(
      new THREE.BoxGeometry(1.8, 0.42, 1.2),
      new THREE.MeshStandardMaterial({ color: 0x38bdf8, roughness: 0.3, transparent: true, opacity: 0.82 })
    );
    vmBlade.position.set(0.2, 1.25, -0.9);
    group.add(stand, tower, vmBlade);
  } else if (s.id === 'disk-store') {
    // 3 shared live files resting inside the single host directory
    const colors = [0xf43f5e, 0x38bdf8, 0x34d399];
    colors.forEach((col, i) => {
      const f = new THREE.Mesh(
        new THREE.BoxGeometry(0.65, 0.48, 0.85),
        new THREE.MeshStandardMaterial({ color: col, roughness: 0.25 })
      );
      f.position.set((i - 1) * 0.72, 1.45, 0);
      group.add(f);
    });
  }
}

STRUCTURES.forEach((s) => {
  const group = new THREE.Group();
  group.position.set(...s.pos.everyday);
  group.userData = { id: s.id, struct: s };

  const geom = buildGeometry(s);
  const mat = new THREE.MeshStandardMaterial({
    color: s.color,
    roughness: 0.32,
    metalness: 0.25,
    transparent: true,
    opacity: 0.88
  });
  const mesh = new THREE.Mesh(geom, mat);
  mesh.userData = { id: s.id, struct: s };
  group.add(mesh);
  pickables.push(mesh);

  const edges = new THREE.LineSegments(
    new THREE.EdgesGeometry(geom),
    new THREE.LineBasicMaterial({ color: 0xffffff, transparent: true, opacity: 0.35 })
  );
  group.add(edges);

  if (s.geom === 'lockrings') {
    const inner1 = new THREE.Mesh(
      new THREE.TorusGeometry(1.05, 0.14, 14, 40),
      new THREE.MeshStandardMaterial({ color: 0xf59e0b, roughness: 0.3, transparent: true, opacity: 0.85 })
    );
    inner1.rotation.x = Math.PI / 3;
    const inner2 = new THREE.Mesh(
      new THREE.TorusGeometry(0.62, 0.12, 14, 36),
      new THREE.MeshStandardMaterial({ color: 0xf43f5e, roughness: 0.3, transparent: true, opacity: 0.9 })
    );
    inner2.rotation.y = Math.PI / 3;
    group.add(inner1, inner2);
    group.userData.rings = [mesh, inner1, inner2];
  }

  decorateMultiMachineNode(group, s);
  scene.add(group);
  meshMap.set(s.id, { group, mesh, mat, edges, targetPos: new THREE.Vector3(...s.pos.everyday) });

  const tag = document.createElement('button');
  tag.type = 'button';
  tag.className = 'node-tag';
  tag.dataset.id = s.id;
  const codeSpan = document.createElement('span');
  codeSpan.className = 'code';
  codeSpan.textContent = s.code + ' · ';
  const nameSpan = document.createElement('span');
  nameSpan.textContent = s.name.replace(/\s*\(.*\)/, '');
  tag.append(codeSpan, nameSpan);
  tag.addEventListener('click', () => handleStructureActivated(s.id));
  labelLayer.appendChild(tag);
  tagMap.set(s.id, tag);
});

// --- State & Camera Transitions ---
let currentView = 'everyday';
let currentSystem = 'all';
let selectedId = 'disk-store';
let cameraGoalPos = new THREE.Vector3(0, 8.2, 23.2);
let cameraGoalTarget = new THREE.Vector3(0, 0, 0);
let cameraLerpFrames = 0;

// --- Build Everyday Multi-Machine & AI Agent Fleet ---
const fleetGroup = new THREE.Group();
scene.add(fleetGroup);
const fleetEntries = [];

function buildFleetNode(f) {
  const g = new THREE.Group();
  g.position.set(...f.pos);
  const mat = new THREE.MeshStandardMaterial({ color: f.color, roughness: 0.28, metalness: 0.28 });
  if (f.kind === 'agent') {
    const core = new THREE.Mesh(new THREE.OctahedronGeometry(0.85, 0), mat);
    const halo = new THREE.Mesh(
      new THREE.TorusGeometry(1.25, 0.08, 12, 36),
      new THREE.MeshBasicMaterial({ color: f.color, transparent: true, opacity: 0.78 })
    );
    halo.rotation.x = Math.PI / 3;
    g.add(core, halo);
    g.userData.halo = halo;
  } else if (f.kind === 'server') {
    for (let i = -1; i <= 1; i++) {
      const blade = new THREE.Mesh(new THREE.BoxGeometry(1.9, 0.34, 1.35), mat);
      blade.position.y = i * 0.48;
      g.add(blade);
    }
  } else {
    const base = new THREE.Mesh(new THREE.BoxGeometry(1.95, 0.2, 1.35), mat);
    const screen = new THREE.Mesh(new THREE.BoxGeometry(1.9, 1.15, 0.1), mat);
    screen.position.set(0, 0.62, -0.62);
    screen.rotation.x = -0.22;
    g.add(base, screen);
  }
  fleetGroup.add(g);

  const tag = document.createElement('button');
  tag.type = 'button';
  tag.className = 'node-tag';
  const codeSpan = document.createElement('span');
  codeSpan.className = 'code';
  codeSpan.textContent = f.code + ' · ';
  const nameSpan = document.createElement('span');
  nameSpan.textContent = f.name.replace(/\s*\(.*\)/, '');
  tag.append(codeSpan, nameSpan);
  tag.addEventListener('click', () => {
    selectStructure(f.linkedStruct);
    spawnPacketFromVec(g.position, meshMap.get('ws-mux').group.position, f.color);
    setTimeout(() => spawnPacket('ws-mux', 'disk-store', f.color, 0.024), 260);
  });
  labelLayer.appendChild(tag);
  fleetEntries.push({ item: f, group: g, tag });
}
EVERYDAY_FLEET.forEach(buildFleetNode);

// --- Build 3D Connecting Tubes ---
const activeParticles = [];

function addCurveTube(a, b, color, yLift, opacity) {
  const mid = a.clone().lerp(b, 0.5);
  mid.y += yLift;
  const curve = new THREE.CatmullRomCurve3([a.clone(), mid, b.clone()]);
  const geom = new THREE.TubeGeometry(curve, 28, 0.075, 8, false);
  const mat = new THREE.MeshBasicMaterial({ color, transparent: true, opacity });
  tubeGroup.add(new THREE.Mesh(geom, mat));
}

function syncNetworkTubes(activeSystem = 'all') {
  while (tubeGroup.children.length) {
    const c = tubeGroup.children.pop();
    c.geometry.dispose();
    c.material.dispose();
  }
  if (currentView === 'everyday') {
    const muxPos = meshMap.get('ws-mux').targetPos;
    const diskPos = meshMap.get('disk-store').targetPos;
    addCurveTube(meshMap.get('mount-a').targetPos, muxPos, 0xf43f5e, 0.55, 0.62);
    addCurveTube(meshMap.get('mount-b').targetPos, muxPos, 0x38bdf8, 0.55, 0.62);
    addCurveTube(muxPos, diskPos, 0x34d399, 0.55, 0.68);
    fleetEntries.forEach(({ item, group }) => {
      addCurveTube(group.position, muxPos, item.color, 0.45, 0.52);
    });
    return;
  }
  EDGES.forEach((e) => {
    const match = activeSystem === 'all' || activeSystem === e.system;
    addCurveTube(
      meshMap.get(e.from).targetPos,
      meshMap.get(e.to).targetPos,
      e.color,
      e.system === 'raw' ? -0.6 : 0.55,
      match ? 0.52 : 0.08
    );
  });
}
syncNetworkTubes();

function spawnPacketFromVec(aVec, bVec, colorHex, speed = 0.02) {
  const a = aVec.clone();
  const b = bVec.clone();
  const mid = a.clone().lerp(b, 0.5);
  mid.y += 0.55;
  const curve = new THREE.CatmullRomCurve3([a, mid, b]);
  const mesh = new THREE.Mesh(
    new THREE.SphereGeometry(0.24, 14, 14),
    new THREE.MeshBasicMaterial({ color: colorHex })
  );
  scene.add(mesh);
  activeParticles.push({ mesh, curve, t: 0, speed });
}

function spawnPacket(fromId, toId, colorHex, speed = 0.018) {
  spawnPacketFromVec(
    meshMap.get(fromId).group.position,
    meshMap.get(toId).group.position,
    colorHex,
    speed
  );
}

function moveCameraTo(posArr, targetArr, frames = 45) {
  cameraGoalPos.set(...posArr);
  cameraGoalTarget.set(...targetArr);
  cameraLerpFrames = frames;
}

function clearChildren(el) {
  while (el.firstChild) el.removeChild(el.firstChild);
}

function renderCardList(el, pairs) {
  clearChildren(el);
  pairs.forEach(([k, v]) => {
    const card = document.createElement('div');
    card.className = 'metric-card';
    const kEl = document.createElement('span');
    kEl.className = 'k';
    kEl.textContent = k;
    const vEl = document.createElement('span');
    vEl.className = 'v';
    vEl.textContent = v;
    card.append(kEl, vEl);
    el.appendChild(card);
  });
}

function renderRelatedChips(el, relIds) {
  clearChildren(el);
  relIds.forEach((relId) => {
    const relObj = STRUCTURES.find((x) => x.id === relId);
    if (!relObj) return;
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'chip-btn';
    btn.textContent = relObj.code + ' · ' + relObj.name.replace(/\s*\(.*\)/, '');
    btn.addEventListener('click', () => {
      selectStructure(relObj.id);
      const p = meshMap.get(relObj.id).targetPos;
      moveCameraTo([p.x * 0.7, p.y + 4.5, p.z + 11.5], [p.x, p.y, p.z], 38);
    });
    el.appendChild(btn);
  });
}

function ensureDirectoryGrid() {
  const dirGrid = document.getElementById('dir-grid');
  if (!dirGrid || dirGrid.firstChild) return;
  STRUCTURES.forEach((item) => {
    const b = document.createElement('button');
    b.type = 'button';
    b.className = 'chip-btn';
    b.textContent = item.code + ' · ' + item.name.replace(/\s*\(.*\)/, '');
    b.addEventListener('click', () => selectStructure(item.id));
    dirGrid.appendChild(b);
  });
}

// --- Inspector Population ---
function selectStructure(id) {
  const s = STRUCTURES.find((item) => item.id === id);
  if (!s) return;
  selectedId = id;
  playTone(580, 0.05, 'triangle', 0.045);

  STRUCTURES.forEach((item) => {
    const entry = meshMap.get(item.id);
    const isSel = item.id === id;
    entry.edges.material.opacity = isSel ? 0.95 : 0.32;
    entry.edges.material.color.setHex(isSel ? 0xf3f5f6 : 0xffffff);
    tagMap.get(item.id).classList.toggle('selected', isSel);
  });

  document.getElementById('ins-id').textContent = 'STRUCT · ' + s.code;
  document.getElementById('ins-sys').textContent = s.system;
  document.getElementById('ins-title').textContent = s.name;
  document.getElementById('ins-pkg').textContent = s.tagline;
  document.getElementById('ins-summary').textContent = s.overview;
  document.getElementById('ins-invariant').textContent = s.guarantees[0] || '';
  document.getElementById('ins-failure').textContent = s.guarantees[1] || '';

  renderCardList(document.getElementById('ins-metrics'), s.specs.slice(0, 2));
  const factsUl = document.getElementById('ins-facts');
  clearChildren(factsUl);
  s.specs.forEach(([k, v]) => {
    const li = document.createElement('li');
    li.textContent = k + ': ' + v;
    factsUl.appendChild(li);
  });
  renderRelatedChips(document.getElementById('ins-related'), s.related);
  ensureDirectoryGrid();
}

// --- Self-Test Quiz Mode ---
let quizIndex = 0;
let quizScore = 0;
const answeredSet = new Set();

function updateQuizUI() {
  const item = QUIZ[quizIndex];
  document.getElementById('quiz-step').textContent = 'Question ' + (quizIndex + 1) + ' of ' + QUIZ.length;
  document.getElementById('quiz-streak').textContent = 'Accuracy: ' + quizScore + ' / ' + QUIZ.length;
  document.getElementById('quiz-score-badge').textContent = quizScore + '/' + QUIZ.length;
  document.getElementById('quiz-prompt').textContent = item.q;
  const fb = document.getElementById('quiz-feedback');
  fb.textContent = 'Click any 3D structure or floating badge in the viewport to submit your answer.';
  fb.className = 'quiz-feedback';

  const choicesEl = document.getElementById('quiz-choices');
  clearChildren(choicesEl);
  STRUCTURES.forEach((s) => {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'chip-btn';
    btn.dataset.qid = s.id;
    btn.textContent = s.code;
    btn.addEventListener('click', () => handleStructureActivated(s.id));
    choicesEl.appendChild(btn);
  });
}

function handleQuizAttempt(clickedId) {
  const item = QUIZ[quizIndex];
  const fb = document.getElementById('quiz-feedback');
  const picked = STRUCTURES.find((s) => s.id === clickedId);
  if (clickedId === item.target) {
    if (!answeredSet.has(quizIndex)) {
      answeredSet.add(quizIndex);
      quizScore += 1;
    }
    playTone(760, 0.09, 'sine', 0.06);
    fb.textContent = 'Correct: ' + picked.code + ' (' + picked.name + ').';
    spawnPacket(clickedId, item.target, 0x34d399, 0.03);
  } else {
    playTone(240, 0.11, 'sawtooth', 0.05);
    fb.textContent = 'Not ' + (picked ? picked.code : clickedId) + '. Hint: ' + item.hint;
  }
  document.getElementById('quiz-streak').textContent = 'Accuracy: ' + quizScore + ' / ' + QUIZ.length;
  document.getElementById('quiz-score-badge').textContent = quizScore + '/' + QUIZ.length;
}

function activateInspectorTab(tabName) {
  document.querySelectorAll('.ins-tab').forEach((b) => {
    const active = b.dataset.tab === tabName;
    b.classList.toggle('active', active);
    b.setAttribute('aria-selected', String(active));
  });
  document.querySelectorAll('.ins-body').forEach((p) => {
    p.classList.toggle('hidden', p.id !== 'tab-' + tabName);
  });
}

function handleStructureActivated(id) {
  selectStructure(id);
  const quizPaneActive = !document.getElementById('tab-quiz').classList.contains('hidden');
  if (currentView === 'quiz' || quizPaneActive) {
    handleQuizAttempt(id);
    return;
  }
  const guideCard = document.getElementById('guide-card');
  if (guideCard && !guideCard.classList.contains('hidden') && GUIDE_STEPS[guideIdx].nextTarget === id) {
    stopGuideAutoPlay();
    applyGuideStep(guideIdx + 1, true);
  }
}

const EVERYDAY_PRIMARY = new Set(['mount-a', 'mount-b', 'ws-mux', 'disk-store']);

const VIEW_PRESETS = {
  everyday: {
    posKey: 'everyday',
    hover: 'Everyday View · Laptop, Office Desktop & Cloud VM sharing 1 live server folder (0 GB copied)',
    telemetry: 'LAYER 1 · BUSINESS & WORKFLOW',
    cam: [[0, 7.8, 22.0], [0, 0, 0]],
    tab: 'overview'
  },
  cutaway: {
    posKey: 'cutaway',
    hover: 'Cutaway · Byte-range lock rings, FlatBuffers codec, and sandboxed inode resolution',
    telemetry: 'LAYER 2 · KERNEL & LOCK CUTAWAY',
    cam: [[1.8, 3.8, 11.5], [1.8, 0.5, 0.2]],
    tab: 'overview',
    focus: 'lock-table'
  },
  quiz: {
    posKey: 'topology',
    hover: 'Self-Test Mode · Click the 3D structure or badge that answers the prompt',
    telemetry: 'VIEW · QUIZ (8 Qs)',
    cam: [[0, 8.2, 22.8], [0, 0, 0]],
    tab: 'quiz'
  },
  topology: {
    posKey: 'topology',
    hover: 'System Topology · All 10 FUSE, WebSocket, Lock, Splice & Inode structures',
    telemetry: 'LAYER 2 · OS & PROTOCOL TOPOLOGY',
    cam: [[0, 8.2, 22.8], [0, 0, 0]],
    tab: 'overview'
  }
};

function updateStructureVisibilityForView(view, posKey) {
  const isEveryday = view === 'everyday';
  fleetGroup.visible = isEveryday;
  fleetEntries.forEach(({ tag }) => {
    tag.style.visibility = isEveryday ? 'visible' : 'hidden';
  });
  STRUCTURES.forEach((s) => {
    const entry = meshMap.get(s.id);
    entry.targetPos.set(...s.pos[posKey]);
    const show = !isEveryday || EVERYDAY_PRIMARY.has(s.id);
    entry.group.visible = show;
    entry.mat.opacity = 0.9;
    tagMap.get(s.id).style.visibility = show ? 'visible' : 'hidden';
    tagMap.get(s.id).classList.remove('dimmed');
  });
}

// --- View Switcher, Subsystem Filters & Simulations ---
function setViewMode(view, moveCam = true) {
  currentView = view;
  const preset = VIEW_PRESETS[view] || VIEW_PRESETS.topology;
  document.querySelectorAll('.view-switcher [data-view]').forEach((b) => {
    const active = b.dataset.view === view;
    b.classList.toggle('active', active);
    b.setAttribute('aria-pressed', String(active));
  });

  updateStructureVisibilityForView(view, preset.posKey);
  setTimeout(() => syncNetworkTubes(currentSystem), 180);

  document.getElementById('hud-hover-text').textContent = preset.hover;
  document.getElementById('hud-telemetry').textContent = preset.telemetry;
  if (moveCam) moveCameraTo(preset.cam[0], preset.cam[1], 45);
  if (preset.focus) selectStructure(preset.focus);
  activateInspectorTab(preset.tab);
  if (view === 'quiz') updateQuizUI();
}

function setSystemFilter(sys) {
  currentSystem = sys;
  document.querySelectorAll('.sys-tab').forEach((b) => {
    const active = b.dataset.sys === sys;
    b.classList.toggle('active', active);
    b.setAttribute('aria-pressed', String(active));
  });
  STRUCTURES.forEach((s) => {
    const match = sys === 'all' || s.system === sys;
    meshMap.get(s.id).mat.opacity = match ? 0.9 : 0.16;
    tagMap.get(s.id).classList.toggle('dimmed', !match);
  });
  syncNetworkTubes(sys);
  playTone(460, 0.04, 'sine', 0.035);
}

function runSimulation(simType) {
  if (currentView === 'everyday') setViewMode('topology', true);
  playTone(640, 0.07, 'triangle', 0.05);
  document.getElementById('hud-telemetry').textContent = 'SIM · ' + simType.toUpperCase();
  if (simType === 'write') {
    selectStructure('hub-fanout');
    spawnPacket('mount-a', 'ws-mux', 0xf43f5e, 0.022);
    setTimeout(() => spawnPacket('ws-mux', 'wire-codec', 0xf43f5e, 0.022), 260);
    setTimeout(() => {
      spawnPacket('wire-codec', 'inode-map', 0xf59e0b, 0.022);
      spawnPacket('wire-codec', 'hub-fanout', 0x38bdf8, 0.022);
    }, 520);
    setTimeout(() => {
      spawnPacket('inode-map', 'disk-store', 0x94a3b8, 0.022);
      spawnPacket('hub-fanout', 'mount-b', 0x38bdf8, 0.022);
    }, 820);
  } else if (simType === 'lock') {
    selectStructure('lock-table');
    spawnPacket('mount-a', 'ws-mux', 0x34d399, 0.02);
    setTimeout(() => spawnPacket('ws-mux', 'wire-codec', 0x34d399, 0.02), 250);
    setTimeout(() => spawnPacket('wire-codec', 'lock-table', 0x34d399, 0.02), 500);
    setTimeout(() => spawnPacket('lock-table', 'inode-map', 0x34d399, 0.02), 800);
  } else if (simType === 'splice') {
    selectStructure('raw-splice');
    for (let i = 0; i < 4; i++) {
      setTimeout(() => {
        spawnPacket('mount-a', 'raw-splice', 0xa78bfa, 0.026);
        setTimeout(() => spawnPacket('raw-splice', 'disk-store', 0xa78bfa, 0.026), 300);
      }, i * 140);
    }
  } else if (simType === 'rejoin') {
    selectStructure('post-join');
    spawnPacket('mount-a', 'post-join', 0xf59e0b, 0.02);
    setTimeout(() => spawnPacket('post-join', 'wire-codec', 0xf59e0b, 0.02), 320);
    setTimeout(() => spawnPacket('wire-codec', 'lock-table', 0x34d399, 0.02), 640);
  }
}

// --- Guided Lifecycle Tour & Interactive Coaching Tips (Steps 0..7) ---
let guideIdx = 0;
let guideVisited = new Set([0]);
let guideTimer = null;

const beaconMesh = new THREE.Mesh(
  new THREE.RingGeometry(1.35, 1.62, 36),
  new THREE.MeshBasicMaterial({ color: 0xf43f5e, side: THREE.DoubleSide, transparent: true, opacity: 0.75 })
);
beaconMesh.rotation.x = Math.PI / 2;
scene.add(beaconMesh);

function applyGuideStep(idx, moveCam = true) {
  guideIdx = (idx + GUIDE_STEPS.length) % GUIDE_STEPS.length;
  guideVisited.add(guideIdx);
  const step = GUIDE_STEPS[guideIdx];

  if (step.view && step.view !== currentView && currentView !== 'quiz') {
    setViewMode(step.view, false);
  }

  document.getElementById('guide-step-badge').textContent = step.badge;
  document.getElementById('guide-title').textContent = step.title;
  document.getElementById('guide-body').textContent = step.body;
  document.getElementById('guide-tip-text').textContent = step.tip;

  const dotsEl = document.getElementById('guide-dots');
  clearChildren(dotsEl);
  GUIDE_STEPS.forEach((_, i) => {
    const dot = document.createElement('button');
    dot.type = 'button';
    dot.className = 'guide-dot' + (i === guideIdx ? ' active' : (guideVisited.has(i) ? ' done' : ''));
    dot.textContent = String(i);
    dot.setAttribute('aria-label', 'Go to step ' + i);
    dot.addEventListener('click', () => {
      stopGuideAutoPlay();
      applyGuideStep(i, true);
    });
    dotsEl.appendChild(dot);
  });

  STRUCTURES.forEach((s) => {
    tagMap.get(s.id).classList.toggle('coach-target', s.id === step.nextTarget);
  });

  selectStructure(step.focus);
  if (moveCam) moveCameraTo(step.cam[0], step.cam[1], 44);
  step.packets.forEach(([from, to, col], i) => {
    setTimeout(() => spawnPacket(from, to, col, 0.022), i * 240);
  });
}

function stopGuideAutoPlay() {
  if (!guideTimer) return;
  clearInterval(guideTimer);
  guideTimer = null;
  const playBtn = document.getElementById('btn-guide-play');
  playBtn.setAttribute('aria-pressed', 'false');
  playBtn.textContent = '▶ Auto-Play Cycle';
}

function toggleGuideAutoPlay() {
  const playBtn = document.getElementById('btn-guide-play');
  if (guideTimer) {
    stopGuideAutoPlay();
    return;
  }
  playBtn.setAttribute('aria-pressed', 'true');
  playBtn.textContent = '⏸ Pause Cycle';
  applyGuideStep(guideIdx, true);
  guideTimer = setInterval(() => {
    applyGuideStep(guideIdx + 1, true);
  }, 4400);
}

// --- Bind DOM Controls ---
document.querySelectorAll('.view-switcher [data-view]').forEach((btn) => {
  btn.addEventListener('click', () => setViewMode(btn.dataset.view, true));
});
document.querySelectorAll('.sys-tab').forEach((btn) => {
  btn.addEventListener('click', () => setSystemFilter(btn.dataset.sys));
});
document.querySelectorAll('.sim-btn').forEach((btn) => {
  btn.addEventListener('click', () => runSimulation(btn.dataset.sim));
});
document.querySelectorAll('.ins-tab').forEach((btn) => {
  btn.addEventListener('click', () => activateInspectorTab(btn.dataset.tab));
});

const slider = document.getElementById('payload-slider');
const payloadOut = document.getElementById('payload-val');
const routeVerdict = document.getElementById('route-verdict');
slider.addEventListener('input', () => {
  const kb = Number(slider.value);
  payloadOut.textContent = kb + ' KiB';
  if (kb <= 64) {
    routeVerdict.textContent = 'Route: WS FlatBuffers';
    selectStructure('ws-mux');
    spawnPacket('mount-a', 'ws-mux', 0xf43f5e, 0.028);
  } else {
    routeVerdict.textContent = 'Route: raw splice(2)';
    selectStructure('raw-splice');
    spawnPacket('mount-a', 'raw-splice', 0xa78bfa, 0.028);
  }
});

document.getElementById('btn-reset-cam').addEventListener('click', () => {
  setSystemFilter('all');
  setViewMode('everyday', true);
});

const audioBtn = document.getElementById('btn-audio');
audioBtn.addEventListener('click', () => {
  audioOn = !audioOn;
  audioBtn.setAttribute('aria-pressed', String(audioOn));
  audioBtn.textContent = audioOn ? 'Audio: On' : 'Audio: Off';
  if (audioOn) playTone(660, 0.06, 'sine', 0.05);
});

document.getElementById('btn-quiz-next').addEventListener('click', () => {
  quizIndex = (quizIndex + 1) % QUIZ.length;
  updateQuizUI();
});
document.getElementById('btn-quiz-reset').addEventListener('click', () => {
  quizIndex = 0;
  quizScore = 0;
  answeredSet.clear();
  updateQuizUI();
});

document.getElementById('btn-guide-play').addEventListener('click', toggleGuideAutoPlay);
document.getElementById('btn-guide-prev').addEventListener('click', () => {
  stopGuideAutoPlay();
  applyGuideStep(guideIdx - 1, true);
});
document.getElementById('btn-guide-next').addEventListener('click', () => {
  stopGuideAutoPlay();
  applyGuideStep(guideIdx + 1, true);
});
document.getElementById('btn-guide-tip').addEventListener('click', () => {
  stopGuideAutoPlay();
  applyGuideStep(guideIdx + 1, true);
});

const guideToggleBtn = document.getElementById('btn-guide-toggle');
guideToggleBtn.addEventListener('click', () => {
  const card = document.getElementById('guide-card');
  const hidden = card.classList.toggle('hidden');
  beaconMesh.visible = !hidden;
  guideToggleBtn.classList.toggle('active', !hidden);
  guideToggleBtn.setAttribute('aria-pressed', String(!hidden));
  guideToggleBtn.textContent = hidden ? 'Guided Tour: Off' : 'Guided Tour: On';
  if (hidden) stopGuideAutoPlay();
  resizeRenderer();
});

// --- Raycaster Pointer Interaction ---
const raycaster = new THREE.Raycaster();
const pointer = new THREE.Vector2();
let downPos = { x: 0, y: 0 };

canvasEl.addEventListener('pointerdown', (e) => {
  downPos = { x: e.clientX, y: e.clientY };
});

canvasEl.addEventListener('pointerup', (e) => {
  if (Math.hypot(e.clientX - downPos.x, e.clientY - downPos.y) > 6) return;
  const rect = canvasEl.getBoundingClientRect();
  pointer.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
  pointer.y = -((e.clientY - rect.top) / rect.height) * 2 + 1;
  raycaster.setFromCamera(pointer, camera);
  const hits = raycaster.intersectObjects(pickables, false);
  if (hits.length > 0 && hits[0].object.userData.id) {
    handleStructureActivated(hits[0].object.userData.id);
  }
});

function resizeRenderer() {
  const w = stageEl.clientWidth || 800;
  const h = stageEl.clientHeight || 600;
  renderer.setSize(w, h, false);
  camera.aspect = w / h;
  camera.updateProjectionMatrix();
}
window.addEventListener('resize', resizeRenderer);
resizeRenderer();

// --- Initialize & Animate Loop ---
setViewMode('everyday', false);
applyGuideStep(0, false);
updateQuizUI();

const projVec = new THREE.Vector3();
function projectTag(pos, yLift, tag, w, h) {
  projVec.copy(pos);
  projVec.y += yLift;
  projVec.project(camera);
  if (projVec.z > 1) {
    tag.style.display = 'none';
    return;
  }
  tag.style.display = 'inline-flex';
  tag.style.left = (projVec.x * 0.5 + 0.5) * w + 'px';
  tag.style.top = (-projVec.y * 0.5 + 0.5) * h + 'px';
}

function renderFrame(now) {
  requestAnimationFrame(renderFrame);
  const t = now * 0.001;

  if (cameraLerpFrames > 0) {
    camera.position.lerp(cameraGoalPos, 0.12);
    controls.target.lerp(cameraGoalTarget, 0.12);
    cameraLerpFrames--;
  }
  controls.update();

  const nextId = GUIDE_STEPS[guideIdx].nextTarget;
  const nextEntry = meshMap.get(nextId);
  if (nextEntry && beaconMesh.visible) {
    beaconMesh.position.copy(nextEntry.group.position);
    beaconMesh.position.y -= 1.05;
    const pulse = 1 + Math.sin(t * 4.2) * 0.18;
    beaconMesh.scale.set(pulse, pulse, 1);
    beaconMesh.material.opacity = 0.45 + Math.sin(t * 4.2) * 0.3;
  }

  const w = stageEl.clientWidth;
  const h = stageEl.clientHeight;

  STRUCTURES.forEach((s, idx) => {
    const entry = meshMap.get(s.id);
    entry.group.position.lerp(entry.targetPos, 0.1);
    entry.mesh.rotation.y = Math.sin(t * 0.7 + idx) * 0.12;
    if (entry.group.userData.rings) {
      const [r0, r1, r2] = entry.group.userData.rings;
      r0.rotation.z = t * 0.45;
      r1.rotation.x = Math.PI / 3 + t * 0.65;
      r2.rotation.y = Math.PI / 3 - t * 0.85;
    }
    projectTag(entry.group.position, s.size[1] * 0.75 + 0.55, tagMap.get(s.id), w, h);
  });

  fleetEntries.forEach(({ group, tag }, idx) => {
    group.rotation.y = Math.sin(t * 0.8 + idx) * 0.15;
    if (group.userData.halo) group.userData.halo.rotation.z = t * 0.9;
    projectTag(group.position, 1.45, tag, w, h);
  });

  for (let i = activeParticles.length - 1; i >= 0; i--) {
    const p = activeParticles[i];
    p.t += p.speed;
    if (p.t >= 1) {
      scene.remove(p.mesh);
      p.mesh.geometry.dispose();
      p.mesh.material.dispose();
      activeParticles.splice(i, 1);
    } else {
      p.mesh.position.copy(p.curve.getPointAt(p.t));
    }
  }

  renderer.render(scene, camera);
}
requestAnimationFrame(renderFrame);
