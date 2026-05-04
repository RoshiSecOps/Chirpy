package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/RoshiSecOps/Chirpy/internal/auth"
	"github.com/RoshiSecOps/Chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println(".env not found")
	}
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL is not set")
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)
	apiCfg := apiConfig{db: dbQueries, platform: os.Getenv("PLATFORM")}
	mux := http.NewServeMux()
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir("./app")))))
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("POST /api/chirps", apiCfg.createChirpHandler)
	mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)
	mux.HandleFunc("POST /api/users", apiCfg.createUserHandler)
	mux.HandleFunc("POST /api/login", apiCfg.loginUserHandler)
	mux.HandleFunc("GET /api/chirps", apiCfg.getChirpsHandler)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.getChirpHanlder)

	server := http.Server{Addr: ":8080", Handler: mux}
	log.Fatal(server.ListenAndServe())

}

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
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

func (cfg *apiConfig) getHits() int {
	return int(cfg.fileserverHits.Load())
}

func (cfg *apiConfig) resetHits() {
	cfg.fileserverHits.Swap(0)
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	if cfg.platform == "dev" {
		cfg.resetHits()
		cfg.db.DeleteUsers(r.Context())
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		return
	}
	respondWithError(w, 403, "")
}

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	dat, err := io.ReadAll(r.Body)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	params := parameters{}
	err = json.Unmarshal(dat, &params)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	hash, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
	}
	user, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: hash,
	})
	if err != nil {
		respondWithError(w, 400, "User could not be created")
		log.Printf("CreateUser error: %v", err)
		return
	}
	respondWithJSON(w, 201, User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	})

}

func (cfg *apiConfig) loginUserHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	dat, err := io.ReadAll(r.Body)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	params := parameters{}
	err = json.Unmarshal(dat, &params)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	user, err := cfg.db.GetUserByEmail(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	test, err := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil {
		respondWithError(w, 500, "Checking hashes went wrong")
		return
	}
	if test != true {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	respondWithJSON(w, 200, User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	})

}

func (cfg *apiConfig) getChirpHanlder(w http.ResponseWriter, r *http.Request) {
	chirpID, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
	}
	chirp, err := cfg.db.GetChirp(r.Context(), chirpID)
	if err != nil {
		respondWithError(w, 404, "Chirp not found")
	}
	respondWithJSON(w, 200, Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	})
}

func (cfg *apiConfig) getChirpsHandler(w http.ResponseWriter, r *http.Request) {
	chirps, err := cfg.db.GetChirps(r.Context())
	if err != nil {
		log.Printf("Error getting chirps: %v", err)
		respondWithError(w, 401, "Could not retrivie churps")
		return
	}
	fmChirps := []Chirp{}
	for _, chirp := range chirps {
		fmChirps = append(fmChirps, Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		})
	}
	respondWithJSON(w, 200, fmChirps)
}
func (cfg *apiConfig) createChirpHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}

	dat, err := io.ReadAll(r.Body)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	params := parameters{}
	err = json.Unmarshal(dat, &params)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	if len(params.Body) < 140 {
		params.Body = cleanBody(params.Body)
		chirp, err := cfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
			Body:   params.Body,
			UserID: params.UserID,
		})
		if err != nil {
			log.Printf("Chirps error: %v", err)
		}
		respondWithJSON(w, 201, Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		})
	} else {
		respondWithError(w, 400, "Chirp is too long")
	}
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`
	<html>
  		<body>
    		<h1>Welcome, Chirpy Admin</h1>
    		<p>Chirpy has been visited %d times!</p>
  		</body>
	</html>`, cfg.getHits())))
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) error {
	response, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	w.Write(response)
	return nil
}

func respondWithError(w http.ResponseWriter, code int, msg string) error {
	return respondWithJSON(w, code, map[string]string{"error": msg})
}

func cleanBody(b string) string {
	profane := []string{"kerfuffle", "sharbert", "fornax"}
	sSlice := strings.Split(b, " ")
	for i, word := range sSlice {
		for _, cword := range profane {
			if strings.ToLower(word) == cword {
				sSlice[i] = "****"
			}
		}
	}
	return strings.Join(sSlice, " ")
}
