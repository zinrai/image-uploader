# image-uploader

image-uploader is a Go-based web application that provides image upload, thumbnail generation, and display functionality. Data is stored in a single bbolt database file.

## Features

- Image upload (supports JPG and PNG)
- Automatic thumbnail generation for uploaded images
- Display of recently uploaded images
- Prevention of duplicate image uploads
- Cleanup command to remove old images exceeding the retention limit

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

## Configuration

Configuration is done via command-line flags:

| Flag | Default | Commands | Description |
|---|---|---|---|
| `--db-path` | `./data.db` | `serve`, `cleanup` | Path to the bbolt database file |
| `--listen-addr` | `:8080` | `serve` | Listen address |

## Usage

- Start the server: `./image-uploader serve`
- To upload an image: Send a POST request to the `/upload` endpoint with the image file as multipart form data.
- To view the image list: Access http://localhost:8080/ in your browser.
- Remove old images: `./image-uploader cleanup`

## License

This project is licensed under the [MIT License](./LICENSE).
