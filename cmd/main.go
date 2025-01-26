package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	prosodyhttpauthmastodon "go.nadia.moe/prosody-http-auth-mastodon"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dbUrl := os.ExpandEnv(os.Getenv("DB_URL"))
	if dbUrl == "" {
		return errors.New("empty DB_URL")
	}

	server := &prosodyhttpauthmastodon.Server{
		Domain: os.Getenv("DOMAIN"),
	}
	err := server.Start(dbUrl)
	if err != nil {
		return fmt.Errorf("starting server: %w", err)
	}

	return http.ListenAndServe(":8080", server)
}
