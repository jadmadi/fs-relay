// Static reference data, multi-machine everyday topology, quiz, and 8-step business + OS guided tour.

export const STRUCTURES = [
  {
    id: 'mount-a',
    code: 'MNT-01',
    name: 'Dev Laptop (/mnt/project)',
    system: 'transport',
    color: 0xf43f5e,
    pos: { everyday: [-7.8, 2.4, 2.2], topology: [-8.2, 2.4, 2.2], cutaway: [-9.5, 3.0, 2.5] },
    geom: 'box',
    size: [2.3, 1.35, 1.6],
    tagline: 'Business: Edit shared files from your laptop with 0 GB copied locally. OS: User-space FUSE 3 mount.',
    overview: 'Your laptop mounts the remote folder at /mnt/project without downloading the full tree. Runs fs-relay-bind as a normal user (no root); clean metadata hits return from RAM until another machine writes.',
    specs: [
      ['Practical benefit', '0 GB local disk used; instant file access'],
      ['Process', 'fs-relay-bind (unprivileged user, no root)'],
      ['Kernel bridge', '/dev/fuse (FUSE 3 protocol)'],
      ['Session ID', 'UUIDv7 assigned on WebSocket handshake']
    ],
    guarantees: [
      'Local stat() and readdir() hits return in sub-millisecond time while no peer machine mutates the file.',
      'When laptop Wi-Fi blips, open handles and SQLite locks survive inside the 8s reconnect grace window.'
    ],
    related: ['ws-mux', 'raw-splice', 'mount-b']
  },
  {
    id: 'mount-b',
    code: 'MNT-02',
    name: 'Office Desktop & Cloud VM (/mnt/project)',
    system: 'coherence',
    color: 0x38bdf8,
    pos: { everyday: [-7.8, -2.2, -1.8], topology: [-8.2, -2.2, -2.0], cutaway: [-9.5, -2.6, -2.2] },
    geom: 'box',
    size: [2.3, 1.35, 1.6],
    tagline: 'Business: Second & third computers sharing the same live folder. OS: Broadcast cache-invalidation peer.',
    overview: 'Your office desktop and cloud build VMs mount the exact same server folder simultaneously. Whenever your laptop saves a file, these machines receive an unsolicited push frame and evict stale cache entries.',
    specs: [
      ['Practical benefit', 'No Dropbox sync delay; zero conflicted copies'],
      ['Invalidation path', 'Unsolicited WS frame (id == 0)'],
      ['Eviction target', 'Kernel dentry + page cache for target Ino'],
      ['Lock contention', 'Waits asynchronously on server lock channel']
    ],
    guarantees: [
      'Never serves stale directory entries after your laptop or another machine finishes a write.',
      'Blocked fcntl(F_SETLKW) calls hold zero mutexes while awaiting lock release.'
    ],
    related: ['hub-fanout', 'lock-table', 'ws-mux']
  },
  {
    id: 'ws-mux',
    code: 'TRN-01',
    name: 'Persistent WebSocket Multiplexer',
    system: 'transport',
    color: 0xf43f5e,
    pos: { everyday: [-1.2, 0.4, 0.4], topology: [-4.2, 1.6, 0.8], cutaway: [-5.2, 2.2, 1.0] },
    geom: 'cylinder',
    size: [0.95, 0.95, 2.2],
    tagline: 'Business: Works over plain HTTP/HTTPS on any port without VPN or NFS firewall rules.',
    overview: 'Carries all 26 FUSE operations over one upgraded HTTP connection. Requests multiplex out-of-order by correlation ID; server push frames use ID 0.',
    specs: [
      ['Endpoint', 'GET /ws?ns=<namespace> (HTTP Upgrade)'],
      ['Framing', 'Binary FlatBuffers Request / Response'],
      ['Multiplexing', 'Concurrent in-flight calls keyed by uint64 id'],
      ['Server push', 'id = 0 for cache invalidation & lock wakeups']
    ],
    guarantees: [
      'Zero TCP/TLS handshake cost after mount startup.',
      'Interleaves fast stat/lookup calls alongside blocking lock waits without head-of-line blocking.'
    ],
    related: ['mount-a', 'wire-codec', 'post-join']
  },
  {
    id: 'post-join',
    code: 'TRN-02',
    name: '8s Session Hold & POST Fallback',
    system: 'transport',
    color: 0xf59e0b,
    pos: { everyday: [-1.2, -1.8, 1.4], topology: [-4.2, -2.1, 1.8], cutaway: [-5.2, -2.4, 2.0] },
    geom: 'octa',
    size: [1.15, 1.15, 1.15],
    tagline: 'Business: Laptop Wi-Fi blips do not crash your build or corrupt open files.',
    overview: 'When a WebSocket drops, the server freezes the session handle table and POSIX locks for 8 seconds instead of tearing them down. Reconnecting with X-FS-Relay-Session reclaims state.',
    specs: [
      ['Header', 'X-FS-Relay-Session: <UUIDv7>'],
      ['Hold window', '8.0 seconds before handle & lock sweep'],
      ['Fallback RPC', 'POST /rpc (stateless unary FlatBuffers)'],
      ['Recovery', 'Seamless re-bind of open FHs and active locks']
    ],
    guarantees: [
      'A transient Wi-Fi or switch flap under 8 seconds never drops an in-flight SQLite transaction.',
      'Expired sessions deterministically release held byte-range locks and wake queued contenders.'
    ],
    related: ['ws-mux', 'lock-table', 'wire-codec']
  },
  {
    id: 'wire-codec',
    code: 'WIR-01',
    name: 'FlatBuffers Zero-Allocation Codec',
    system: 'transport',
    color: 0xf43f5e,
    pos: { everyday: [0.4, 0.8, -0.4], topology: [-0.6, 1.5, 0.0], cutaway: [-1.8, 2.2, 0.0] },
    geom: 'box',
    size: [1.9, 1.5, 1.9],
    tagline: 'Business: Keeps CPU usage low and tree walks 10x faster than XML/JSON WebDAV.',
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
  }
];

