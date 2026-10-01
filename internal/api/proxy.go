package api

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for seamless web and desktop access
	},
	ReadBufferSize:  64 * 1024,
	WriteBufferSize: 64 * 1024,
}

// HandleWebSocketProxy handles bridging browser WebSocket <-> remote RDP TCP port
func HandleWebSocketProxy(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	if target == "" {
		http.Error(w, "missing target query parameter (e.g. ?target=192.168.1.10:3389)", http.StatusBadRequest)
		return
	}

	// Ensure port is present
	if _, _, err := net.SplitHostPort(target); err != nil {
		target = net.JoinHostPort(target, "3389")
	}

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade error", "err", err)
		return
	}
	defer wsConn.Close()

	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	tcpConn, err := dialer.Dial("tcp", target)
	if err != nil {
		slog.Error("failed to connect to RDP target", "target", target, "err", err)
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("TCP dial error: %v", err))
		_ = wsConn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
		return
	}
	defer tcpConn.Close()

	slog.Info("RDP proxy tunnel established", "target", target, "client", r.RemoteAddr)

	errc := make(chan error, 2)

	// WebSocket -> TCP
	go func() {
		for {
			mt, data, err := wsConn.ReadMessage()
			if err != nil {
				slog.Debug("WS read error", "err", err)
				errc <- fmt.Errorf("ws read: %w", err)
				return
			}
			if mt == websocket.BinaryMessage || mt == websocket.TextMessage {
				if _, err := tcpConn.Write(data); err != nil {
					slog.Debug("TCP write error", "err", err)
					errc <- fmt.Errorf("tcp write: %w", err)
					return
				}
			}
		}
	}()

	// TCP -> WebSocket
	go func() {
		buf := make([]byte, 64*1024)
		for {
			n, err := tcpConn.Read(buf)
			if n > 0 {
				if werr := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					slog.Debug("WS write error", "err", werr)
					errc <- fmt.Errorf("ws write: %w", werr)
					return
				}
			}
			if err != nil {
				slog.Debug("TCP read error", "err", err)
				if err != io.EOF {
					errc <- fmt.Errorf("tcp read: %w", err)
				} else {
					errc <- fmt.Errorf("tcp server closed connection (EOF)")
				}
				return
			}
		}
	}()

	cause := <-errc
	slog.Info("RDP proxy tunnel closed", "target", target, "cause", cause)
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, cause.Error())
	_ = wsConn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
}
