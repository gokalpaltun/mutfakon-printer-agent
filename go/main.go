// Mutfak On printer agent. Listens on 127.0.0.1 only; the kasa page calls it with fetch().
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
)

const version = "0.2.3"

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func handler(allowed []string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			writeJSON(w, 404, map[string]any{"error": "NOT_FOUND"})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(testPageHTML))
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "version": version, "platform": runtime.GOOS})
	})
	mux.HandleFunc("/printers", func(w http.ResponseWriter, r *http.Request) {
		printers, err := listPrinters()
		if err != nil {
			writeJSON(w, 500, map[string]any{"error": "LIST_FAILED", "message": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"printers": printers})
	})
	mux.HandleFunc("/print", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]any{"error": "METHOD_NOT_ALLOWED"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
		var body struct {
			Printer string `json:"printer"`
			Copies  int    `json:"copies"`
			Width   int    `json:"width"`
			Height  int    `json:"height"`
			Data    string `json:"data"` // base64, 1 bit per dot
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Printer == "" {
			writeJSON(w, 400, map[string]any{"error": "BAD_REQUEST"})
			return
		}
		bitmap, err := base64.StdEncoding.DecodeString(body.Data)
		if err != nil {
			writeJSON(w, 400, map[string]any{"error": "BAD_BITMAP"})
			return
		}
		job, err := rasterReceipt(body.Width, body.Height, bitmap, body.Copies)
		if err != nil {
			writeJSON(w, 400, map[string]any{"error": "BAD_BITMAP", "message": err.Error()})
			return
		}
		if err := printRaw(body.Printer, job); err != nil {
			log.Printf("print failed: %v", err)
			writeJSON(w, 500, map[string]any{"error": "PRINT_FAILED", "message": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})
	mux.HandleFunc("/test-print-hard", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]any{"error": "METHOD_NOT_ALLOWED"})
			return
		}
		var body struct {
			Printer string `json:"printer"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Printer == "" {
			writeJSON(w, 400, map[string]any{"error": "PRINTER_REQUIRED"})
			return
		}
		job, err := hardTestReceipt(1)
		if err == nil {
			err = printRaw(body.Printer, job)
		}
		if err != nil {
			log.Printf("hard test print failed: %v", err)
			writeJSON(w, 500, map[string]any{"error": "PRINT_FAILED", "message": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})
	mux.HandleFunc("/test-print", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]any{"error": "METHOD_NOT_ALLOWED"})
			return
		}
		var body struct {
			Printer string `json:"printer"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Printer == "" {
			writeJSON(w, 400, map[string]any{"error": "PRINTER_REQUIRED"})
			return
		}
		if err := printRaw(body.Printer, testReceipt(body.Printer)); err != nil {
			log.Printf("print failed: %v", err)
			writeJSON(w, 500, map[string]any{"error": "PRINT_FAILED", "message": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	// Only our own pages may call the agent (the browser sends Origin on cross-origin fetches).
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			ok := false
			for _, a := range allowed {
				if a == origin {
					ok = true
				}
			}
			if !ok {
				writeJSON(w, 403, map[string]any{"error": "ORIGIN_NOT_ALLOWED"})
				return
			}
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Vary", "Origin")
			h.Set("Access-Control-Allow-Headers", "Content-Type")
			h.Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
			h.Set("Access-Control-Allow-Private-Network", "true") // Chrome's preflight for public page -> localhost
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func main() {
	port := env("AGENT_PORT", "9187")
	// The agent's own page calls it too (same origin).
	allowed := []string{"http://127.0.0.1:" + port, "http://localhost:" + port}
	allowed = append(allowed, strings.Split(env("AGENT_ORIGINS", "http://localhost:5280,http://mutfakon-portal.trackallops.com,https://mutfakon-portal.trackallops.com"), ",")...)
	addr := "127.0.0.1:" + port
	fmt.Printf("Mutfak On printer agent %s on http://%s\n", version, addr)
	log.Fatal(http.ListenAndServe(addr, handler(allowed)))
}
