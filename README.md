[![pipeline status](https://gitlab.efficientip.com/data-factory/glake-simple-uploader/badges/main/pipeline.svg)](https://gitlab.efficientip.com/data-factory/glake-simple-uploader/-/commits/main)

# simple_uploader

Simple HTTP server to save artifacts

# Usage

## Start Server

```
mkdir $HOME/tmp
./simple_uploader -tokens <tokens_file> <upload_dir>
```

(see "Security" section below for `-tokens` option)

## Checking Status

You can check the status of the server using the following call:

```
$ curl 'http://localhost:25478/status?token=f9403fc5f537b4ab332d'
{"ok":true}
```

## Uploading

You can upload files with `POST /upload`.
The filename is taken from the original file if available.
If not, SHA1 hex digest will be used as the filename.
The file is uploaded in a directory named after the token used to upload it.

```
$ echo 'Hello, world!' > sample.txt
$ curl -Ffile=@sample.txt 'http://localhost:25478/upload?token=f9403fc5f537b4ab332d'
{"ok":true,"path":"/<upload_dir>/f9403fc5f537b4ab332d/sample.txt"}
```

```
$ cat /<upload_dir>/f9403fc5f537b4ab332d/sample.txt
hello, world!
```

## CORS Preflight Request

* `OPTIONS /upload`

```
$ curl -I -XOPTIONS 'http://localhost:25478/upload'
HTTP/1.1 204 No Content
Access-Control-Allow-Methods: POST
Access-Control-Allow-Origin: *
Date: Sun, 06 Sep 2020 09:45:32 GMT
```

# TLS

To enable TLS support, add `-cert` and `-key` options:

```
$ ./simple_uploader -tokens <tokens_file> -cert ./cert.pem -key ./key.pem root/
INFO[0000] starting up simple-upload-server
INFO[0000] start listening TLS                           cert=./cert.pem key=./key.pem port=25443
INFO[0000] start listening                               ip=0.0.0.0 port=25478 root=root token=28d93c74c8589ab62b5e upload_limit=5242880
...
```

This server listens on `25443/tcp` for TLS connections by default. This can be changed by passing `-tlsport` option.

NOTE: The endpoint using HTTP is still active even if TLS is enabled.

# Security

## Tokens

There is no Basic/Digest authentication.
This app implements dead simple authentication: "security token".
Tokens must be in uuid format, appended in the tokens_file.

All requests should have a `token` parameter (it can be passed as a query string or a form parameter).
The server accepts the request only when the token is matching a list of known token; otherwise, the server rejects the request and respond `401 Unauthorized`.

You can specify the server accepted tokens from the file referenced using the `-tokens` option.

## Token Brute Force Attack Mitigation

There is a basic protection against token brute force attack. Any source IP attempting to connect with an invalid token is blacklisted for 5 minutes after 3 attemtps.
You can use the `-maxattempts` option to specify another value.

## CORS

If you enable CORS support using `-cors` option, the server append `Access-Control-Allow-Origin` header to the response. This feature is disabled by default.

# Docker

```
docker build -t simple-uploader:latest .
docker run -p 8080:8080 -user `id -u`:`id -g` -v $(pwd)/data:/var/html/simple_uploader/data -v $(pwd)/tokens/tokens:/etc/simple_uploader/tokens simple-uploader:latest
```
