package main

import (
	"chirpy/database"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	log.Print(godotenv.Load())
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("platform")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)
	mux := http.NewServeMux()
	s := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	hits := &apiConfig{
		fileserversHits: atomic.Int32{},
		dbQueries:       dbQueries,
		platform:        platform,
	}
	mux.HandleFunc("GET /api/healthz", readyhandler)
	mux.HandleFunc("GET /admin/metrics", hits.hitshandler)
	mux.HandleFunc("POST /admin/reset", hits.resethandler)
	mux.HandleFunc("POST /api/validate_chirp", chirpcharhandler)
	mux.HandleFunc("POST /api/users", hits.usercreatorhandler)
	handler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", hits.middlewareMetricsInc(handler))
	s.ListenAndServe()
}

type apiConfig struct {
	fileserversHits atomic.Int32
	dbQueries       *database.Queries
	platform        string
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
	w.Write(fmt.Appendf(nil, `
	<html>
		<body>
			<h1>Welcome, Chirpy Admin</h1>
			<p>Chirpy has been visited %d times!</p>
		</body>
	</html>`, cfg.fileserversHits.Load()))
}
func (cfg *apiConfig) resethandler(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	_ = cfg.fileserversHits.Swap(0)
	err := cfg.dbQueries.DeleteAllUsers(r.Context())
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

type erro struct {
	Err string `json:"error"`
}
type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

func chirpcharhandler(w http.ResponseWriter, r *http.Request) {
	type body struct {
		Body string `json:"body"`
	}
	type val struct {
		Val string `json:"cleaned_body"`
	}
	decoerr := json.NewDecoder(r.Body)
	bode := body{}
	err := decoerr.Decode(&bode)
	if err != nil {
		data, err := json.Marshal(erro{Err: err.Error()})
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
func (cfg *apiConfig) usercreatorhandler(w http.ResponseWriter, r *http.Request) {
	type em5i1 struct {
		Email string `json:"email"`
	}
	decoerr := json.NewDecoder(r.Body)
	bode := em5i1{}
	err := decoerr.Decode(&bode)
	if err != nil {
		data, err := json.Marshal(erro{Err: err.Error()})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
		return
	}
	ooser, err := cfg.dbQueries.CreateUser(r.Context(), bode.Email)
	if err != nil {
		data, err := json.Marshal(erro{Err: err.Error()})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(data)
		return
	}
	oos3r := User{
		ID:        ooser.ID,
		CreatedAt: ooser.CreatedAt,
		UpdatedAt: ooser.UpdatedAt,
		Email:     ooser.Email,
	}
	data, err := json.Marshal(oos3r)
	if err != nil {
		data, err := json.Marshal(erro{Err: err.Error()})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(data)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(data)
}
