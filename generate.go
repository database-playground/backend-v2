package main

//go:generate go run -mod=mod ./ent/entc.go
//go:generate go tool gqlgen generate
//go:generate go tool gofumpt -w ./graph
