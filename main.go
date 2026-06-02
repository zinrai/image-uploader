package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"

	bolt "go.etcd.io/bbolt"
)

const (
	uploadDir      = "./image"
	thumbnailDir   = "./thumb"
	maxUploadSize  = 40 * 1024 * 1024 // 40MB
	thumbnailSize  = 120
	displayLimit   = 480
	retentionLimit = displayLimit * 2 // buffer so cleanup doesn't thin the displayed page
	defaultDBPath  = "./data.db"
	defaultAddr    = ":8080"
)

//go:embed templates/*.html
var templateFS embed.FS

var (
	db    *bolt.DB
	tmpls *template.Template
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n\nCommands:\n  serve    Start the HTTP server\n  cleanup  Remove images exceeding the retention limit\n", os.Args[0])
	os.Exit(1)
}

func initDB(dbPath string) {
	var err error
	db, err = bolt.Open(dbPath, 0600, nil)
	if err != nil {
		slog.Error("failed to open database", "path", dbPath, "error", err)
		os.Exit(1)
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketImages)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketSHA256)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		slog.Error("failed to initialize buckets", "error", err)
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "serve":
		runServe(os.Args[2:])
	case "cleanup":
		runCleanup(os.Args[2:])
	default:
		usage()
	}
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	dbPath := fs.String("db-path", defaultDBPath, "Path to the bbolt database file")
	listenAddr := fs.String("listen-addr", defaultAddr, "Listen address")
	fs.Parse(args)

	initDB(*dbPath)
	defer db.Close()

	var err error
	tmpls, err = template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		slog.Error("failed to parse templates", "error", err)
		os.Exit(1)
	}

	for _, dir := range []string{uploadDir, thumbnailDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			slog.Error("failed to create directory", "dir", dir, "error", err)
			os.Exit(1)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /upload", uploadHandler)
	mux.HandleFunc("GET /{$}", indexHandler)
	mux.Handle("GET /image/", http.StripPrefix("/image/", http.FileServer(http.Dir(uploadDir))))
	mux.Handle("GET /thumb/", http.StripPrefix("/thumb/", http.FileServer(http.Dir(thumbnailDir))))

	slog.Info("starting server", "addr", *listenAddr)
	if err := http.ListenAndServe(*listenAddr, mux); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func runCleanup(args []string) {
	fs := flag.NewFlagSet("cleanup", flag.ExitOnError)
	dbPath := fs.String("db-path", defaultDBPath, "Path to the bbolt database file")
	fs.Parse(args)

	initDB(*dbPath)
	defer db.Close()

	deleted, err := cleanupOldImages(retentionLimit)
	if err != nil {
		slog.Error("cleanup failed", "error", err)
		os.Exit(1)
	}

	slog.Info("cleanup complete", "deleted", deleted)
}
