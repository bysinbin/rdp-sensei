// ========================================================
// Rdp Sensei - Multi-Session WebAssembly RDP Engine
// Zero external plugins/apps required. 
// Uses Go WASM client + Go WebSocket TCP proxy.
// ========================================================

class RDPSession {
  constructor(id, device, canvas, viewport) {
    this.id = id;
    this.device = device;
    this.canvas = canvas;
    this.viewport = viewport;
    this.ctx2d = canvas.getContext('2d');
    this.connected = false;
    this.status = 'connecting';
    this.statusMsg = 'Connecting...';
    this.pointerCache = new Map();
    this.audioCtx = null;
    this.audioNextAt = 0;
    this.clipboardSyncPromise = Promise.resolve();
  }

  initAudio() {
    try {
      if (this.audioCtx) {
        try { this.audioCtx.close(); } catch (_) {}
      }
      this.audioCtx = new (window.AudioContext || window.webkitAudioContext)({ latencyHint: 'playback' });
      this.audioNextAt = 0;
    } catch (e) {
      console.warn('AudioContext not available:', e);
    }
  }

  closeAudio() {
    if (this.audioCtx) {
      try { this.audioCtx.close(); } catch (_) {}
      this.audioCtx = null;
      this.audioNextAt = 0;
    }
  }
}

class RDPEngine {
  constructor() {
    this.go = null;
    this.wasmLoaded = false;
    this.sessions = new Map(); // sessionId -> RDPSession
    this.activeSessionId = null;
    this.onSessionStatusChange = null;
    this.onSessionClose = null;
    this.touchbarSource = null;
  }

  async init() {
    if (this.wasmLoaded) return;

    // Attach global callbacks expected by WASM module
    window.rdpOnReady = (sid) => this.handleReady(sid);
    window.rdpOnError = (sid, msg) => this.handleError(sid, msg);
    window.rdpOnClose = (sid) => this.handleClose(sid);
    window.rdpOnClipboard = (sid, text) => this.handleRemoteClipboard(sid, text);
    window.rdpAudioPlay = (sid, sr, ch, bits, data) => this.handleAudioPlay(sid, sr, ch, bits, data);
    window.rdpOnH264 = (sid, x, y, w, h, isKey, data) => this.handleH264(sid, x, y, w, h, isKey, data);
    window.rdpOnPointerHide = (sid) => this.handlePointerHide(sid);
    window.rdpOnPointerCached = (sid, idx) => this.handlePointerCached(sid, idx);
    window.rdpOnPointerUpdate = (sid, idx, bpp, hx, hy, w, h, andM, xorD) => 
      this.handlePointerUpdate(sid, idx, bpp, hx, hy, w, h, andM, xorD);

    this.bindGlobalKeyboard();
    this.initTouchBarSSE();
    window.addEventListener('resize', () => this.updateLayouts());

    if (typeof Go === 'undefined') {
      throw new Error('wasm_exec.js not loaded');
    }
    this.go = new Go();
    try {
      const resp = await fetch('/main.wasm');
      if (!resp.ok) throw new Error(`HTTP ${resp.status} fetching main.wasm`);
      const wasmBytes = await resp.arrayBuffer();
      const result = await WebAssembly.instantiate(wasmBytes, this.go.importObject);
      this.go.run(result.instance);
      this.wasmLoaded = true;
      console.log('✅ Go WASM Multi-Session RDP Engine initialized');
    } catch (err) {
      console.error('Failed to instantiate Go WASM engine:', err);
      throw err;
    }
  }

  initTouchBarSSE() {
    try {
      this.touchbarSource = new EventSource('/api/touchbar/events');
      this.touchbarSource.onmessage = (e) => {
        const key = e.data;
        if (!key) return;
        const active = this.getActiveSession();
        if (!active || !active.connected) return;

        if (key === 'AltTab') {
          this.sendAltTab(active.id);
        } else if (key === 'CAD') {
          this.sendCtrlAltDel(active.id);
        } else if (key === 'Meta') {
          this.sendWindowsKey(active.id);
        } else {
          this.sendKey(key, active.id);
        }
      };
    } catch (e) {
      console.warn('Physical Touch Bar SSE not connected:', e);
    }
  }

