package web

import (
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
)

// termControlMessage is the JSON shape sent as a text frame by the browser
// terminal for out-of-band events (currently just resize); keystrokes travel
// as binary frames and go straight to the exec session's stdin.
type termControlMessage struct {
	Type string `json:"type"`
	Cols uint   `json:"cols"`
	Rows uint   `json:"rows"`
}

// handleContainerTerminal upgrades the connection to a WebSocket and relays
// bytes between it and an interactive `docker exec` shell session, giving
// the browser a real terminal inside the container.
func (s *Server) handleContainerTerminal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	// Start the exec session before upgrading so a failure (e.g. unknown
	// container) can still be reported as a normal HTTP error.
	execID, hijacked, err := s.exec.ExecShell(ctx, id)
	if err != nil {
		http.Error(w, "starting terminal: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer hijacked.Close()

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()

	done := make(chan error, 2)

	// docker -> browser
	go func() {
		buf := make([]byte, 4096)
		for {
			n, readErr := hijacked.Reader.Read(buf)
			if n > 0 {
				if writeErr := conn.Write(ctx, websocket.MessageBinary, buf[:n]); writeErr != nil {
					done <- writeErr
					return
				}
			}
			if readErr != nil {
				done <- readErr
				return
			}
		}
	}()

	// browser -> docker (keystrokes as binary frames, control messages as text/JSON)
	go func() {
		for {
			msgType, data, readErr := conn.Read(ctx)
			if readErr != nil {
				done <- readErr
				return
			}
			switch msgType {
			case websocket.MessageBinary:
				if _, writeErr := hijacked.Conn.Write(data); writeErr != nil {
					done <- writeErr
					return
				}
			case websocket.MessageText:
				var msg termControlMessage
				if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 {
					_ = s.exec.ExecResize(ctx, execID, msg.Rows, msg.Cols)
				}
			}
		}
	}()

	<-done
	conn.Close(websocket.StatusNormalClosure, "session ended")
}
