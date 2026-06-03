package main

import (
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

func processFile(fh *multipart.FileHeader) (string, int, string) {
	if fh.Size > maxUploadSize {
		return "file too large", http.StatusBadRequest, ""
	}

	src, err := fh.Open()
	if err != nil {
		return "failed to open file", http.StatusInternalServerError, ""
	}
	defer src.Close()

	buf := make([]byte, 512)
	if _, err := src.Read(buf); err != nil {
		return "failed to read file", http.StatusInternalServerError, ""
	}
	contentType := http.DetectContentType(buf)
	if contentType != "image/jpeg" && contentType != "image/png" {
		return "file type not allowed, only JPG and PNG are allowed", http.StatusBadRequest, ""
	}

	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return "failed to seek file", http.StatusInternalServerError, ""
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, src); err != nil {
		return "failed to generate SHA-256", http.StatusInternalServerError, ""
	}
	sha256sum := hex.EncodeToString(hash.Sum(nil))

	blocked, err := isBlocked(sha256sum)
	if err != nil {
		return "database error", http.StatusInternalServerError, ""
	}
	if blocked {
		return "bad request", http.StatusBadRequest, ""
	}

	existingFilename, err := findBySHA256(sha256sum)
	if err != nil {
		return "database error", http.StatusInternalServerError, ""
	}
	if existingFilename != "" {
		slog.Info("duplicate detected", "sha256", sha256sum[:12], "filename", existingFilename)
		return "", http.StatusOK, "/images/" + existingFilename
	}

	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return "failed to seek file", http.StatusInternalServerError, ""
	}

	filename := sha256sum + filepath.Ext(fh.Filename)
	if err := saveFile(src, filename); err != nil {
		return "failed to save file", http.StatusInternalServerError, ""
	}

	thumbnailFilename, err := generateThumbnail(filename)
	if err != nil {
		slog.Error("failed to generate thumbnail", "error", err)
		return "failed to generate thumbnail", http.StatusInternalServerError, ""
	}

	if err := saveToDatabase(filename, thumbnailFilename, sha256sum); err != nil {
		slog.Error("failed to save to database", "error", err)
		return "failed to save to database", http.StatusInternalServerError, ""
	}

	slog.Info("uploaded image", "filename", filename, "thumbnail", thumbnailFilename, "bytes", fh.Size)
	return "", http.StatusOK, "/images/" + filename
}

func saveFile(src io.Reader, filename string) error {
	dst, err := os.Create(filepath.Join(uploadDir, filename))
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func generateThumbnail(filename string) (string, error) {
	f, err := os.Open(filepath.Join(uploadDir, filename))
	if err != nil {
		return "", err
	}
	defer f.Close()

	srcImg, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}

	bounds := srcImg.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	// Crop to square from center, then resize
	cropSize := srcW
	if srcH < srcW {
		cropSize = srcH
	}
	x0 := (srcW - cropSize) / 2
	y0 := (srcH - cropSize) / 2

	cropped := image.NewRGBA(image.Rect(0, 0, cropSize, cropSize))
	draw.Copy(cropped, image.Point{}, srcImg, image.Rect(x0, y0, x0+cropSize, y0+cropSize), draw.Src, nil)

	thumb := image.NewRGBA(image.Rect(0, 0, thumbnailSize, thumbnailSize))
	draw.CatmullRom.Scale(thumb, thumb.Bounds(), cropped, cropped.Bounds(), draw.Over, nil)

	thumbnailFilename := sha256Name(filename) + ".jpg"
	out, err := os.Create(filepath.Join(thumbnailDir, thumbnailFilename))
	if err != nil {
		return "", err
	}
	defer out.Close()

	if err := jpeg.Encode(out, thumb, &jpeg.Options{Quality: 85}); err != nil {
		return "", err
	}

	return thumbnailFilename, nil
}

func sha256Name(filename string) string {
	ext := filepath.Ext(filename)
	return filename[:len(filename)-len(ext)]
}
