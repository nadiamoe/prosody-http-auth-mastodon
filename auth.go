package prosodyhttpauthmastodon

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	db       *sql.DB
	mux      *http.ServeMux
	selftest ProsodyAuthRequest
}

type Options struct {
	// DBURL is the URI that should be used to connect to mastodon's database.
	DBURL string
	// Selftest specifies an username and a password that, if non-empty, will be used to attempt a login when hitting
	// /health using the same logic used for /auth.
	// If this credentials fail to authenticate, following the same logic that they would if POSTed to /auth, then
	// /health will return 403 Forbidden, signaling that something is wrong with the setup.
	Selftest ProsodyAuthRequest
}

func (s *Server) Start(opts Options) error {
	db, err := sql.Open("postgres", opts.DBURL)
	if err != nil {
		return fmt.Errorf("opening sql connection: %w", err)
	}

	s.db = db

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("GET /health", s.health)
	s.mux.HandleFunc("POST /auth", s.auth)

	s.selftest = opts.Selftest

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

	if s.selftest.Username == "" {
		rw.WriteHeader(http.StatusOK)
		return
	}

	selftestReq := &bytes.Buffer{}
	err = json.NewEncoder(selftestReq).Encode(s.selftest)
	if err != nil {
		statusLogWrite(rw, http.StatusInternalServerError, "encoding selftest data: %v", err)
		return
	}

	r.Body = io.NopCloser(selftestReq)

	// "Proxy" to /auth with selftest credentials.
	s.auth(rw, r)
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
		statusLogWrite(rw, http.StatusBadRequest, "empty username or password")
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
		"SELECT encrypted_password FROM users WHERE approved = true AND disabled = false AND account_id = "+
			"(SELECT id FROM accounts WHERE username = lower($1) AND domain IS NULL)",
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

	log.Printf("authentication succeeded for %q", authReq.Username)

	// The module expects the response body to be exactly true if the username and password are correct.
	// Ref: https://modules.prosody.im/mod_auth_custom_http.html
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte("true"))
}

func statusLogWrite(rw http.ResponseWriter, status int, msg string, args ...any) {
	log.Printf(msg, args...)

	rw.WriteHeader(status)
	fmt.Fprintf(rw, msg, args...)
}
