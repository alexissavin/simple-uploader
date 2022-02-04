# simple_uploader
Simple HTTP server to save artifacts

# Usage

## Start Server

```
$ mkdir $HOME/tmp
$ ./simple_uploader -token f9403fc5f537b4ab332d <upload_dir>
```

(see "Security" section below for `-token` option)

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

* `OPTIONS /files/(filename)`
* `OPTIONS /upload`

```
$ curl -I 'http://localhost:25478/files/foo'
HTTP/1.1 204 No Content
Access-Control-Allow-Methods: PUT,GET,HEAD
Access-Control-Allow-Origin: *
Date: Sun, 06 Sep 2020 09:45:20 GMT

$ curl -I -XOPTIONS 'http://localhost:25478/upload'
HTTP/1.1 204 No Content
Access-Control-Allow-Methods: POST
Access-Control-Allow-Origin: *
Date: Sun, 06 Sep 2020 09:45:32 GMT
```

notes:

* Requests using `*` as a path, like as `OPTIONS * HTTP/1.1`, are not supported.
* On sending `OPTIONS` request, `token` parameter is not required.
* For `/files/(filename)` request, server replies "204 No Content" even if the specified file does not exist.


# TLS

To enable TLS support, add `-cert` and `-key` options:

```
$ ./simple_uploader -cert ./cert.pem -key ./key.pem root/
INFO[0000] starting up simple-upload-server
WARN[0000] token generated                               token=28d93c74c8589ab62b5e
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

All requests should have a `token` parameter (it can be passed as a query string or a form parameter).
The server accepts the request only when the token is matching a list of known token; otherwise, the server rejects the request and respond `401 Unauthorized`.

You can specify the server accepted tokens from the file referenced using the `-tokens` option.


## CORS

If you enable CORS support using `-cors` option, the server append `Access-Control-Allow-Origin` header to the response. This feature is disabled by default.

# Docker

```
$ docker run -p 25478:25478 -v $HOME/tmp:/var/root alexissavin/simple_uploader -token f9403fc5f537b4ab332d /var/root
```