  getProxyWSURL() {
    const proto = location.protocol === 'https:' ? 'wss://' : 'ws://';
    return proto + location.host;
  }

  async connect(device, canvasElement, viewportElement) {
    if (!this.wasmLoaded) {
      await this.init();
    }

    const sessionId = device.id || `sess_${Date.now()}`;
    
    // If session already exists, focus it
    if (this.sessions.has(sessionId)) {
      const existing = this.sessions.get(sessionId);
      if (existing.connected) {
        this.setActiveSession(sessionId);
        return sessionId;
      }
      this.disconnect(sessionId);
    }

    const session = new RDPSession(sessionId, device, canvasElement, viewportElement);
    this.sessions.set(sessionId, session);
    this.activeSessionId = sessionId;

    this.bindSessionEvents(session);

    const host = device.host || '127.0.0.1';
    const port = String(device.port || 3389);
    const domain = device.domain || '';
    const user = device.username || '';
    const pass = device.password || '';

    // Calculate smart resolution
    let width = device.width || 0;
    let height = device.height || 0;
    if (width <= 0 || height <= 0) {
      width = viewportElement.clientWidth || window.innerWidth;
      height = viewportElement.clientHeight || window.innerHeight;
      width = Math.floor(width / 4) * 4;
      height = Math.floor(height / 4) * 4;
    }
    if (width < 800) width = 1440;
    if (height < 600) height = 900;

    canvasElement.width = width;
    canvasElement.height = height;

    const swapAltMeta = device.swapAltMeta ?? false;

    session.initAudio();

    if (this.onSessionStatusChange) {
      this.onSessionStatusChange(sessionId, 'connecting', `Connecting to ${host}:${port}...`);
    }

    // Inform physical Touch Bar that active session is launching
    fetch('/api/touchbar/state?active=1', { method: 'POST' }).catch(() => {});

    if (typeof window.rdpConnect === 'function') {
      window.rdpConnect(
        sessionId,
        this.getProxyWSURL(),
        host,
        port,
        domain,
        user,
        pass,
        width,
        height,
        canvasElement.id,
        swapAltMeta
      );
    } else {
      throw new Error('rdpConnect function not available in WASM runtime');
    }

    return sessionId;
  }

  disconnect(sessionId) {
    const sid = sessionId || this.activeSessionId;
    if (!sid) return;

    if (typeof window.rdpDisconnect === 'function') {
      window.rdpDisconnect(sid);
    }

    const s = this.sessions.get(sid);
    if (s) {
      s.connected = false;
      s.status = 'disconnected';
      s.closeAudio();
    }

    this.sessions.delete(sid);

    if (this.onSessionClose) {
      this.onSessionClose(sid);
    }

    if (this.sessions.size === 0) {
      this.activeSessionId = null;
      fetch('/api/touchbar/state?active=0', { method: 'POST' }).catch(() => {});
    } else if (this.activeSessionId === sid) {
      // Pick next available session
      const nextId = this.sessions.keys().next().value;
      this.setActiveSession(nextId);
    }
  }

  getActiveSession() {
    return this.activeSessionId ? this.sessions.get(this.activeSessionId) : null;
  }

  setActiveSession(sessionId) {
    if (!this.sessions.has(sessionId)) return;
    this.activeSessionId = sessionId;

    // Show only active viewport
    for (const [id, sess] of this.sessions.entries()) {
      if (sess.viewport) {
        if (id === sessionId) {
          sess.viewport.style.display = 'flex';
          sess.canvas.focus();
        } else {
          sess.viewport.style.display = 'none';
        }
      }
    }

    this.updateLayout(sessionId);
  }

  handleReady(sessionId) {
    const s = this.sessions.get(sessionId);
    if (!s) return;
    s.connected = true;
    s.status = 'connected';
    s.statusMsg = 'Connected';
    this.updateLayout(sessionId);
    if (this.onSessionStatusChange) {
      this.onSessionStatusChange(sessionId, 'connected', 'Connected');
    }
    s.canvas.focus();
  }

