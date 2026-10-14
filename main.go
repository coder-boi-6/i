package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

func main() {
	mux := http.NewServeMux()
	s := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	hits := &apiConfig{
		fileserversHits: atomic.Int32{},
	}
	mux.HandleFunc("GET /api/healthz", readyhandler)
	mux.HandleFunc("GET /admin/metrics", hits.hitshandler)
	mux.HandleFunc("POST /admin/reset", hits.resethandler)
	mux.HandleFunc("POST /api/validate_chirp", chirpcharhandler)
	handler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", hits.middlewareMetricsInc(handler))
	s.ListenAndServe()
}

type apiConfig struct {
	fileserversHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserversHits.Add(1)
		next.ServeHTTP(w, r)
	})
}
func readyhandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
func (cfg *apiConfig) hitshandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	w.Write([]byte(fmt.Sprintf(`
	<html>
		<body>
			<h1>Welcome, Chirpy Admin</h1>
			<p>Chirpy has been visited %d times!</p>
		</body>
	</html>`, cfg.fileserversHits.Load())))
}
func (cfg *apiConfig) resethandler(w http.ResponseWriter, req *http.Request) {
	_ = cfg.fileserversHits.Swap(0)
}
func chirpcharhandler(w http.ResponseWriter, r *http.Request) {
	type body struct {
		Body string `json:"body"`
	}
	type erro struct {
		Err string `json:"error"`
	}
	type val struct {
		Val string `json:"cleaned_body"`
	}
	decoerr := json.NewDecoder(r.Body)
	bode := body{}
	err := decoerr.Decode(&bode)
	if err != nil {
		data, err := json.Marshal(erro{Err: "smth errored out bri"})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	}
	if len(bode.Body) > 140 {
		data, err := json.Marshal(erro{Err: "too long vrub"})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	} else {
		tune := []string{"kerfuffle", "sharbert", "fornax"}
		splitbod := strings.Split(bode.Body, " ")
		for i := range splitbod {
			for _, j := range tune {
				if strings.ToLower(splitbod[i]) == j {
					splitbod[i] = "****"
				}
			}
		}
		al := val{Val: strings.Join(splitbod, " ")}
		data, err := json.Marshal(al)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}
}
