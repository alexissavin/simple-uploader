package main

import (
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
)

var logger *logrus.Logger

func run(args []string) int {
	bindAddress := flag.String("ip", "0.0.0.0", "IP address to bind")
	listenPort := flag.Int("port", 8080, "port number to listen on")
	tlsListenPort := flag.Int("tlsport", 8443, "port number to listen on with TLS")
	// 5,242,880 bytes == 5 MiB
	maxUploadSize := flag.Int64("upload_limit", 5242880, "max size of uploaded file (byte)")
	tokensFlag := flag.String("tokens", "/etc/simple_uploader/tokens", "specify the file containing the security tokens")
	maxattempts := flag.Int64("maxattempts", 3, "specify the maximum failed connection attempts")
	logLevelFlag := flag.String("loglevel", "info", "logging level")
	certFile := flag.String("cert", "", "path to certificate file")
	keyFile := flag.String("key", "", "path to key file")
	corsEnabled := flag.Bool("cors", false, "if true, add ACAO header to support CORS")
	helpRequested := flag.Bool("help", false, "display the usage")
	flag.Parse()

	if *helpRequested == true {
		flag.Usage()
		return 2
	}

	serverRoot := flag.Arg(0)
	if len(serverRoot) == 0 {
		serverRoot = "/var/html/simple_uploader/data"

		if _, err := os.Stat(serverRoot); errors.Is(err, os.ErrNotExist) {
			logger.Fatal("Missing Data Directory")
			return 1
		}
	}

	if logLevel, err := logrus.ParseLevel(*logLevelFlag); err != nil {
		logrus.WithError(err).Error("failed to parse logging level, so set to default")
	} else {
		logger.Level = logLevel
	}

	tokensFile := *tokensFlag
	if tokensFile == "" {
		logger.Fatal("Missing Tokens File")
		return 1
	}

	tlsEnabled := *certFile != "" && *keyFile != ""

	server := NewServer(serverRoot, *maxUploadSize, tokensFile, *corsEnabled, *maxattempts)

	http.Handle("/upload", server)

	errors := make(chan error)

	go func() {
		logger.WithFields(logrus.Fields{
			"ip":           *bindAddress,
			"port":         *listenPort,
			"tokensfile":   tokensFile,
			"upload_limit": *maxUploadSize,
			"root":         serverRoot,
			"cors":         *corsEnabled,
		}).Info("Start Listening")

		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", *bindAddress, *listenPort), nil); err != nil {
			errors <- err
		}
	}()

	if tlsEnabled {
		go func() {
			logger.WithFields(logrus.Fields{
				"cert": *certFile,
				"key":  *keyFile,
				"port": *tlsListenPort,
			}).Info("Start listening TLS")

			if err := http.ListenAndServeTLS(fmt.Sprintf("%s:%d", *bindAddress, *tlsListenPort), *certFile, *keyFile, nil); err != nil {
				errors <- err
			}
		}()
	}

	err := <-errors
	logger.WithError(err).Info("Closing simple-upload-server")

	return 0
}

func main() {
	logger = logrus.New()
	logger.Info("Starting up simple-upload-server")

	result := run(os.Args)
	os.Exit(result)
}
