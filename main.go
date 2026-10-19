package main

import (
	"chirpy/auth"
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
	mux.HandleFunc("POST /api/chirps", hits.chirpcreatorhandler)
	mux.HandleFunc("POST /api/users", hits.usercreatorhandler)
	mux.HandleFunc("GET /api/chirps", hits.chirpsgetter)
	mux.HandleFunc("GET /api/chirps/{chirpID}", hits.chirpgetter)
	mux.HandleFunc("POST /api/login", hits.userloginquery)
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
func (cfg *apiConfig) hitshandler(w http.ResponseWriter, r *http.Request) {
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
type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}
type oser struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (cfg *apiConfig) chirpcreatorhandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	type body struct {
		Body    string    `json:"body"`
		User_Id uuid.UUID `json:"user_id"`
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
		chirp, err := cfg.dbQueries.CreateChirp(r.Context(), database.CreateChirpParams{
			Body:   al.Val,
			UserID: bode.User_Id,
		})
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
		mirp := Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		}
		data, err := json.Marshal(mirp)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Write(data)
		return
	}
}
func (cfg *apiConfig) usercreatorhandler(w http.ResponseWriter, r *http.Request) {
	decoerr := json.NewDecoder(r.Body)
	bode := oser{}
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
	hasj, err := auth.HashPassword(bode.Password)
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
	ooser, err := cfg.dbQueries.CreateUser(r.Context(), database.CreateUserParams{
		Email:          bode.Email,
		HashedPassword: hasj,
	})
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
func (cfg *apiConfig) chirpsgetter(w http.ResponseWriter, r *http.Request) {
	chirps, err := cfg.dbQueries.RetrieveAllChirps(r.Context())
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
	ie := []Chirp{}
	for _, i := range chirps {
		i := Chirp{
			ID:        i.ID,
			CreatedAt: i.CreatedAt,
			UpdatedAt: i.UpdatedAt,
			Body:      i.Body,
			UserID:    i.UserID,
		}
		ie = append(ie, i)
	}
	data, err := json.Marshal(ie)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
func (cfg *apiConfig) chirpgetter(w http.ResponseWriter, r *http.Request) {
	chirpIDString := r.PathValue("chirpID")
	id, err := uuid.Parse(chirpIDString)
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
	chirp, err := cfg.dbQueries.GetChirp(r.Context(), id)
	if err != nil {
		data, err := json.Marshal(erro{Err: err.Error()})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write(data)
		return
	}
	chirpfr := Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	}
	data, err := json.Marshal(chirpfr)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
func (cfg *apiConfig) userloginquery(w http.ResponseWriter, r *http.Request) {
	decoerr := json.NewDecoder(r.Body)
	bode := oser{}
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
	ooser, err := cfg.dbQueries.UserbyEmail(r.Context(), bode.Email)
	if err != nil {
		data, err := json.Marshal(erro{Err: "Incorrect email or password"})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(data)
		return
	}
	truth, err := auth.CheckPasswordHash(bode.Password, ooser.HashedPassword)
	if (err != nil) || (truth == false) {
		data, err := json.Marshal(erro{Err: "Incorrect email or password"})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(data)
		return
	}
	yut := User{
		ID:        ooser.ID,
		CreatedAt: ooser.CreatedAt,
		UpdatedAt: ooser.UpdatedAt,
		Email:     ooser.Email,
	}
	data, err := json.Marshal(yut)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
