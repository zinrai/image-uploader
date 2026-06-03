package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var errNotFound = errors.New("sha256 not found in database")

const (
	bucketImages    = "images"
	bucketSHA256    = "sha256"
	bucketBlocklist = "blocklist"
)

type ImageInfo struct {
	Filename          string `json:"filename"`
	ThumbnailFilename string `json:"thumbnail_filename"`
	SHA256Sum         string `json:"sha256sum"`
	ThumbnailPath     string `json:"-"`
	ImagePath         string `json:"-"`
}

func makeKey() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func findBySHA256(sha256sum string) (string, error) {
	var filename string
	err := db.View(func(tx *bolt.Tx) error {
		sha := tx.Bucket([]byte(bucketSHA256))
		imageKey := sha.Get([]byte(sha256sum))
		if imageKey == nil {
			return nil
		}

		images := tx.Bucket([]byte(bucketImages))
		data := images.Get(imageKey)
		if data == nil {
			return nil
		}

		var info ImageInfo
		if err := json.Unmarshal(data, &info); err != nil {
			return err
		}
		filename = info.Filename
		return nil
	})
	return filename, err
}

func saveToDatabase(filename, thumbnailFilename, sha256sum string) error {
	info := ImageInfo{
		Filename:          filename,
		ThumbnailFilename: thumbnailFilename,
		SHA256Sum:         sha256sum,
	}

	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	key := makeKey()

	return db.Update(func(tx *bolt.Tx) error {
		images := tx.Bucket([]byte(bucketImages))
		if err := images.Put([]byte(key), data); err != nil {
			return err
		}

		sha := tx.Bucket([]byte(bucketSHA256))
		return sha.Put([]byte(sha256sum), []byte(key))
	})
}

func getRecentImages(limit int) ([]ImageInfo, error) {
	var images []ImageInfo

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketImages))
		c := b.Cursor()

		count := 0
		for k, v := c.Last(); k != nil && count < limit; k, v = c.Prev() {
			var info ImageInfo
			if err := json.Unmarshal(v, &info); err != nil {
				return fmt.Errorf("unmarshal image info: %w", err)
			}
			info.ThumbnailPath = "/thumbs/" + info.ThumbnailFilename
			info.ImagePath = "/images/" + info.Filename
			images = append(images, info)
			count++
		}

		return nil
	})

	return images, err
}

func isBlocked(sha256sum string) (bool, error) {
	var blocked bool
	err := db.View(func(tx *bolt.Tx) error {
		if tx.Bucket([]byte(bucketBlocklist)).Get([]byte(sha256sum)) != nil {
			blocked = true
		}
		return nil
	})
	return blocked, err
}

func blockSHA256(sha256sum string) (filename, thumbnail string, err error) {
	err = db.Update(func(tx *bolt.Tx) error {
		sha := tx.Bucket([]byte(bucketSHA256))
		images := tx.Bucket([]byte(bucketImages))
		bl := tx.Bucket([]byte(bucketBlocklist))

		imageKey := sha.Get([]byte(sha256sum))
		if imageKey == nil {
			return errNotFound
		}
		data := images.Get(imageKey)
		if data == nil {
			return errNotFound
		}
		var info ImageInfo
		if err := json.Unmarshal(data, &info); err != nil {
			return err
		}
		filename = info.Filename
		thumbnail = info.ThumbnailFilename

		if err := images.Delete(imageKey); err != nil {
			return err
		}
		if err := sha.Delete([]byte(sha256sum)); err != nil {
			return err
		}
		return bl.Put([]byte(sha256sum), nil)
	})
	return
}

func unblockSHA256(sha256sum string) error {
	return db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucketBlocklist)).Delete([]byte(sha256sum))
	})
}

func cleanupOldImages(limit int) (int, error) {
	type expiredEntry struct {
		key      string
		sha256   string
		filename string
		thumb    string
	}

	var expired []expiredEntry

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketImages))
		c := b.Cursor()

		count := 0
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			count++
			if count <= limit {
				continue
			}

			var info ImageInfo
			if err := json.Unmarshal(v, &info); err != nil {
				return fmt.Errorf("unmarshal image info: %w", err)
			}
			expired = append(expired, expiredEntry{
				key:      string(k),
				sha256:   info.SHA256Sum,
				filename: info.Filename,
				thumb:    info.ThumbnailFilename,
			})
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	if len(expired) == 0 {
		return 0, nil
	}

	err = db.Update(func(tx *bolt.Tx) error {
		images := tx.Bucket([]byte(bucketImages))
		sha := tx.Bucket([]byte(bucketSHA256))

		for _, e := range expired {
			if err := images.Delete([]byte(e.key)); err != nil {
				return err
			}
			if err := sha.Delete([]byte(e.sha256)); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("delete db entries: %w", err)
	}

	for _, e := range expired {
		os.Remove(filepath.Join(uploadDir, e.filename))
		os.Remove(filepath.Join(thumbnailDir, e.thumb))
		slog.Info("deleted image", "filename", e.filename)
	}

	return len(expired), nil
}
