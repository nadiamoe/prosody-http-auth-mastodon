package prosodyhttpauthmastodon

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	db  *sql.DB
	mux *http.ServeMux
}

func (s *Server) Start(conn string) error {
	db, err := sql.Open("postgres", conn)
	if err != nil {
		return fmt.Errorf("opening sql connection: %w", err)
	}

	s.db = db

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("GET /health", s.health)
	s.mux.HandleFunc("POST /auth", s.auth)
	return nil
}

type ProsodyAuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (r *ProsodyAuthRequest) Empty() bool {
	return r.Username == "" || r.Password == ""
}

func (s *Server) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(rw, r)
}

func (s *Server) health(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := s.db.PingContext(ctx)
	if err != nil {
		statusLogWrite(rw, http.StatusInternalServerError, "error connecting to database: %v", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
}

func (s *Server) auth(rw http.ResponseWriter, r *http.Request) {
	authReq := ProsodyAuthRequest{}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(&authReq)
	if err != nil {
		statusLogWrite(rw, http.StatusBadRequest, "could not parse request body: %v", err)
		return
	}

	if authReq.Empty() {
		statusLogWrite(rw, http.StatusBadRequest, "empty username or password ")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err = s.db.PingContext(ctx)
	if err != nil {
		statusLogWrite(rw, http.StatusInternalServerError, "error connecting to database: %v", err)
		return
	}

	var hash string
	err = s.db.QueryRowContext(
		ctx,
		"SELECT encrypted_password FROM users WHERE account_id = (SELECT id FROM accounts WHERE username = lower($1) AND domain IS NULL)",
		authReq.Username,
	).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		statusLogWrite(rw, http.StatusNotFound, "No record found for user %q", authReq.Username)
		return
	} else if err != nil {
		statusLogWrite(rw, http.StatusInternalServerError, "querying db: %v", err)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(authReq.Password))
	if err != nil {
		statusLogWrite(rw, http.StatusForbidden, "authentication failed for %q: %v", authReq.Username, err)
		return
	}

	statusLogWrite(rw, http.StatusOK, "authentication succeeded for %q", authReq.Username)
}

func statusLogWrite(rw http.ResponseWriter, status int, msg string, args ...any) {
	log.Printf(msg, args...)

	rw.WriteHeader(status)
	fmt.Fprintf(rw, msg, args...)
}