STRUCTURES.push(
  {
    id: 'hub-fanout',
    code: 'COH-01',
    name: 'Namespace Fan-out Hub',
    system: 'coherence',
    color: 0x38bdf8,
    pos: { everyday: [0.4, -1.2, -1.2], topology: [0.4, -1.8, -1.5], cutaway: [0.2, -2.2, -1.8] },
    geom: 'torus',
    size: [1.15, 0.32, 24],
    tagline: 'Business: Keeps every connected computer viewing the exact same up-to-date files.',
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
    pos: { everyday: [2.2, 0.9, 0.8], topology: [3.2, 1.8, 1.6], cutaway: [2.2, 0.6, 1.2] },
    geom: 'lockrings',
    size: [1.55, 0.22, 32],
    tagline: 'Business: Lets multiple computers safely share a live SQLite database without corruption.',
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
    pos: { everyday: [1.2, -0.4, 2.4], topology: [1.2, -0.1, 3.2], cutaway: [0.5, -0.4, 3.6] },
    geom: 'cylinder',
    size: [0.65, 0.65, 5.8],
    tagline: 'Business: Streams large video, dataset, or archive files at full line rate.',
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
    pos: { everyday: [4.4, 0.3, -0.6], topology: [5.2, 0.4, -1.2], cutaway: [5.8, 0.8, -1.0] },
    geom: 'box',
    size: [2.1, 1.8, 1.8],
    tagline: 'Business: Strict security boundary isolating each shared project namespace on the host.',
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
    name: 'One Shared Host Folder (data/project/)',
    system: 'storage',
    color: 0x94a3b8,
    pos: { everyday: [7.8, -0.2, 0.2], topology: [8.4, -0.4, 0.4], cutaway: [9.4, -0.4, 0.4] },
    geom: 'cylinder',
    size: [1.45, 1.45, 2.4],
    tagline: 'Business: One real directory on the server; zero proprietary lock-in or duplicate copies.',
    overview: 'All connected laptops, desktops, and VMs read and write this single plain POSIX directory on the server host. Backup tools, git, and local server processes can inspect the files directly.',
    specs: [
      ['Storage model', '1 live folder on server; 0 full copies on clients'],
      ['Format', 'Native host filesystem under data/<namespace>/'],
      ['Benchmark edge', '10.6x faster than WebDAV on git status / tree walks'],
      ['Dependencies', 'Zero root daemon, zero kernel NFS exports']
    ],
    guarantees: [
      'Stopping fs-relay leaves a clean, directly readable directory tree on the host.',
      'Atomic rename and fsync semantics map 1:1 to the underlying host filesystem.'
    ],
    related: ['inode-map', 'raw-splice']
  }
);

