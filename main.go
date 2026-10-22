package main

import (
	"chirpy/auth"
	"chirpy/database"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	log.Print(godotenv.Load())
	dbURL := os.Getenv("db_url")
	platform := os.Getenv("platform")
	secret := os.Getenv("secret")
	polka_key := os.Getenv("polka_key")
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
		secret:          secret,
		polka_key:       polka_key,
	}
	mux.HandleFunc("GET /api/healthz", readyhandler)
	mux.HandleFunc("GET /admin/metrics", hits.hitshandler)
	mux.HandleFunc("POST /admin/reset", hits.resethandler)
	mux.HandleFunc("POST /api/chirps", hits.chirpcreatorhandler)
	mux.HandleFunc("POST /api/users", hits.usercreatorhandler)
	mux.HandleFunc("GET /api/chirps", hits.chirpsgetter)
	mux.HandleFunc("GET /api/chirps/{chirpID}", hits.chirpgetter)
	mux.HandleFunc("POST /api/login", hits.userloginquery)
	mux.HandleFunc("POST /api/refresh", hits.refreshtokenquery)
	mux.HandleFunc("POST /api/revoke", hits.revoketoken)
	mux.HandleFunc("PUT /api/users", hits.updateuser)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", hits.deletechirp)
	mux.HandleFunc("POST /api/polka/webhooks", hits.upgradechirpyred)
	handler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", hits.middlewareMetricsInc(handler))
	s.ListenAndServe()
}

