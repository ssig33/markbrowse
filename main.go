package main

import (
	"flag"
	"fmt"
	"log"
	"os"

)

func main() {
	var port int
	flag.IntVar(&port, "port", 8080, "Port to run the server on")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [--port <port>] <directory>\n", os.Args[0])
		os.Exit(1)
	}

	rootDir := args[0]

	// Check if directory exists
	info, err := os.Stat(rootDir)
	if err != nil {
		log.Fatalf("Error accessing directory %s: %v", rootDir, err)
	}
	if !info.IsDir() {
		log.Fatalf("%s is not a directory", rootDir)
	}

	fmt.Printf("Starting markbrowse server on port %d for directory: %s\n", port, rootDir)
	if err := Start(port, rootDir); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}