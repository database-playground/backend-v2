package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"entgo.io/ent/dialect"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/graph"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/redis/rueidis"
	"github.com/vektah/gqlparser/v2/ast"

	_ "github.com/database-playground/backend-v2/ent/runtime"
	_ "github.com/mattn/go-sqlite3"
)

const defaultPort = "8080"

func main() {
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

	srv := handler.New(graph.NewSchema(client))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

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

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", authMiddleware(srv))

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
