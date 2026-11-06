// fs-relay 3D Protocol & Topology Lab (Three.js + Claude Interactive Diagram Pattern)
import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';

const STRUCTURES = [
  {
    id: 'mount-a',
    code: 'MNT-01',
    name: 'Client Mount A (/mnt/work-a)',
    system: 'transport',
    color: 0xf43f5e,
    pos: { topology: [-8.2, 2.4, 2.2], cutaway: [-9.5, 3.0, 2.5] },
    geom: 'box',
    size: [2.3, 1.35, 1.6],
    tagline: 'Primary user-space FUSE 3 mount with in-process attribute & directory caches.',
    overview: 'Runs fs-relay-bind as an unprivileged user process. The Linux kernel VFS talks to /dev/fuse; clean metadata hits stay in-process until an invalidation frame arrives.',
    specs: [
      ['Process', 'fs-relay-bind (no root)'],
      ['Kernel bridge', '/dev/fuse (FUSE 3 protocol)'],
      ['Local cache', 'Inode attr + dentry cache (invalidated on push)'],
      ['Session ID', 'UUIDv7 assigned on WebSocket handshake']
    ],
    guarantees: [
      'Local stat() and readdir() hits return in sub-millisecond time while no remote writer mutates the inode.',
      'When disconnected, open handles and POSIX byte-range locks survive inside the 8s reconnect grace window.'
    ],
    related: ['ws-mux', 'raw-splice', 'mount-b']
  },
  {
    id: 'mount-b',
    code: 'MNT-02',
    name: 'Client Mount B (/mnt/work-b)',
    system: 'coherence',
    color: 0x38bdf8,
    pos: { topology: [-8.2, -2.2, -2.0], cutaway: [-9.5, -2.6, -2.2] },
    geom: 'box',
    size: [2.3, 1.35, 1.6],
    tagline: 'Peer FUSE 3 mount subscribed to broadcast invalidations and lock wakeups.',
    overview: 'Second machine mounting the same namespace. Receives unsolicited FlatBuffers invalidation frames over its WebSocket whenever Mount A mutates shared inodes.',
    specs: [
      ['Peer role', 'Concurrent reader / contender'],
      ['Invalidation path', 'Unsolicited WS frame (id == 0)'],
      ['Eviction target', 'Kernel dentry + page cache for target Ino'],
      ['Lock contention', 'Waits asynchronously on server lock channel']
    ],
    guarantees: [
      'Never serves stale directory entries after a peer write completes and broadcasts.',
      'Blocked fcntl(F_SETLKW) calls hold zero mutexes while awaiting release notification.'
    ],
    related: ['hub-fanout', 'lock-table', 'ws-mux']
  },
  {
    id: 'ws-mux',
    code: 'TRN-01',
    name: 'Persistent WebSocket Multiplexer',
    system: 'transport',
    color: 0xf43f5e,
    pos: { topology: [-4.2, 1.6, 0.8], cutaway: [-5.2, 2.2, 1.0] },
    geom: 'cylinder',
    size: [0.95, 0.95, 2.2],
    tagline: 'Full-duplex binary FlatBuffers pipe for metadata RPCs and server push.',
    overview: 'Carries all 26 FUSE operations over a single upgraded HTTP connection. Requests multiplex out-of-order by correlation ID; server push frames use ID 0.',
    specs: [
      ['Endpoint', 'GET /ws?ns=<namespace> (HTTP Upgrade)'],
      ['Framing', 'Binary FlatBuffers Request / Response'],
      ['Multiplexing', 'Concurrent in-flight calls keyed by uint64 id'],
      ['Server push', 'id = 0 for cache invalidation & lock wakeups']
    ],
    guarantees: [
      'Zero TCP/TLS handshake cost after mount startup.',
      'Interleaves fast stat/lookup calls alongside long blocking lock waits without head-of-line blocking.'
    ],
    related: ['mount-a', 'wire-codec', 'post-join']
  },
  {
    id: 'post-join',
    code: 'TRN-02',
    name: '8s Session Hold & POST Fallback',
    system: 'transport',
    color: 0xf59e0b,
    pos: { topology: [-4.2, -2.1, 1.8], cutaway: [-5.2, -2.4, 2.0] },
    geom: 'octa',
    size: [1.15, 1.15, 1.15],
    tagline: 'UUIDv7 session continuity across network blips and stateless HTTP POST.',
    overview: 'When a WebSocket drops, the server freezes the session handle table and POSIX locks for 8 seconds instead of tearing them down. Reconnecting with X-FS-Relay-Session reclaims state.',
    specs: [
      ['Header', 'X-FS-Relay-Session: <UUIDv7>'],
      ['Hold window', '8.0 seconds before handle & lock sweep'],
      ['Fallback RPC', 'POST /rpc (stateless unary FlatBuffers)'],
      ['Recovery', 'Seamless re-bind of open FHs and active locks']
    ],
    guarantees: [
      'A transient Wi-Fi or switch flap under 8 seconds never drops an in-flight SQLite transaction or open file descriptor.',
      'Expired sessions deterministically release all held byte-range locks and wake queued contenders.'
    ],
    related: ['ws-mux', 'lock-table', 'wire-codec']
  },
  {
    id: 'wire-codec',
    code: 'WIR-01',
    name: 'FlatBuffers Zero-Allocation Codec',
    system: 'transport',
    color: 0xf43f5e,
    pos: { topology: [-0.6, 1.5, 0.0], cutaway: [-1.8, 2.2, 0.0] },
    geom: 'box',
    size: [1.9, 1.5, 1.9],
    tagline: 'Direct vtable offset reads in internal/wire with zero reflection or JSON parsing.',
    overview: 'Decodes incoming binary envelopes directly from the socket buffer via FlatBuffers vtable offsets. Dispatches via a closed 26-entry method switch table.',
    specs: [
      ['Schema', 'internal/wire/fsrelay.fbs'],
      ['Imports', 'Zero net/http, zero os, zero go-fuse imports'],
      ['Error model', 'Single errno table -> wire error.code'],
      ['Threshold', 'Payloads <= 64 KiB stay inline in FlatBuffers']
    ],
    guarantees: [
      'Zero heap allocations for field decoding during hot metadata lookups.',
      'Strict package boundary: internal/wire never touches OS files or HTTP headers.'
    ],
    related: ['ws-mux', 'hub-fanout', 'inode-map']
  },
  {
    id: 'hub-fanout',
    code: 'COH-01',
    name: 'Namespace Fan-out Hub',
    system: 'coherence',
    color: 0x38bdf8,
    pos: { topology: [0.4, -1.8, -1.5], cutaway: [0.2, -2.2, -1.8] },
    geom: 'torus',
    size: [1.15, 0.32, 24],
    tagline: 'Broadcasts targeted inode/dentry invalidations to every other connected mount.',
    overview: 'Tracks active WebSocket sessions per namespace. When any mutating call (Write, Unlink, Rename, Setattr) succeeds, the hub fans out an invalidation frame to all peers except the writer.',
    specs: [
      ['Scope', 'Per-namespace subscriber registry'],
      ['Trigger ops', 'Write, Create, Unlink, Rename, Mkdir, Setattr'],
      ['Filter rule', 'Sender session excluded from echo'],
      ['Payload', 'Target Ino + parent dir Ino + entry name']
    ],
    guarantees: [
      'Peers evict only the mutated inode instead of flushing their entire mount cache.',
      'Writer mount keeps its warm cache because it originated the mutation.'
    ],
    related: ['mount-b', 'wire-codec', 'inode-map']
  },
  {
    id: 'lock-table',
    code: 'LCK-01',
    name: 'POSIX Byte-Range & SQLite Lock Table',
    system: 'locks',
    color: 0x34d399,
    pos: { topology: [3.2, 1.8, 1.6], cutaway: [2.2, 0.6, 1.2] },
    geom: 'lockrings',
    size: [1.55, 0.22, 32],
    tagline: 'Server-coordinated fcntl byte-range intervals (PENDING, RESERVED, SHARED/EXCLUSIVE).',
    overview: 'Coordinates POSIX advisory locks across remote machines so multi-node SQLite and flock/fcntl workloads serialize cleanly without corrupting shared journals.',
    specs: [
      ['Ops', 'Getlk, Setlk, Setlkw (blocking wait)'],
      ['SQLite ranges', 'PENDING (0x40000000), RESERVED (+1), SHARED (+2..510)'],
      ['Wait discipline', 'Waiter parks on channel; holds zero mutexes'],
      ['Owner key', '(Session UUIDv7, FUSE lk_owner uint64)']
    ],
    guarantees: [
      'Overlapping exclusive byte ranges strictly serialize across all connected mounts.',
      'A blocked Setlkw waiter never blocks unrelated metadata or I/O calls on the server.'
    ],
    related: ['post-join', 'inode-map', 'mount-b']
  },
  {
    id: 'raw-splice',
    code: 'SPL-01',
    name: 'HTTP Raw Bulk Splice Lane (> 64 KiB)',
    system: 'raw',
    color: 0xa78bfa,
    pos: { topology: [1.2, -0.1, 3.2], cutaway: [0.5, -0.4, 3.6] },
    geom: 'cylinder',
    size: [0.65, 0.65, 5.8],
    tagline: 'Bypasses FlatBuffers framing for bulk reads and writes above 64 KiB.',
    overview: 'Large reads and writes switch to raw HTTP streaming (/raw) with compact X-FS-Relay-* headers, copying bytes straight between the TCP socket and the local file descriptor.',
    specs: [
      ['Crossover', 'Payloads > 64 KiB (configurable threshold)'],
      ['Headers', 'X-FS-Relay-Ino, X-FS-Relay-Fh, X-FS-Relay-Off'],
      ['Framing overhead', '0 bytes of FlatBuffers envelope around bulk data'],
      ['Throughput', 'Saturates gigabit/loopback pipes without double buffering']
    ],
    guarantees: [
      'Eliminates base64 or nested byte-vector copying on multi-megabyte reads and writes.',
      'Coexists with the WebSocket lane: metadata stays on WS while bulk streams over HTTP.'
    ],
    related: ['mount-a', 'disk-store', 'ws-mux']
  },
  {
    id: 'inode-map',
    code: 'LOC-01',
    name: 'internal/local Inode & Sandbox Guard',
    system: 'storage',
    color: 0xf59e0b,
    pos: { topology: [5.2, 0.4, -1.2], cutaway: [5.8, 0.8, -1.0] },
    geom: 'box',
    size: [2.1, 1.8, 1.8],
    tagline: 'Single-function path resolver refusing ../ escape + 64-bit inode & handle tables.',
    overview: 'The only package allowed to touch os and unix syscalls. Resolves every path beneath data/<namespace>/ in a single hardened function and maps stable uint64 inodes and file handles.',
    specs: [
      ['Sandbox root', 'data/<namespace>/ (strict lexical + openat guard)'],
      ['State tables', 'Inode map + Open file handle table'],
      ['Locking rule', 'Table mutex never held across disk I/O'],
      ['Error output', 'Returns wrapped syscall.Errno values']
    ],
    guarantees: [
      'Any path traversal attempt (../ or symlink escape outside namespace) returns EACCES/EINVAL before touching disk.',
      'Inode table locks are released before pread/pwrite syscalls execute.'
    ],
    related: ['wire-codec', 'lock-table', 'disk-store']
  },
  {
    id: 'disk-store',
    code: 'DSK-01',
    name: 'Host Backing Directory (POSIX + xattr)',
    system: 'storage',
    color: 0x94a3b8,
    pos: { topology: [8.4, -0.4, 0.4], cutaway: [9.4, -0.4, 0.4] },
    geom: 'cylinder',
    size: [1.45, 1.45, 2.4],
    tagline: 'Plain host directory storing real files, permissions, symlinks, and extended attributes.',
    overview: 'No proprietary block format or object chunking. Files live as ordinary POSIX files on the server host disk so backup tools, git, and local processes can inspect them directly.',
    specs: [
      ['Format', 'Native host filesystem under data/<namespace>/'],
      ['POSIX surface', 'pread, pwrite, fsync, rename, symlink, xattr'],
      ['Benchmark edge', '10.6x faster than WebDAV on git status / tree walks'],
      ['Dependencies', 'Zero root daemon, zero kernel NFS exports']
    ],
    guarantees: [
      'Stopping fs-relay leaves a clean, directly readable directory tree on the host.',
      'Atomic rename and fsync semantics map 1:1 to the underlying host filesystem.'
    ],
    related: ['inode-map', 'raw-splice']
  }
];