  handleError(sessionId, msg) {
    console.error(`[${sessionId}] RDP Error:`, msg);
    const s = this.sessions.get(sessionId);
    if (s) {
      s.connected = false;
      s.status = 'error';
      s.statusMsg = msg;
    }
    if (this.onSessionStatusChange) {
      this.onSessionStatusChange(sessionId, 'error', msg);
    }
  }

  handleClose(sessionId) {
    const s = this.sessions.get(sessionId);
    if (s) {
      s.connected = false;
      s.status = 'disconnected';
      s.statusMsg = 'Closed';
    }
    if (this.onSessionStatusChange) {
      this.onSessionStatusChange(sessionId, 'disconnected', 'Session Closed');
    }
  }

  handleRemoteClipboard(sessionId, text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).catch(err => {
        console.warn('Local clipboard write failed:', err);
      });
    }
  }

  syncLocalClipboard(sessionId) {
    const s = sessionId ? this.sessions.get(sessionId) : this.getActiveSession();
    if (!s || !s.connected || !navigator.clipboard || !navigator.clipboard.readText) return;

    navigator.clipboard.readText()
      .then(text => {
        if (text && typeof window.rdpClipboardChanged === 'function') {
          window.rdpClipboardChanged(s.id, text);
        }
      })
      .catch(() => {});
  }

  sendCtrlAltDel(sessionId) {
    const s = sessionId ? this.sessions.get(sessionId) : this.getActiveSession();
    if (!s || !s.connected) return;
    if (typeof window.rdpKeyDown === 'function') {
      window.rdpKeyDown(s.id, 'ControlLeft');
      window.rdpKeyDown(s.id, 'AltLeft');
      window.rdpKeyDown(s.id, 'Delete');
      setTimeout(() => {
        window.rdpKeyUp(s.id, 'Delete');
        window.rdpKeyUp(s.id, 'AltLeft');
        window.rdpKeyUp(s.id, 'ControlLeft');
      }, 100);
    }
  }

  sendWindowsKey(sessionId) {
    const s = sessionId ? this.sessions.get(sessionId) : this.getActiveSession();
    if (!s || !s.connected) return;
    if (typeof window.rdpKeyDown === 'function') {
      window.rdpKeyDown(s.id, 'MetaLeft');
      setTimeout(() => {
        window.rdpKeyUp(s.id, 'MetaLeft');
      }, 100);
    }
  }

  sendKey(code, sessionId) {
    const s = sessionId ? this.sessions.get(sessionId) : this.getActiveSession();
    if (!s || !s.connected) return;
    if (typeof window.rdpKeyDown === 'function') {
      window.rdpKeyDown(s.id, code);
      setTimeout(() => {
        if (typeof window.rdpKeyUp === 'function') {
          window.rdpKeyUp(s.id, code);
        }
      }, 60);
    }
  }

  sendAltTab(sessionId) {
    const s = sessionId ? this.sessions.get(sessionId) : this.getActiveSession();
    if (!s || !s.connected) return;
    if (typeof window.rdpKeyDown === 'function') {
      window.rdpKeyDown(s.id, 'AltLeft');
      window.rdpKeyDown(s.id, 'Tab');
      setTimeout(() => {
        window.rdpKeyUp(s.id, 'Tab');
        setTimeout(() => {
          window.rdpKeyUp(s.id, 'AltLeft');
        }, 150);
      }, 50);
    }
  }

  updateLayouts() {
    for (const sid of this.sessions.keys()) {
      this.updateLayout(sid);
    }
  }

  updateLayout(sessionId) {
    const s = this.sessions.get(sessionId);
    if (!s) return;
    const c = s.canvas;
    const vp = s.viewport;
    if (!c || !vp) return;

    if (!vp.classList.contains('fit-screen')) {
      c.style.width = c.width + 'px';
      c.style.height = c.height + 'px';
      return;
    }

    const vpWidth = vp.clientWidth;
    const vpHeight = vp.clientHeight;
    if (vpWidth <= 0 || vpHeight <= 0) return;

    const canvasAspect = (c.width || 1920) / (c.height || 1080);
    const vpAspect = vpWidth / vpHeight;

    let renderWidth, renderHeight;
    if (vpAspect > canvasAspect) {
      renderHeight = vpHeight;
      renderWidth = renderHeight * canvasAspect;
    } else {
      renderWidth = vpWidth;
      renderHeight = renderWidth / canvasAspect;
    }

    c.style.width = Math.floor(renderWidth) + 'px';
    c.style.height = Math.floor(renderHeight) + 'px';
  }

  handleAudioPlay(sessionId, sampleRate, channels, bitsPerSample, uint8Data) {
    const s = this.sessions.get(sessionId);
    if (!s || !s.audioCtx) return;
    try {
      const bytesPerSample = bitsPerSample >> 3;
      const numSamples = uint8Data.byteLength / (channels * bytesPerSample);
      if (numSamples <= 0) return;

      const audioBuf = s.audioCtx.createBuffer(channels, numSamples, sampleRate);
      if (bitsPerSample === 16) {
        const int16 = new Int16Array(uint8Data.buffer, uint8Data.byteOffset, numSamples * channels);
        for (let ch = 0; ch < channels; ch++) {
          const out = audioBuf.getChannelData(ch);
          for (let i = 0; i < numSamples; i++) {
            out[i] = int16[i * channels + ch] / 32768.0;
          }
        }
      }

      const src = s.audioCtx.createBufferSource();
      src.buffer = audioBuf;
      src.connect(s.audioCtx.destination);

      const now = s.audioCtx.currentTime;
      if (s.audioNextAt < now) {
        s.audioNextAt = now + 0.1;
      }
      src.start(s.audioNextAt);
      s.audioNextAt += audioBuf.duration;
    } catch (e) {
      console.warn('Audio play error:', e);
    }
  }

  handleH264(sessionId, destX, destY, w, h, isKey, uint8Data) {}

  handlePointerHide(sessionId) {
    const s = this.sessions.get(sessionId);
    if (s && s.canvas) s.canvas.style.cursor = 'none';
  }

  handlePointerCached(sessionId, idx) {
    const s = this.sessions.get(sessionId);
    if (!s) return;
    const css = s.pointerCache.get(idx);
    if (css && s.canvas) s.canvas.style.cursor = css;
  }

  handlePointerUpdate(sessionId, idx, bpp, hx, hy, w, h, andM, xorD) {
    const s = this.sessions.get(sessionId);
    if (!s) return;
    const css = this.buildCursorCss(bpp, hx, hy, w, h, andM, xorD);
    s.pointerCache.set(idx, css);
    if (s.canvas) s.canvas.style.cursor = css;
  }

  buildCursorCss(xorBpp, hotX, hotY, w, h, andMask, xorData) {
    try {
      const tmp = document.createElement('canvas');
      tmp.width = w;
      tmp.height = h;
      const octx = tmp.getContext('2d');
      const img = octx.createImageData(w, h);
      const px = img.data;

      const andStride = (((w + 15) >> 4) << 1);
      const andBit = (x, y) => {
        const off = y * andStride + (x >> 3);
        if (off >= andMask.length) return 0;
        return (andMask[off] >> (7 - (x & 7))) & 1;
      };

      if (xorBpp === 32) {
        const stride = w * 4;
        for (let y = 0; y < h; y++) {
          for (let x = 0; x < w; x++) {
            const s = y * stride + x * 4;
            const o = (y * w + x) << 2;
            const b = xorData[s], g = xorData[s+1], r = xorData[s+2], a = xorData[s+3];
            px[o] = r; px[o+1] = g; px[o+2] = b; px[o+3] = a;
          }
        }
      } else {
        for (let y = 0; y < h; y++) {
          for (let x = 0; x < w; x++) {
            const a = andBit(x, y);
            const o = (y * w + x) << 2;
            if (a === 0) {
              px[o] = 0; px[o+1] = 0; px[o+2] = 0; px[o+3] = 255;
            } else {
              px[o+3] = 0;
            }
          }
        }
      }

      octx.putImageData(img, 0, 0);
      const hx = Math.max(0, Math.min(w - 1, hotX | 0));
      const hy = Math.max(0, Math.min(h - 1, hotY | 0));
      return `url('${tmp.toDataURL()} ') ${hx} ${hy}, auto`;
    } catch (_) {
      return 'default';
    }
  }

  bindSessionEvents(session) {
    const c = session.canvas;
    const sid = session.id;

    c.addEventListener('mousemove', e => {
      if (!session.connected) return;
      const r = c.getBoundingClientRect();
      const scaleX = c.width / r.width;
      const scaleY = c.height / r.height;
      const x = Math.floor((e.clientX - r.left) * scaleX);
      const y = Math.floor((e.clientY - r.top) * scaleY);
      if (typeof window.rdpMouseMove === 'function') window.rdpMouseMove(sid, x, y);
    });

    c.addEventListener('mousedown', e => {
      if (!session.connected) return;
      e.preventDefault();
      c.focus();
      const r = c.getBoundingClientRect();
      const scaleX = c.width / r.width;
      const scaleY = c.height / r.height;
      const x = Math.floor((e.clientX - r.left) * scaleX);
      const y = Math.floor((e.clientY - r.top) * scaleY);
      if (typeof window.rdpMouseDown === 'function') window.rdpMouseDown(sid, x, y, e.button);
    });

    c.addEventListener('mouseup', e => {
      if (!session.connected) return;
      e.preventDefault();
      const r = c.getBoundingClientRect();
      const scaleX = c.width / r.width;
      const scaleY = c.height / r.height;
      const x = Math.floor((e.clientX - r.left) * scaleX);
      const y = Math.floor((e.clientY - r.top) * scaleY);
      if (typeof window.rdpMouseUp === 'function') window.rdpMouseUp(sid, x, y, e.button);
    });

    c.addEventListener('wheel', e => {
      if (!session.connected) return;
      e.preventDefault();
      let delta = -e.deltaY / 100;
      if (e.deltaMode === 1) delta = -e.deltaY / 3;
      if (typeof window.rdpMouseWheel === 'function') window.rdpMouseWheel(sid, delta);
    }, { passive: false });

    c.addEventListener('contextmenu', e => e.preventDefault());
    c.addEventListener('pointerdown', () => {
      c.focus();
      this.syncLocalClipboard(sid);
    });
  }

  bindGlobalKeyboard() {
    window.addEventListener('keydown', e => {
      const active = this.getActiveSession();
      const sessionView = document.getElementById('rdpSessionView');
      const isSessionActive = sessionView && sessionView.classList.contains('active');
      if (!active || !active.connected || !isSessionActive) return;

      if (document.activeElement !== active.canvas) {
        active.canvas.focus();
      }

      if (e.code === 'Tab' && (e.altKey || e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        e.stopPropagation();
        this.sendAltTab(active.id);
        return;
      }

      if (e.code === 'Tab' || e.code === 'AltLeft' || e.code === 'AltRight' || e.code.startsWith('Meta') || e.code.startsWith('F')) {
        e.preventDefault();
      }

      if (typeof window.rdpKeyDown === 'function') {
        window.rdpKeyDown(active.id, e.code);
      }
    });

    window.addEventListener('keyup', e => {
      const active = this.getActiveSession();
      const sessionView = document.getElementById('rdpSessionView');
      const isSessionActive = sessionView && sessionView.classList.contains('active');
      if (!active || !active.connected || !isSessionActive) return;

      if (e.code === 'Tab' || e.code === 'AltLeft' || e.code === 'AltRight' || e.code.startsWith('Meta') || e.code.startsWith('F')) {
        e.preventDefault();
      }

      if (typeof window.rdpKeyUp === 'function') {
        window.rdpKeyUp(active.id, e.code);
      }
    });
  }
}

window.rdpEngine = new RDPEngine();
