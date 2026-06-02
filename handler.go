package main

import (
	"fmt"
	"log/slog"
	"net/http"
)

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		http.Error(w, "no file provided", http.StatusBadRequest)
		return
	}

	var links []string

	for _, fh := range files {
		msg, code, imagePath := processFile(fh)
		if code != http.StatusOK {
			http.Error(w, fmt.Sprintf("%s: %s", fh.Filename, msg), code)
			return
		}
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		links = append(links, fmt.Sprintf("%s://%s%s", scheme, r.Host, imagePath))
	}

	w.Header().Set("Content-Type", "text/plain")
	for _, link := range links {
		fmt.Fprintln(w, link)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	images, err := getRecentImages(displayLimit)
	if err != nil {
		slog.Error("failed to fetch images", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := tmpls.ExecuteTemplate(w, "error.html", map[string]string{"Error": "Failed to fetch images"}); err != nil {
			slog.Error("failed to execute error template", "error", err)
		}
		return
	}

	if err := tmpls.ExecuteTemplate(w, "index.html", map[string]any{
		"Images": images,
	}); err != nil {
		slog.Error("failed to execute index template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
