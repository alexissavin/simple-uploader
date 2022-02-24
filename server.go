package main

import (
	"bufio"
	"crypto/sha1"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"path/filepath"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

var (
	rePathUpload       = regexp.MustCompile(`^/upload$`)
	errTokenMismatch   = errors.New("token mismatched")
	errMissingToken    = errors.New("missing token")
	errInvalidToken    = errors.New("invalid token format")
	errTooManyAttempts = errors.New("too many failed connection attempts")
)

// FailedConnectionTracker
type fct struct {
	attempts int64
	last     int64
}

// Server represents a simple-upload server.
type Server struct {
	DocumentRoot string
	// MaxUploadSize limits the size of the uploaded content, specified with "byte".
	MaxUploadSize    int64
	SecureTokens     []string
	EnableCORS       bool
	MaxAttempts      int64
	FailedConTracker map[string]fct
}

// Read the tokens file
func LoadTokens(tokens_file string) []string {
	file, err := os.Open(tokens_file)
	res := make([]string, 0)

	if err != nil {
		logger.WithError(err).Fatal("Unable to open tokens file")
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		token := strings.TrimRight(scanner.Text(), "\n")
		res = append(res, token)
	}

	file.Close()

	return (res)
}

// NewServer creates a new simple-upload server.
func NewServer(documentRoot string, maxUploadSize int64, token_file string, enableCORS bool, MaxAttempts int64) Server {
	return Server{
		DocumentRoot:     documentRoot,
		MaxUploadSize:    maxUploadSize,
		SecureTokens:     LoadTokens(token_file),
		EnableCORS:       enableCORS,
		MaxAttempts:      MaxAttempts,
		FailedConTracker: make(map[string]fct),
	}
}

func (s Server) handlePost(w http.ResponseWriter, r *http.Request) {
	// Retrieve the token from the query strings
	token := r.URL.Query().Get("token")
	// If empty attempt to retrieve the token within the form
	if token == "" {
		token = r.FormValue("token")
	}
	// Create the directory for the given token
	uploadDir := path.Join(s.DocumentRoot, token)

	if _, err := os.Stat(uploadDir); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(uploadDir, os.ModePerm)
		if err != nil {
			logger.WithError(err).Error("Failed to create upload directory for the given token")
			w.WriteHeader(http.StatusInternalServerError)
			writeError(w, err)
			return
		}
	}
	// Retrieve the form file
	srcFile, info, err := r.FormFile("file")
	if err != nil {
		logger.WithError(err).Error("Failed to acquire the uploaded content")
		w.WriteHeader(http.StatusInternalServerError)
		writeError(w, err)
		return
	}
	defer srcFile.Close()
	logger.Debug(info)
	size, err := getSize(srcFile)
	if err != nil {
		logger.WithError(err).Error("Failed to get the size of the uploaded content")
		w.WriteHeader(http.StatusInternalServerError)
		writeError(w, err)
		return
	}
	if size > s.MaxUploadSize {
		logger.WithField("size", size).Info("File size exceeded")
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		writeError(w, errors.New("Uploaded file size exceeds the limit"))
		return
	}
	body, err := ioutil.ReadAll(srcFile)
	if err != nil {
		logger.WithError(err).Error("Failed to read the uploaded content")
		w.WriteHeader(http.StatusInternalServerError)
		writeError(w, err)
		return
	}
	filename := info.Filename
	if filename == "" {
		filename = fmt.Sprintf("%x", sha1.Sum(body))
	}
	dstPath := path.Join(uploadDir, filepath.Base(filename))
	dstFile, err := os.OpenFile(dstPath + ".tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		logger.WithError(err).WithField("path", dstPath).Error("Failed to open the file")
		w.WriteHeader(http.StatusInternalServerError)
		writeError(w, err)
		return
	}
	//FIXME replace the following statement to close the file explicitely and rename it once uploaded
	//defer dstFile.Close()
	if written, err := dstFile.Write(body); err != nil {
		logger.WithError(err).WithField("path", dstPath).Error("Failed to write file content")
		w.WriteHeader(http.StatusInternalServerError)
		writeError(w, err)
		dstFile.Close()
		return
	} else if int64(written) != size {
		dstFile.Close()
		logger.WithFields(logrus.Fields{
			"size":    size,
			"written": written,
		}).Error("Uploaded file size and written size differ")
		w.WriteHeader(http.StatusInternalServerError)
		writeError(w, fmt.Errorf("The size of uploaded content is %d, but %d bytes written", size, written))
	}
	dstFile.Close()
	err = os.Rename(dstPath + ".tmp", dstPath)
  if err != nil {
  	logger.Error("Unable to rename temporary upload file")
  }
	uploadedURL := strings.TrimPrefix(dstPath, s.DocumentRoot)
	if !strings.HasPrefix(uploadedURL, "/") {
		uploadedURL = "/" + uploadedURL
	}
	uploadedURL = "/files" + uploadedURL
	logger.WithFields(logrus.Fields{
		"path": dstPath,
		"url":  uploadedURL,
		"size": size,
	}).Info("File uploaded by POST")
	if s.EnableCORS {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.WriteHeader(http.StatusOK)
	writeSuccess(w, uploadedURL)
}

func (s Server) handleOptions(w http.ResponseWriter, r *http.Request) {
	var allowedMethods []string
	if rePathUpload.MatchString(r.URL.Path) {
		allowedMethods = []string{http.MethodPost}
	} else {
		w.WriteHeader(http.StatusNotFound)
		writeError(w, errors.New("Not found"))
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(allowedMethods, ","))
	w.WriteHeader(http.StatusNoContent)
}

func getSrcIP(r *http.Request) (string, error) {
	//Get IP from the X-REAL-IP header
	ip := r.Header.Get("X-REAL-IP")
	netIP := net.ParseIP(ip)
	if netIP != nil {
		return ip, nil
	}

	//Get IP from X-FORWARDED-FOR header
	ips := r.Header.Get("X-FORWARDED-FOR")
	splitIps := strings.Split(ips, ",")
	for _, ip := range splitIps {
		netIP := net.ParseIP(ip)
		if netIP != nil {
			return ip, nil
		}
	}

	//Get IP from RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}
	netIP = net.ParseIP(ip)
	if netIP != nil {
		return ip, nil
	}
	return "", fmt.Errorf("No valid IP found")
}

func (s Server) checkToken(r *http.Request) error {
	//FIXME Ensure client IP is not blacklisted
	//FIXME If getSrcIP(r) is in the list and for less than X Minute, do not answer
	srcIP, srcIPError := getSrcIP(r)

	if srcIPError == nil {
		connectionTime := time.Now().Unix()
		tracker, trackerExists := s.FailedConTracker[srcIP]

		if trackerExists {
			tracker.last = connectionTime
			tracker.attempts = tracker.attempts + 1
			s.FailedConTracker[srcIP] = tracker
			if tracker.attempts > s.MaxAttempts && tracker.last > (connectionTime-300) {
				logger.WithFields(logrus.Fields{
					"srcIP":    srcIP,
					"attempts": tracker.attempts,
				}).Error("Too many connection attempts from the following client")
				time.Sleep(time.Second * 4)
				return errTooManyAttempts
			}
		} else {
			s.FailedConTracker[srcIP] = fct{
				last:     connectionTime,
				attempts: 1,
			}
		}
	} else {
		logger.Error("Connection attempt from non identified source")
	}

	// Retrieve the token from the query strings
	token := r.URL.Query().Get("token")
	// If empty attempt to retrieve the token within the form
	if token == "" {
		token = r.FormValue("token")
	}
	// If still empty, token is missing from the query
	if token == "" {
		return errMissingToken
	}
	// Validate the token format and validity
	match, _ := regexp.MatchString("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", token)
	if match {
		for _, t := range s.SecureTokens {
			if token == t {
				if srcIPError == nil {
					delete(s.FailedConTracker, srcIP)
				}
				return nil
			}
		}

		return errTokenMismatch
	}

	return errInvalidToken
}

func (s Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := s.checkToken(r); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		writeError(w, err)
		return
	}
	switch r.Method {
	case http.MethodOptions:
		s.handleOptions(w, r)
	case http.MethodPost:
		s.handlePost(w, r)
	default:
		w.Header().Add("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		writeError(w, fmt.Errorf("HTTP Method \"%s\" is not allowed", r.Method))
	}
}
