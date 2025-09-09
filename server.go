package main

import (
	"embed"
	"fmt"
	"net/http"

	"github.com/ssig33/markbrowse/handler"
)

//go:embed template/*
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Start starts the HTTP server
func Start(port int, rootDir string) error {
	h := handler.New(rootDir, templateFS, staticFS)

	http.HandleFunc("/", h.ServeHTTP)

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Server listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, nil)
}