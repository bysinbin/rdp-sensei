//go:build js && wasm

package main

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
	"syscall/js"

	"github.com/nakagami/grdp"
	"github.com/nakagami/grdp/plugin/rdpsnd"
)

type SessionClient struct {
	id             string
	client         *grdp.RdpClient
	canvas         js.Value
	ctx2d          js.Value
	localClipboard string
	clipMu         sync.Mutex
	swapAltMeta    bool
	closed         bool
}

var (
	sessionsMu sync.Mutex
	sessions   = make(map[string]*SessionClient)
)

func getSession(id string) *SessionClient {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	if id != "" {
		return sessions[id]
	}
	// Fallback to first available session
	for _, s := range sessions {
		return s
	}
	return nil
}

func main() {
	js.Global().Set("rdpConnect", js.FuncOf(jsConnect))
	js.Global().Set("rdpDisconnect", js.FuncOf(jsDisconnect))
	js.Global().Set("rdpMouseMove", js.FuncOf(jsMouseMove))
	js.Global().Set("rdpMouseDown", js.FuncOf(jsMouseDown))
	js.Global().Set("rdpMouseUp", js.FuncOf(jsMouseUp))
	js.Global().Set("rdpMouseWheel", js.FuncOf(jsMouseWheel))
	js.Global().Set("rdpKeyDown", js.FuncOf(jsKeyDown))
	js.Global().Set("rdpKeyUp", js.FuncOf(jsKeyUp))
	js.Global().Set("rdpClipboardChanged", js.FuncOf(jsClipboardChanged))

	// Block forever — JS callbacks keep things alive.
	select {}
}

// jsConnect handles:
// rdpConnect(sessionId, proxyWsURL, host, port, domain, user, password, width, height, canvasId, swapAltMeta)
// Or legacy: rdpConnect(proxyWsURL, host, port, domain, user, password, width, height[, swapAltMeta])
func jsConnect(_ js.Value, args []js.Value) any {
	var sessionId, proxyWsURL, host, port, domain, user, password, canvasId string
	var width, height int
	var swapAltMeta bool

	if len(args) >= 10 {
		// Multi-session signature
		sessionId = args[0].String()
		proxyWsURL = args[1].String()
		host = args[2].String()
		port = args[3].String()
		domain = args[4].String()
		user = args[5].String()
		password = args[6].String()
		width = args[7].Int()
		height = args[8].Int()
		canvasId = args[9].String()
		if len(args) >= 11 {
			swapAltMeta = args[10].Bool()
		}
	} else if len(args) >= 8 {
		// Legacy single-session signature
		sessionId = "default"
		proxyWsURL = args[0].String()
		host = args[1].String()
		port = args[2].String()
		domain = args[3].String()
		user = args[4].String()
		password = args[5].String()
		width = args[6].Int()
		height = args[7].Int()
		canvasId = "rdpCanvas"
		if len(args) >= 9 {
			swapAltMeta = args[8].Bool()
		}
	} else {
		return fmt.Sprintf("usage: rdpConnect(sessionId, proxyWsURL, host, port, domain, user, password, width, height, canvasId[, swapAltMeta])")
	}

	if canvasId == "" {
		canvasId = "canvas_" + sessionId
	}

	go func() {
		if err := connectSession(sessionId, proxyWsURL, host, port, domain, user, password, width, height, canvasId, swapAltMeta); err != nil {
			slog.Error("connect", "session", sessionId, "err", err)
			js.Global().Call("rdpOnError", sessionId, err.Error())
		}
	}()
	return nil
}

