package prosodyhttpauthmastodon_test

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	prosodyhttpauthmastodon "go.nadia.moe/prosody-http-auth-mastodon"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestServer(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	if deadline, hasIt := t.Deadline(); hasIt {
		ctx, cancel = context.WithDeadline(ctx, deadline)
	}
	t.Cleanup(cancel)

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "postgres:17.0",
			Env: map[string]string{
				"POSTGRES_USER":     "test",
				"POSTGRES_PASSWORD": "test",
				"POSTGRES_DB":       "test",
			},
			ExposedPorts: []string{"5432"},
			WaitingFor: wait.ForAll(
				wait.ForExposedPort(),
				wait.ForLog("database system is ready to accept connections"),
			),
		},
	})
	if err != nil {
		t.Fatalf("creating postgres container: %v", err)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("mapping port: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("getting container host: %v", err)
	}

	t.Cleanup(func() {
		_ = container.Stop(ctx, nil)
	})

	dbConn := fmt.Sprintf("postgresql://test:test@%s/test?sslmode=disable", net.JoinHostPort(host, port.Port()))
	db, err := sql.Open("postgres", dbConn)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}

	err = db.Ping()
	if err != nil {
		t.Fatalf("pinging db: %v", err)
	}

	row := db.QueryRowContext(ctx,
		"CREATE TABLE users (email varchar(255) NOT NULL, encrypted_password varchar(255) NOT NULL)",
	)
	if row.Err() != nil {
		t.Fatalf("creating table: %v", err)
	}

	row = db.QueryRowContext(ctx,
		"INSERT INTO users VALUES ('admin@owo.cafe', '$2y$10$jRO9TrmycLZQZqHJpr8F4ezOCh6EVDpenyZJYceHhGuDRyBvARFl6')", // bcrypt('nya nya uwu')
	)
	if row.Err() != nil {
		t.Fatalf("creating table: %v", err)
	}

	err = db.Close()
	if err != nil {
		t.Fatalf("closing conn: %v", err)
	}

	t.Log(dbConn)

	authServer := &prosodyhttpauthmastodon.Server{Domain: "owo.cafe"}
	err = authServer.Start(dbConn)
	if err != nil {
		t.Fatalf("auth server connecting to DB: %v", err)
	}

	server := httptest.NewServer(authServer)
	t.Cleanup(server.Close)

	t.Run("health", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/health")
		if err != nil {
			t.Fatalf("requesting /health: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("auth server did not pass heath: status %d", resp.StatusCode)
		}
	})

	t.Run("no content-type", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequest(http.MethodPost, server.URL+"/auth", nil)
		if err != nil {
			t.Fatalf("creating request: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("requesting /auth: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("unexpected status code %d", resp.StatusCode)
		}
	})

	t.Run("bad body", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequest(http.MethodPost, server.URL+"/auth", strings.NewReader(`{"foo":"bar"}`))
		if err != nil {
			t.Fatalf("creating request: %v", err)
		}

		req.Header.Add("content-type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("requesting /auth: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("unexpected status code %d", resp.StatusCode)
		}
	})

	for _, tc := range []struct {
		name         string
		username     string
		password     string
		expectedCode int
	}{
		{
			name:         "empty user",
			username:     "",
			password:     "uwu",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "empty password",
			username:     "admin@owo.cafe",
			password:     "",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "out of domain",
			username:     "perico@palot.es",
			password:     "foo",
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "non-existing user",
			username:     "who@owo.cafe",
			password:     "foo",
			expectedCode: http.StatusNotFound,
		},
		{
			name:         "wrong password",
			username:     "admin@owo.cafe",
			password:     "uwu",
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "ok password",
			username:     "admin@owo.cafe",
			password:     "nya nya uwu",
			expectedCode: http.StatusOK,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := fmt.Sprintf(`{"username":%q,"password":%q}`, tc.username, tc.password)
			req, err := http.NewRequest(http.MethodPost, server.URL+"/auth", strings.NewReader(body))
			if err != nil {
				t.Fatalf("creating request: %v", err)
			}

			req.Header.Add("content-type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("requesting /auth: %v", err)
			}
			resp.Body.Close()

			if resp.StatusCode != tc.expectedCode {
				t.Fatalf("expected code %d, got %d", tc.expectedCode, resp.StatusCode)
			}
		})
	}
}
