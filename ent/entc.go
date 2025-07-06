//go:build ignore
// +build ignore

package main

import (
	"log"

	"entgo.io/contrib/entgql"
	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

// it must be executed from ../generate.go

func main() {
	ex, err := entgql.NewExtension(
		entgql.WithSchemaGenerator(),
		entgql.WithSchemaPath("./graph/ent.graphqls"),
		entgql.WithConfigPath("./gqlgen.yml"),
	)
	if err != nil {
		log.Fatalf("creating entgql extension: %v", err)
	}
	if err := entc.Generate("./ent/schema", &gen.Config{}, entc.Extensions(ex), entc.FeatureNames(
		"intercept",
		"schema/snapshot",
		"sql/globalid",
	)); err != nil {
		log.Fatalf("running ent codegen: %v", err)
	}
}