type apiConfig struct {
	fileserversHits atomic.Int32
	dbQueries       *database.Queries
	platform        string
	secret          string
	polka_key       string
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
	ChirpyRed bool      `json:"is_chirpy_red"`
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
		Body string `json:"body"`
	}
	type val struct {
		Val string `json:"cleaned_body"`
	}
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		data, err := json.Marshal(erro{Err: err.Error()})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(data)
		return
	}
	user_id, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		data, err := json.Marshal(erro{Err: err.Error()})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(data)
		return
	}
	decoerr := json.NewDecoder(r.Body)
	bode := body{}
	err = decoerr.Decode(&bode)
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
			UserID: user_id,
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
		ChirpyRed: ooser.IsChirpyRed,
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
	var err error
	author_id_ID := uuid.UUID{}
	author_id := uuid.NullUUID{}
	author_idstring := r.URL.Query().Get("author_id")
	if len(author_idstring) == 0 {
		author_id.Valid = false
	} else {
		author_id_ID, err = uuid.Parse(author_idstring)
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
		author_id.UUID = author_id_ID
		author_id.Valid = true
	}
	chirps, err := cfg.dbQueries.RetrieveAllChirps(r.Context(), author_id)
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
	order := r.URL.Query().Get("sort")
	if order == "desc" {
		sort.Slice(ie, func(i, j int) bool {
			if ie[i].CreatedAt.Before(ie[j].CreatedAt) {
				return false
			}
			return true
		})
	} else {
		sort.Slice(ie, func(i, j int) bool {
			if ie[i].CreatedAt.Before(ie[j].CreatedAt) {
				return true
			}
			return false
		})
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
	type loginResponse struct {
		User
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
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
	ooser, err := cfg.dbQueries.UserByEmail(r.Context(), bode.Email)
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
		ChirpyRed: ooser.IsChirpyRed,
	}
	token, err := auth.MakeJWT(yut.ID, cfg.secret, time.Hour)
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
	refresh := auth.MakeRefreshToken()
	_, err = cfg.dbQueries.CreateRefreshRow(r.Context(), database.CreateRefreshRowParams{
		Token:     refresh,
		UserID:    yut.ID,
		ExpiresAt: time.Now().Add(time.Hour * 24 * 60),
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
	yummers := loginResponse{
		User:         yut,
		Token:        token,
		RefreshToken: refresh,
	}
	data, err := json.Marshal(yummers)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
func (cfg *apiConfig) refreshtokenquery(w http.ResponseWriter, r *http.Request) {
	type tokenresp struct {
		Token string `json:"token"`
	}
	refreshtoken, err := auth.GetBearerToken(r.Header)
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
	user, err := cfg.dbQueries.UserRefreshTokenThing(r.Context(), refreshtoken)
	if err != nil {
		data, err := json.Marshal(erro{Err: err.Error()})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(data)
		return
	}
	tokenstring, err := auth.MakeJWT(user.ID, cfg.secret, time.Hour)
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
	yumrs := tokenresp{
		Token: tokenstring,
	}
	data, err := json.Marshal(yumrs)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
func (cfg *apiConfig) revoketoken(w http.ResponseWriter, r *http.Request) {
	refreshtoken, err := auth.GetBearerToken(r.Header)
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
	err = cfg.dbQueries.RevokeToken(r.Context(), refreshtoken)
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
	w.WriteHeader(http.StatusNoContent)
}
func (cfg *apiConfig) updateuser(w http.ResponseWriter, r *http.Request) {
	tokenmaxxing, err := auth.GetBearerToken(r.Header)
	if err != nil {
		data, err := json.Marshal(erro{Err: err.Error()})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(data)
		return
	}
	decoerr := json.NewDecoder(r.Body)
	bode := oser{}
	err = decoerr.Decode(&bode)
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
	user_id, err := auth.ValidateJWT(tokenmaxxing, cfg.secret)
	if err != nil {
		data, err := json.Marshal(erro{Err: err.Error()})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(data)
		return
	}
	oozer, err := cfg.dbQueries.UpdateUser(r.Context(), database.UpdateUserParams{
		HashedPassword: hasj,
		Email:          bode.Email,
		ID:             user_id,
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
	yut := User{
		ID:        oozer.ID,
		CreatedAt: oozer.CreatedAt,
		UpdatedAt: oozer.UpdatedAt,
		Email:     oozer.Email,
		ChirpyRed: oozer.IsChirpyRed,
	}
	data, err := json.Marshal(yut)
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
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
func (cfg *apiConfig) deletechirp(w http.ResponseWriter, r *http.Request) {
	tokenmaxxing, err := auth.GetBearerToken(r.Header)
	if err != nil {
		data, err := json.Marshal(erro{Err: err.Error()})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(data)
		return
	}
	user_id, err := auth.ValidateJWT(tokenmaxxing, cfg.secret)
	if err != nil {
		data, err := json.Marshal(erro{Err: err.Error()})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(data)
		return
	}
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
	_, err = cfg.dbQueries.GetChirp(r.Context(), id)
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
	_, err = cfg.dbQueries.DeleteChirp(r.Context(), database.DeleteChirpParams{
		UserID: user_id,
		ID:     id,
	})
	if errors.Is(err, sql.ErrNoRows) {
		data, err := json.Marshal(erro{Err: err.Error()})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		w.Write(data)
		return
	} else if err != nil {
		data, err := json.Marshal(erro{Err: err.Error()})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(data)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (cfg *apiConfig) upgradechirpyred(w http.ResponseWriter, r *http.Request) {
	type eventinput struct {
		Event string `json:"event"`
		Data  struct {
			User_ID string `json:"user_id"`
		} `json:"data"`
	}
	polakey, err := auth.GetAPIKey(r.Header)
	if polakey != cfg.polka_key {
		w.WriteHeader(http.StatusUnauthorized)
	}
	decoerr := json.NewDecoder(r.Body)
	bode := eventinput{}
	err = decoerr.Decode(&bode)
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
	if bode.Event != "user.upgraded" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	user_id, err := uuid.Parse(bode.Data.User_ID)
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
	_, err = cfg.dbQueries.UpgradetoChirpyRed(r.Context(), user_id)
	if errors.Is(err, sql.ErrNoRows) {
		data, err := json.Marshal(erro{Err: err.Error()})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write(data)
		return
	} else if err != nil {
		data, err := json.Marshal(erro{Err: err.Error()})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(data)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	w.Write([]byte(""))
}