func connectSession(sessionId, proxyWsURL, host, port, domain, user, password string, width, height int, canvasId string, swapAltMeta bool) error {
	sessionsMu.Lock()
	if old, exists := sessions[sessionId]; exists && old.client != nil {
		old.client.Close()
		delete(sessions, sessionId)
	}
	sessionsMu.Unlock()

	hostPort := host + ":" + port
	wsURL := proxyWsURL + "/ws?target=" + hostPort

	g := grdp.NewRdpClient(hostPort, width, height, func(hp string) (net.Conn, error) {
		return dialWebSocket(wsURL)
	})

	canvas := js.Global().Get("document").Call("getElementById", canvasId)
	if canvas.IsNull() || canvas.IsUndefined() {
		// Fallback to rdpCanvas
		canvas = js.Global().Get("document").Call("getElementById", "rdpCanvas")
	}

	var ctx2d js.Value
	if !canvas.IsNull() && !canvas.IsUndefined() {
		ctx2d = canvas.Call("getContext", "2d")
		canvas.Set("width", width)
		canvas.Set("height", height)
	}

	sess := &SessionClient{
		id:          sessionId,
		client:      g,
		canvas:      canvas,
		ctx2d:       ctx2d,
		swapAltMeta: swapAltMeta,
	}

	sessionsMu.Lock()
	sessions[sessionId] = sess
	sessionsMu.Unlock()

	g.OnAudio(func(af rdpsnd.AudioFormat, data []byte) {
		cp := make([]byte, len(data))
		copy(cp, data)
		playAudio(sessionId, int(af.SamplesPerSec), int(af.Channels), int(af.BitsPerSample), cp)
	})

	g.OnH264Raw(func(destX, destY, w, h int, isKey bool, data []byte) {
		jsArr := js.Global().Get("Uint8Array").New(len(data))
		js.CopyBytesToJS(jsArr, data)
		js.Global().Call("rdpOnH264", sessionId, destX, destY, w, h, isKey, jsArr)
	})

	uint8Ctor := js.Global().Get("Uint8Array")
	g.OnPointerHide(func() {
		js.Global().Call("rdpOnPointerHide", sessionId)
	}).OnPointerCached(func(idx uint16) {
		js.Global().Call("rdpOnPointerCached", sessionId, int(idx))
	}).OnPointerUpdate(func(idx, xorBpp, hotX, hotY, w, h uint16, andMask, xorData []byte) {
		andArr := uint8Ctor.New(len(andMask))
		if len(andMask) > 0 {
			js.CopyBytesToJS(andArr, andMask)
		}
		xorArr := uint8Ctor.New(len(xorData))
		if len(xorData) > 0 {
			js.CopyBytesToJS(xorArr, xorData)
		}
		js.Global().Call("rdpOnPointerUpdate",
			sessionId, int(idx), int(xorBpp), int(hotX), int(hotY), int(w), int(h), andArr, xorArr)
	})

	g.OnError(func(e error) {
		slog.Debug("rdp error", "session", sessionId, "err", e)
		js.Global().Call("rdpOnError", sessionId, e.Error())
	}).OnClose(func() {
		slog.Debug("rdp close", "session", sessionId)
		sessionsMu.Lock()
		delete(sessions, sessionId)
		sessionsMu.Unlock()
		js.Global().Call("rdpOnClose", sessionId)
	}).OnSuccess(func() {
		slog.Debug("rdp success", "session", sessionId)
	}).OnReady(func() {
		slog.Debug("rdp ready", "session", sessionId)
		js.Global().Call("rdpOnReady", sessionId)
	}).OnBitmap(func(bs []grdp.Bitmap) {
		for i := range bs {
			d := make([]byte, len(bs[i].Data))
			copy(d, bs[i].Data)
			bs[i].Data = d
		}
		go renderBitmaps(sess, bs)
	})

	g.OnClipboard(
		func(text string) {
			js.Global().Call("rdpOnClipboard", sessionId, text)
		},
		func() string {
			sess.clipMu.Lock()
			defer sess.clipMu.Unlock()
			return sess.localClipboard
		},
	)

	if err := g.Login(domain, user, password); err != nil {
		sessionsMu.Lock()
		delete(sessions, sessionId)
		sessionsMu.Unlock()
		return err
	}

	return nil
}

func renderBitmaps(sess *SessionClient, bs []grdp.Bitmap) {
	if sess.ctx2d.IsNull() || sess.ctx2d.IsUndefined() {
		return
	}

	uint8ClampedCtor := js.Global().Get("Uint8ClampedArray")
	imageDataCtor := js.Global().Get("ImageData")

	for _, bm := range bs {
		w := bm.DestRight - bm.DestLeft + 1
		if w > bm.Width {
			w = bm.Width
		}
		h := bm.DestBottom - bm.DestTop + 1
		if h > bm.Height {
			h = bm.Height
		}
		if w <= 0 || h <= 0 {
			continue
		}

		rgba := make([]byte, w*h*4)
		if bm.BitsPerPixel == 4 {
			srcStride := bm.Width * 4
			dstStride := w * 4
			for row := 0; row < h; row++ {
				src := bm.Data[row*srcStride:]
				dst := rgba[row*dstStride:]
				for col := 0; col < w; col++ {
					dst[col*4+0] = src[col*4+2] // R ← BGRA[2]
					dst[col*4+1] = src[col*4+1] // G
					dst[col*4+2] = src[col*4+0] // B ← BGRA[0]
					dst[col*4+3] = src[col*4+3] // A
				}
			}
		} else {
			m := bm.RGBA()
			for row := 0; row < h; row++ {
				src := m.Pix[row*m.Stride : row*m.Stride+w*4]
				copy(rgba[row*w*4:], src)
			}
		}

		jsArr := uint8ClampedCtor.New(len(rgba))
		js.CopyBytesToJS(jsArr, rgba)
		imageData := imageDataCtor.New(jsArr, w, h)
		sess.ctx2d.Call("putImageData", imageData, bm.DestLeft, bm.DestTop)
	}
}