export const EDGES = [
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

export const QUIZ = [
  { q: 'A 4 MiB file read arrives from your Dev Laptop. Which structure bypasses FlatBuffers framing to stream raw bytes above 64 KiB?', target: 'raw-splice', hint: 'Look for the purple bulk bypass cylinder connecting the client directly toward the shared folder.' },
  { q: 'Your Laptop saves config.db. Which structure broadcasts an unsolicited invalidation frame (id == 0) to your Office Desktop & Cloud VM?', target: 'hub-fanout', hint: 'Inspect the cyan torus responsible for per-namespace cache coherence fan-out.' },
  { q: 'Your Laptop Wi-Fi drops for 3 seconds mid-transaction. Which structure preserves open file handles and POSIX locks for 8 seconds?', target: 'post-join', hint: 'Check the amber octahedron handling UUIDv7 session continuity and HTTP POST fallback.' },
  { q: 'Two computers run concurrent SQLite transactions. Which structure coordinates PENDING, RESERVED, and EXCLUSIVE byte-range intervals without holding a mutex while waiting?', target: 'lock-table', hint: 'Find the emerald concentric ring assembly representing byte-range lock intervals.' },
  { q: 'Which structure enforces the single-function path sandbox under data/<namespace>/ and refuses ../ traversal before any disk syscall runs?', target: 'inode-map', hint: 'Select the amber storage gateway right in front of the Shared Host Folder.' },
  { q: 'Which component decodes incoming RPC envelopes directly via vtable offsets with zero net/http or os imports?', target: 'wire-codec', hint: 'Locate the central crimson cube between the WebSocket multiplexer and the server subsystems.' },
  { q: 'Which peer mount node represents your Office Desktop and Cloud VM receiving instant cache eviction pushes?', target: 'mount-b', hint: 'Select the lower-left cyan client mount node (MNT-02).' },
  { q: 'Where do all shared files live as a single copy of ordinary POSIX files with zero client disk duplication?', target: 'disk-store', hint: 'Click the rightmost slate cylinder representing the One Shared Host Folder (DSK-01).' }
];

export const GUIDE_STEPS = [
  {
    badge: 'STEP 0 OF 7 · BUSINESS & WORKFLOW LAYER',
    title: '0. One Live Folder Across Laptop, Desktop & Cloud VM (Zero Copies)',
    body: 'Instead of syncing duplicate copies of a folder to every computer, your Dev Laptop, Office Desktop, and Cloud VM all mount one shared directory over plain HTTP. Zero gigabytes are duplicated on client disks.',
    view: 'everyday',
    focus: 'disk-store',
    nextTarget: 'mount-a',
    cam: [[0, 7.8, 22.0], [0, 0, 0]],
    packets: [['mount-a', 'ws-mux', 0xf43f5e], ['ws-mux', 'disk-store', 0xf43f5e], ['mount-b', 'ws-mux', 0x38bdf8], ['ws-mux', 'disk-store', 0x38bdf8]],
    tip: 'Click MNT-01 (Dev Laptop) or click here to see what happens when you save a file on your laptop.'
  },
  {
    badge: 'STEP 1 OF 7 · BUSINESS & WORKFLOW LAYER',
    title: '1. Save on Your Laptop, See It Immediately on Your Desktop',
    body: 'When you save a file on your Laptop, it writes straight to the single host directory and notifies your Office Desktop and Cloud VM in milliseconds. Nobody waits for background file sync or deals with "conflicted copy" files.',
    view: 'everyday',
    focus: 'mount-a',
    nextTarget: 'mount-b',
    cam: [[-2.4, 6.8, 19.5], [-1.0, 0.2, 0.2]],
    packets: [['mount-a', 'ws-mux', 0xf43f5e], ['ws-mux', 'disk-store', 0xf59e0b], ['hub-fanout', 'mount-b', 0x38bdf8]],
    tip: 'Click MNT-02 (Office Desktop & Cloud VM) or click here to see how shared SQLite databases stay safe.'
  },
  {
    badge: 'STEP 2 OF 7 · BUSINESS & WORKFLOW LAYER',
    title: '2. Two Commands, No Root, 10x Faster Than WebDAV',
    body: 'Sharing a directory usually requires root-run NFS/SMB daemons and firewall rules. fs-relay runs as a normal user over any HTTP port, coordinates SQLite locks across machines, and runs git status 10.6x faster than WebDAV.',
    view: 'everyday',
    focus: 'mount-b',
    nextTarget: 'ws-mux',
    cam: [[0.5, 6.8, 19.5], [0.5, 0.0, 0.0]],
    packets: [['mount-b', 'ws-mux', 0x38bdf8], ['ws-mux', 'lock-table', 0x34d399], ['lock-table', 'disk-store', 0x34d399]],
    tip: 'Ready for the OS internals? Click TRN-01 or click here to zoom into the Technical Protocol Layer →'
  },
  {
    badge: 'STEP 3 OF 7 · TECHNICAL OS LAYER',
    title: '3. Persistent WebSocket Multiplexer & FlatBuffers Decode',
    body: 'Under the hood, fs-relay-bind upgrades one HTTP connection into a full-duplex WebSocket (TRN-01). Small metadata calls (<= 64 KiB) decode directly via FlatBuffers vtable offsets (WIR-01) with zero heap allocations.',
    view: 'topology',
    focus: 'wire-codec',
    nextTarget: 'inode-map',
    cam: [[-2.2, 5.0, 13.5], [-2.0, 1.4, 0.4]],
    packets: [['mount-a', 'ws-mux', 0xf43f5e], ['ws-mux', 'wire-codec', 0xf43f5e], ['wire-codec', 'inode-map', 0xf59e0b]],
    tip: 'Click LOC-01 (pulsing badge) or click here to inspect the namespace path sandbox and disk write.'
  },
  {
    badge: 'STEP 4 OF 7 · TECHNICAL OS LAYER',
    title: '4. Sandboxed Path Guard & Unsolicited Cache Invalidation',
    body: 'internal/local (LOC-01) verifies every path stays inside data/<namespace>/, releases table locks before disk I/O, and triggers the Fan-out Hub (COH-01) to push an invalidation frame (id == 0) to peer mounts.',
    view: 'topology',
    focus: 'hub-fanout',
    nextTarget: 'lock-table',
    cam: [[1.2, 4.8, 14.2], [1.0, -0.4, -0.8]],
    packets: [['inode-map', 'disk-store', 0x94a3b8], ['wire-codec', 'hub-fanout', 0x38bdf8], ['hub-fanout', 'mount-b', 0x38bdf8]],
    tip: 'Click LCK-01 (pulsing emerald rings) or click here to inspect POSIX byte-range & SQLite locking.'
  },
  {
    badge: 'STEP 5 OF 7 · TECHNICAL OS LAYER',
    title: '5. Server-Coordinated POSIX & SQLite Byte-Range Locks',
    body: 'The Lock Table (LCK-01) tracks PENDING, RESERVED, and EXCLUSIVE byte intervals per (Session UUIDv7, lk_owner). Contending fcntl(F_SETLKW) calls park on a Go channel holding zero mutexes.',
    view: 'cutaway',
    focus: 'lock-table',
    nextTarget: 'raw-splice',
    cam: [[2.2, 4.2, 11.8], [2.2, 0.6, 1.0]],
    packets: [['mount-a', 'ws-mux', 0x34d399], ['ws-mux', 'wire-codec', 0x34d399], ['wire-codec', 'lock-table', 0x34d399]],
    tip: 'Click SPL-01 (pulsing purple cylinder) or click here to see how bulk payloads above 64 KiB stream.'
  },
  {
    badge: 'STEP 6 OF 7 · TECHNICAL OS LAYER',
    title: '6. > 64 KiB Zero-Copy HTTP Raw Splice Lane',
    body: 'Reads and writes larger than 64 KiB bypass FlatBuffers framing completely and stream over /raw (SPL-01) with compact X-FS-Relay-* headers straight between the TCP socket and host file descriptor.',
    view: 'topology',
    focus: 'raw-splice',
    nextTarget: 'post-join',
    cam: [[1.2, 4.0, 14.0], [1.2, -0.2, 2.0]],
    packets: [['mount-a', 'raw-splice', 0xa78bfa], ['raw-splice', 'disk-store', 0xa78bfa]],
    tip: 'Click TRN-02 (pulsing amber octahedron) or click here to test the 8-second disconnect grace window.'
  },
  {
    badge: 'STEP 7 OF 7 · TECHNICAL OS LAYER',
    title: '7. 8-Second Session Hold & Seamless Rejoin',
    body: 'When a WebSocket drops, the server holds open file handles and byte-range locks for 8 seconds (TRN-02). Reconnecting with X-FS-Relay-Session reclaims the session without dropping in-flight operations.',
    view: 'topology',
    focus: 'post-join',
    nextTarget: 'disk-store',
    cam: [[-3.5, 4.2, 13.5], [-3.5, -1.2, 1.2]],
    packets: [['mount-a', 'post-join', 0xf59e0b], ['post-join', 'wire-codec', 0xf59e0b], ['wire-codec', 'lock-table', 0x34d399]],
    tip: 'Tour complete! Click here to return to Step 0 (Everyday View) or try 3 · Self-Test Quiz above.'
  }
];