const EDGES = [
  { from: 'mount-a', to: 'ws-mux', system: 'transport', color: 0xf43f5e },
  { from: 'mount-b', to: 'ws-mux', system: 'transport', color: 0x38bdf8 },
  { from: 'mount-a', to: 'post-join', system: 'transport', color: 0xf59e0b },
  { from: 'post-join', to: 'wire-codec', system: 'transport', color: 0xf59e0b },
  { from: 'ws-mux', to: 'wire-codec', system: 'transport', color: 0xf43f5e },
  { from: 'wire-codec', to: 'hub-fanout', system: 'coherence', color: 0x38bdf8 },
  { from: 'hub-fanout', to: 'mount-b', system: 'coherence', color: 0x38bdf8 },
  { from: 'wire-codec', to: 'lock-table', system: 'locks', color: 0x34d399 },
  { from: 'lock-table', to: 'inode-map', system: 'locks', color: 0x34d399 },
  { from: 'wire-codec', to: 'inode-map', system: 'storage', color: 0xf59e0b },
  { from: 'inode-map', to: 'disk-store', system: 'storage', color: 0x94a3b8 },
  { from: 'mount-a', to: 'raw-splice', system: 'raw', color: 0xa78bfa },
  { from: 'raw-splice', to: 'disk-store', system: 'raw', color: 0xa78bfa }
];

