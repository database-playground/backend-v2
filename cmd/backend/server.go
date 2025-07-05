package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/graph"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/joho/godotenv"
	"github.com/redis/rueidis"
	"github.com/vektah/gqlparser/v2/ast"

	_ "github.com/database-playground/backend-v2/ent/runtime"
	_ "github.com/mattn/go-sqlite3"
)

const defaultPort = "8080"

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	// Create ent.Client and run the schema migration.
	client, err := ent.Open(dialect.SQLite, "file:ent?mode=memory&cache=shared&_fk=1")
	if err != nil {
		log.Fatal("opening ent client", err)
	}
	if err := client.Schema.Create(
		context.Background(),
	); err != nil {
		log.Fatal("opening ent client", err)
	}

	redisClient, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress: strings.Split(os.Getenv("REDIS_ADDRESSES"), ","),
		Username:    os.Getenv("REDIS_USERNAME"),
		Password:    os.Getenv("REDIS_PASSWORD"),
	})
	if err != nil {
		log.Fatal("opening redis client", err)
	}

	storage := auth.NewRedisStorage(redisClient)
	authMiddleware := auth.Middleware(storage)

	srv := handler.New(graph.NewSchema(client, storage))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	srv.SetErrorPresenter(graph.NewErrorPresenter())

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", authMiddleware(srv))

	// FIXME: IT MUST BE DELETED IN PRODUCTION!!
	http.HandleFunc("POST /auth-as-admin", func(w http.ResponseWriter, r *http.Request) {
		token, err := storage.Create(r.Context(), auth.TokenInfo{
			Machine: r.UserAgent(),
			User:    "1",
			Scopes:  []string{"*"},
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     auth.CookieAuthToken,
			Value:    token,
			Path:     "/",
			MaxAge:   auth.DefaultTokenExpire, // 8 hours
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})

		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("POST /logout", func(w http.ResponseWriter, r *http.Request) {
		// get auth-token from cookie
		cookie, err := r.Cookie("auth-token")
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		err = storage.Delete(r.Context(), cookie.Value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     auth.CookieAuthToken,
			Value:    "",
			Path:     "/",
			Expires:  time.Now().Add(-time.Second),
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
	})

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