func jsDisconnect(_ js.Value, args []js.Value) any {
	sid := ""
	if len(args) >= 1 && args[0].Type() == js.TypeString {
		sid = args[0].String()
	}
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	if sid != "" {
		if s, ok := sessions[sid]; ok {
			if s.client != nil {
				s.client.Close()
			}
			delete(sessions, sid)
		}
	} else {
		// Disconnect all
		for k, s := range sessions {
			if s.client != nil {
				s.client.Close()
			}
			delete(sessions, k)
		}
	}
	return nil
}

func jsMouseMove(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return nil
	}
	var sess *SessionClient
	var x, y int
	if args[0].Type() == js.TypeString && len(args) >= 3 {
		sess = getSession(args[0].String())
		x = args[1].Int()
		y = args[2].Int()
	} else {
		sess = getSession("")
		x = args[0].Int()
		y = args[1].Int()
	}
	if sess != nil && sess.client != nil {
		sess.client.MouseMove(x, y)
	}
	return nil
}

func jsMouseDown(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return nil
	}
	var sess *SessionClient
	var x, y, btn int
	if args[0].Type() == js.TypeString && len(args) >= 4 {
		sess = getSession(args[0].String())
		x = args[1].Int()
		y = args[2].Int()
		btn = args[3].Int()
	} else {
		sess = getSession("")
		x = args[0].Int()
		y = args[1].Int()
		btn = args[2].Int()
	}
	if sess != nil && sess.client != nil {
		sess.client.MouseDown(btn, x, y)
	}
	return nil
}

func jsMouseUp(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return nil
	}
	var sess *SessionClient
	var x, y, btn int
	if args[0].Type() == js.TypeString && len(args) >= 4 {
		sess = getSession(args[0].String())
		x = args[1].Int()
		y = args[2].Int()
		btn = args[3].Int()
	} else {
		sess = getSession("")
		x = args[0].Int()
		y = args[1].Int()
		btn = args[2].Int()
	}
	if sess != nil && sess.client != nil {
		sess.client.MouseUp(btn, x, y)
	}
	return nil
}

func jsMouseWheel(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return nil
	}
	var sess *SessionClient
	var delta float64
	if args[0].Type() == js.TypeString && len(args) >= 2 {
		sess = getSession(args[0].String())
		delta = args[1].Float()
	} else {
		sess = getSession("")
		delta = args[0].Float()
	}
	if sess != nil && sess.client != nil {
		sess.client.MouseWheel(delta)
	}
	return nil
}

func jsKeyDown(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return nil
	}
	var sess *SessionClient
	var keyName string
	if len(args) >= 2 && args[0].Type() == js.TypeString {
		sess = getSession(args[0].String())
		keyName = args[1].String()
	} else {
		sess = getSession("")
		keyName = args[0].String()
	}
	if sess != nil && sess.client != nil {
		code := jsCodeToRDP(keyName, sess.swapAltMeta)
		if code != 0 {
			sess.client.KeyDown(code)
		}
	}
	return nil
}

func jsKeyUp(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return nil
	}
	var sess *SessionClient
	var keyName string
	if len(args) >= 2 && args[0].Type() == js.TypeString {
		sess = getSession(args[0].String())
		keyName = args[1].String()
	} else {
		sess = getSession("")
		keyName = args[0].String()
	}
	if sess != nil && sess.client != nil {
		code := jsCodeToRDP(keyName, sess.swapAltMeta)
		if code != 0 {
			sess.client.KeyUp(code)
		}
	}
	return nil
}

func jsClipboardChanged(_ js.Value, args []js.Value) any {
	var sess *SessionClient
	var text string
	if len(args) >= 2 && args[0].Type() == js.TypeString {
		sess = getSession(args[0].String())
		text = args[1].String()
	} else if len(args) >= 1 {
		sess = getSession("")
		text = args[0].String()
	}
	if sess != nil {
		sess.clipMu.Lock()
		sess.localClipboard = text
		sess.clipMu.Unlock()

		if sess.client != nil {
			sess.client.NotifyClipboardChanged()
		}
	}
	return nil
}