const QUIZ = [
  {
    q: 'A 4 MiB file read arrives from Mount A. Which structure bypasses FlatBuffers framing to stream raw bytes above 64 KiB?',
    target: 'raw-splice',
    hint: 'Look for the purple bulk bypass cylinder connecting Mount A directly toward the backing store.'
  },
  {
    q: 'Mount A writes to shared file config.db. Which structure broadcasts an unsolicited invalidation frame (id == 0) to Mount B?',
    target: 'hub-fanout',
    hint: 'Inspect the cyan torus responsible for per-namespace cache coherence fan-out.'
  },
  {
    q: 'A laptop Wi-Fi connection drops for 3 seconds mid-transaction. Which structure preserves open file handles and POSIX locks for 8 seconds?',
    target: 'post-join',
    hint: 'Check the amber octahedron handling UUIDv7 session continuity and HTTP POST fallback.'
  },
  {
    q: 'Two nodes run concurrent SQLite transactions. Which structure coordinates PENDING, RESERVED, and EXCLUSIVE byte-range intervals without holding a mutex while waiting?',
    target: 'lock-table',
    hint: 'Find the emerald concentric ring assembly representing byte-range lock intervals.'
  },
  {
    q: 'Which structure enforces the single-function path sandbox under data/<namespace>/ and refuses ../ traversal before any disk syscall runs?',
    target: 'inode-map',
    hint: 'Select the amber storage gateway right in front of the Host Backing Directory.'
  },
  {
    q: 'Which component decodes incoming RPC envelopes directly via vtable offsets with zero net/http or os imports?',
    target: 'wire-codec',
    hint: 'Locate the central crimson cube between the WebSocket multiplexer and the server subsystems.'
  },
  {
    q: 'Which peer client mount receives unsolicited cache eviction pushes and waits on contended byte-range locks?',
    target: 'mount-b',
    hint: 'Select the lower-left cyan client mount node.'
  },
  {
    q: 'Where do files finally rest as ordinary POSIX files and extended attributes with zero proprietary chunking?',
    target: 'disk-store',
    hint: 'Click the rightmost slate cylinder representing the host backing directory.'
  }
];

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
const stageEl = document.querySelector('.viewport-wrap');
const canvasEl = document.getElementById('canvas3d');
const labelLayer = document.getElementById('label-layer');

