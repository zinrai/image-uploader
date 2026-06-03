# image-uploader

image-uploader is a Go-based web application that provides image upload, thumbnail generation, and display functionality. Data is stored in a single bbolt database file.

## Features

- Image upload (supports JPG and PNG)
- Automatic thumbnail generation for uploaded images
- Display of recently uploaded images
- Prevention of duplicate image uploads
- Cleanup command to remove old images exceeding the retention limit
- Block/unblock commands for SHA-256-based takedown of specific images

## Commands

### serve

Start the HTTP server.

```
$ ./image-uploader serve
```

### cleanup

Remove images exceeding the retention limit (960 images). Images beyond the limit are deleted from the database and the filesystem, oldest first.

```
$ ./image-uploader cleanup
```

With Docker Compose:

```
$ docker compose run --rm cleanup
```

To run periodically via cron:

```
*/10 * * * * cd /path/to/project && docker compose run --rm cleanup
```

### block

Delete an image identified by its SHA-256 from the database and filesystem, and register the SHA-256 to the blocklist so it cannot be uploaded again. Fails if the SHA-256 is not present in the database.

```
$ ./image-uploader block <sha256>
```

With Docker Compose:

```
$ SHA=<sha256> docker compose run --rm block
```

### unblock

Remove a SHA-256 from the blocklist, allowing the image to be uploaded again.

```
$ ./image-uploader unblock <sha256>
```

With Docker Compose:

```
$ SHA=<sha256> docker compose run --rm unblock
```

## Configuration

Configuration is done via command-line flags:

| Flag | Default | Commands | Description |
|---|---|---|---|
| `--db-path` | `./data.db` | `serve`, `cleanup`, `block`, `unblock` | Path to the bbolt database file |
| `--listen-addr` | `:8080` | `serve` | Listen address |

## Usage

- Start the server: `./image-uploader serve`
- To upload an image: Send a POST request to the `/upload` endpoint with the image file as multipart form data.
- To view the image list: Access http://localhost:8080/ in your browser.
- Remove old images: `./image-uploader cleanup`

## License

This project is licensed under the [MIT License](./LICENSE).
