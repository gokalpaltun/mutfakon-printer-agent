// Mutfak On printer agent. Listens on 127.0.0.1 only; the kasa page calls it with fetch().
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
)

const version = "0.2.0"

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
	allowed := strings.Split(env("AGENT_ORIGINS", "http://localhost:5280,http://mutfakon-portal.trackallops.com,https://mutfakon-portal.trackallops.com"), ",")
	addr := "127.0.0.1:" + port
	fmt.Printf("Mutfak On printer agent %s on http://%s\n", version, addr)
	log.Fatal(http.ListenAndServe(addr, handler(allowed)))
}