const renderer = new THREE.WebGLRenderer({ canvas: canvasEl, antialias: true, alpha: true });
renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

const scene = new THREE.Scene();
const camera = new THREE.PerspectiveCamera(42, 1, 0.1, 200);
camera.position.set(0, 8.2, 22.8);

const controls = new OrbitControls(camera, canvasEl);
controls.enableDamping = true;
controls.dampingFactor = 0.06;
controls.maxPolarAngle = Math.PI * 0.82;
controls.minDistance = 6;
controls.maxDistance = 36;
controls.target.set(0, 0, 0);

// Subtle grid floor & lighting
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

// --- Build 3D Structures & 2D Floating Labels ---
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
  return new THREE.BoxGeometry(s.size[0], s.size[1], s.size[2]);
}

STRUCTURES.forEach((s) => {
  const group = new THREE.Group();
  group.position.set(...s.pos.topology);
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

  scene.add(group);
  meshMap.set(s.id, { group, mesh, mat, edges, targetPos: new THREE.Vector3(...s.pos.topology) });

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

// --- Build 3D Connecting Tubes ---
const activeParticles = [];

function rebuildTubes(activeSystem = 'all') {
  while (tubeGroup.children.length) {
    const c = tubeGroup.children.pop();
    c.geometry.dispose();
    c.material.dispose();
  }
  EDGES.forEach((e) => {
    const a = meshMap.get(e.from).targetPos;
    const b = meshMap.get(e.to).targetPos;
    const mid = a.clone().lerp(b, 0.5);
    mid.y += (e.system === 'splice' ? -0.6 : 0.55);
    const curve = new THREE.CatmullRomCurve3([a.clone(), mid, b.clone()]);
    const geom = new THREE.TubeGeometry(curve, 28, 0.065, 8, false);
    const match = activeSystem === 'all' || activeSystem === e.system;
    const mat = new THREE.MeshBasicMaterial({
      color: e.color,
      transparent: true,
      opacity: match ? 0.52 : 0.08
    });
    const tube = new THREE.Mesh(geom, mat);
    tube.userData = { curve, edge: e };
    tubeGroup.add(tube);
  });
}
rebuildTubes();

function spawnPacket(fromId, toId, colorHex, speed = 0.018) {
  const a = meshMap.get(fromId).group.position.clone();
  const b = meshMap.get(toId).group.position.clone();
  const mid = a.clone().lerp(b, 0.5);
  mid.y += 0.55;
  const curve = new THREE.CatmullRomCurve3([a, mid, b]);
  const mesh = new THREE.Mesh(
    new THREE.SphereGeometry(0.22, 14, 14),
    new THREE.MeshBasicMaterial({ color: colorHex })
  );
  scene.add(mesh);
  activeParticles.push({ mesh, curve, t: 0, speed });
}

// --- State & Camera Transitions ---
let currentView = 'topology';
let currentSystem = 'all';
let selectedId = 'wire-codec';
let cameraGoalPos = new THREE.Vector3(0, 8.2, 22.8);
let cameraGoalTarget = new THREE.Vector3(0, 0, 0);
let cameraLerpFrames = 0;

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
  }
}

