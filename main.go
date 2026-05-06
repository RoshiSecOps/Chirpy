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
	apiCfg := apiConfig{db: dbQueries, platform: os.Getenv("PLATFORM"), secret: os.Getenv("JWT_SECRET")}
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
	mux.HandleFunc("PUT /api/users", apiCfg.editUserHandler)
	mux.HandleFunc("POST /api/login", apiCfg.loginUserHandler)
	mux.HandleFunc("POST /api/refresh", apiCfg.refreshTokenHandler)
	mux.HandleFunc("POST /api/revoke", apiCfg.refreshTokenRevokeHandler)
	mux.HandleFunc("GET /api/chirps", apiCfg.getChirpsHandler)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.getChirpHanlder)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.deleteChirpHandler)
	mux.HandleFunc("POST /api/polka/webhooks", apiCfg.upgradeUserHandler)

	server := http.Server{Addr: ":8080", Handler: mux}
	log.Fatal(server.ListenAndServe())

}

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	secret         string
}
type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	IsRed     bool      `json:"is_chirpy_red"`
}

type UserToken struct {
	User
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
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

func (cfg *apiConfig) deleteChirpHandler(w http.ResponseWriter, r *http.Request) {
	jToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, "could not get auth token from request")
		return
	}
	userId, err := auth.ValidateJWT(jToken, cfg.secret)
	if err != nil {
		respondWithError(w, 403, "Could not validate token")
		return
	}
	chirpID, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	chirp, err := cfg.db.GetChirp(r.Context(), chirpID)
	if err != nil {
		respondWithError(w, 404, "Chirp not found")
		return
	}
	if userId != chirp.UserID {
		respondWithError(w, 403, "forbidden")
		return
	}
	err = cfg.db.RemoveChirpById(r.Context(), chirpID)
	if err != nil {
		respondWithError(w, 500, "could not delete chirp")
		return
	}
	respondWithJSON(w, 204, "Success")
}
func (cfg *apiConfig) upgradeUserHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Event string `json:"event"`
		Data  struct {
			UserId uuid.UUID `json:"user_id"`
		} `json:"data"`
	}
	akey, err := auth.GetAPIKey(r.Header)
	if err != nil {
		respondWithError(w, 401, "")
	}
	if akey != os.Getenv("POLKA_KEY") {
		respondWithError(w, 401, "")
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
	if params.Event != "user.upgrade" {
		respondWithJSON(w, 204, "")
	}
	err = cfg.db.UpgradeUser(r.Context(), params.Data.UserId)
	if err != nil {
		respondWithError(w, 404, "Not Found")
	}
	respondWithJSON(w, 204, "")
}

func (cfg *apiConfig) editUserHandler(w http.ResponseWriter, r *http.Request) {
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
	jToken, err := auth.GetBearerToken(r.Header)
	userId, err := auth.ValidateJWT(jToken, cfg.secret)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	hashedPass, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, 500, "Pass was not hashed")
		return
	}
	user, err := cfg.db.EditUserById(r.Context(), database.EditUserByIdParams{
		Email: params.Email, HashedPassword: hashedPass, ID: userId,
	})
	if err != nil {
		respondWithError(w, 500, "Failed editing user")
	}
	respondWithJSON(w, 200, User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
		IsRed:     user.IsChirpyRed,
	})
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
		IsRed:     user.IsChirpyRed,
	})

}

func (cfg *apiConfig) refreshTokenRevokeHandler(w http.ResponseWriter, r *http.Request) {
	rtoken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, "Could not parse token")
		return
	}
	err = cfg.db.RevokeToken(r.Context(), rtoken)
	if err != nil {
		respondWithError(w, 401, "Could not revoke token")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	type JToken struct {
		Token string `json:"token"`
	}
	rtoken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 500, "Could not parse token")
		return
	}
	user, err := cfg.db.GetUserFromRefreshToken(r.Context(), rtoken)
	if err != nil {
		respondWithError(w, 401, "Could not get user by token")
		return
	}
	id := user.ID
	jtoken, err := auth.MakeJWT(id, cfg.secret, time.Hour)
	if err != nil {
		respondWithError(w, 500, "Could not generate JWT")
		return
	}
	respondWithJSON(w, 200, JToken{
		Token: jtoken,
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
	expiration := time.Hour
	rToken, err := auth.MakeRefreshToken()
	if err != nil {
		respondWithError(w, 500, "could not get refresh token")
	}
	token, err := auth.MakeJWT(user.ID, cfg.secret, expiration)
	cfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token: rToken, UserID: user.ID,
	})
	if err != nil {
		respondWithError(w, 500, "could not create token")
	}
	respondWithJSON(w, 200, UserToken{
		User: User{
			ID:        user.ID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Email:     user.Email,
			IsRed:     user.IsChirpyRed},
		Token:        token,
		RefreshToken: rToken,
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
	s := r.URL.Query().Get("author_id")
	if s != "" {
		userId, err := uuid.Parse(s)
		if err != nil {
			respondWithError(w, 401, "")
			return
		}
		chirps, err := cfg.db.GetChirpsByUserId(r.Context(), userId)
		if err != nil {
			respondWithError(w, 401, "")
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
		return
	}
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
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 500, "Error getting auth header")
		return
	}
	tokenId, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	if len(params.Body) < 140 {
		params.Body = cleanBody(params.Body)
		chirp, err := cfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
			Body:   params.Body,
			UserID: tokenId,
		})
		if err != nil {
			log.Printf("Chirps error: %v", err)
		}
		respondWithJSON(w, 201, Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    tokenId,
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
