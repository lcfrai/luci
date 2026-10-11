// A hand-written fixture for preview transport; it is not model-generated code.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html lang="en"><meta charset="utf-8"><title>Preview calculator</title><h1>Preview calculator</h1><form action="/sum"><label>First amount <input name="a" type="number" step="any" value="12.5"></label><label>Second amount <input name="b" type="number" step="any" value="7.25"></label><button>Add amounts</button></form><p>A local fixture for verifying the preview build and serving path.</p></html>`)
	})
	mux.HandleFunc("/sum", func(w http.ResponseWriter, r *http.Request) {
		a, ea := strconv.ParseFloat(r.URL.Query().Get("a"), 64)
		b, eb := strconv.ParseFloat(r.URL.Query().Get("b"), 64)
		if ea != nil || eb != nil || math.IsInf(a, 0) || math.IsInf(b, 0) || math.IsNaN(a) || math.IsNaN(b) || math.Abs(a) > 1e6 || math.Abs(b) > 1e6 {
			http.Error(w, "Use finite amounts between -1000000 and 1000000", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"sum": a + b, "greeting": os.Getenv("APP_GREETING"), "uid": os.Getuid(), "modelKeyPresent": os.Getenv("OPENAI_API_KEY") != "" || os.Getenv("ANTHROPIC_API_KEY") != ""})
	})
	server := &http.Server{Addr: ":3000", Handler: mux, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 8192}
	if err := server.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