// --- View Switcher, Subsystem Filters & Simulations ---
function setViewMode(view) {
  currentView = view;
  document.querySelectorAll('.view-switcher [data-view]').forEach((b) => {
    const active = b.dataset.view === view;
    b.classList.toggle('active', active);
    b.setAttribute('aria-pressed', String(active));
  });

  const posKey = view === 'cutaway' ? 'cutaway' : 'topology';
  STRUCTURES.forEach((s) => {
    meshMap.get(s.id).targetPos.set(...s.pos[posKey]);
  });
  setTimeout(() => rebuildTubes(currentSystem), 180);

  if (view === 'cutaway') {
    document.getElementById('hud-hover-text').textContent = 'Cutaway · Byte-range lock rings, FlatBuffers codec, and sandboxed inode resolution';
    document.getElementById('hud-telemetry').textContent = 'VIEW · CUTAWAY';
    moveCameraTo([1.8, 3.8, 11.5], [1.8, 0.5, 0.2], 50);
    selectStructure('lock-table');
    activateInspectorTab('overview');
  } else if (view === 'quiz') {
    document.getElementById('hud-hover-text').textContent = 'Self-Test Mode · Click the 3D structure or badge that answers the prompt';
    document.getElementById('hud-telemetry').textContent = 'VIEW · QUIZ (8 Qs)';
    moveCameraTo([0, 8.2, 20.5], [0, 0, 0], 45);
    activateInspectorTab('quiz');
    updateQuizUI();
  } else {
    document.getElementById('hud-hover-text').textContent = 'Drag to orbit · Scroll to zoom · Click any 3D structure';
    document.getElementById('hud-telemetry').textContent = 'IDLE · 10 structures online';
    moveCameraTo([0, 8.2, 22.8], [0, 0, 0], 45);
    activateInspectorTab('overview');
  }
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
    const entry = meshMap.get(s.id);
    entry.mat.opacity = match ? 0.9 : 0.16;
    tagMap.get(s.id).classList.toggle('dimmed', !match);
  });
  rebuildTubes(sys);
  playTone(460, 0.04, 'sine', 0.035);
}

function runSimulation(simType) {
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

// --- Bind DOM Controls ---
document.querySelectorAll('.view-switcher [data-view]').forEach((btn) => {
  btn.addEventListener('click', () => setViewMode(btn.dataset.view));
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
  setViewMode('topology');
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
selectStructure('wire-codec');
updateQuizUI();

const projVec = new THREE.Vector3();
function animate(now) {
  requestAnimationFrame(animate);
  const t = now * 0.001;

  if (cameraLerpFrames > 0) {
    camera.position.lerp(cameraGoalPos, 0.12);
    controls.target.lerp(cameraGoalTarget, 0.12);
    cameraLerpFrames--;
  }
  controls.update();

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

    projVec.copy(entry.group.position);
    projVec.y += s.size[1] * 0.75 + 0.45;
    projVec.project(camera);
    const tag = tagMap.get(s.id);
    if (projVec.z > 1) {
      tag.style.display = 'none';
    } else {
      tag.style.display = 'inline-flex';
      const x = (projVec.x * 0.5 + 0.5) * w;
      const y = (-projVec.y * 0.5 + 0.5) * h;
      tag.style.left = x + 'px';
      tag.style.top = y + 'px';
    }
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
requestAnimationFrame(animate);
