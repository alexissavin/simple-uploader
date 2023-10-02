package main

import (
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"
	"errors"
	"os/signal"
	"syscall"
	"os"
)

var logger *logrus.Logger

func run(args []string) int {
	bindAddress := flag.String("ip", "0.0.0.0", "IP address to bind")
	listenPort := flag.Int("port", 8080, "port number to listen on")
	tlsListenPort := flag.Int("tlsport", 8443, "port number to listen on with TLS")
	// maxUploadSize := flag.Int64("upload_limit", 67108864, "max size of uploaded file (bytes), default 64MB")
	maxUploadSize := flag.Int64("upload_limit", 134217728, "max size of uploaded file (bytes), default 128MB")
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
		logger.WithError(err).Error("failed to parse logging level, so set to default")
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

	http.Handle("/status", server)
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

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-signalChan:
		logger.Infof("Received signal: %v", sig)
		// Add any necessary cleanup or shutdown logic here.
		// For example, gracefully close open connections or files.
		//ctx, cancel := context.WithTimeout(context.Background(), 8 * time.Second)

		//defer func() {
			// extra handling here if needed
			//cancel()
		//}()

		//if err := http.Shutdown(ctx); err != nil {
		//	logger.Fatalf("Simple-upload-server Shutdown Failed:%+v", err)
		//}
		logger.Print("Simple-upload-server Exited")

		// Then exit the program.
		return 0
	case err := <-errors:
		logger.WithError(err).Info("Simple-upload-server Exited with Error(s)")
		return 1
	}
}

func main() {
	logger = logrus.New()
	logger.Info("Starting up simple-upload-server")

	result := run(os.Args)
	os.Exit(result)
}