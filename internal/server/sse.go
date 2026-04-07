package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type sseEvent struct {
	Name string
	Data any
}

type sseHub struct {
	mu      sync.RWMutex
	clients map[chan sseEvent]struct{}
}

func newSSEHub() *sseHub {
	return &sseHub{clients: map[chan sseEvent]struct{}{}}
}

func (h *sseHub) subscribe() chan sseEvent {
	ch := make(chan sseEvent, 8)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *sseHub) unsubscribe(ch chan sseEvent) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

func (h *sseHub) broadcast(name string, data any) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ev := sseEvent{Name: name, Data: data}
	for ch := range h.clients {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (d *Daemon) sseStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := d.sse.subscribe()
	defer d.sse.unsubscribe(ch)

	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		case ev := <-ch:
			payload, err := json.Marshal(ev.Data)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Name, payload)
			flusher.Flush()
		}
	}
}
