package main

import (
	"bufio"
	"crypto/sha1"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	tokenDumpLentgh = 8
)

var (
	rePathStatus          = regexp.MustCompile(`^/status$`)
	rePathUpload          = regexp.MustCompile(`^/upload$`)
	errServerError        = errors.New("internal server error")
	errTokenMismatch      = errors.New("token mismatched")
	errMissingToken       = errors.New("missing token")
	errInvalidToken       = errors.New("invalid token format")
	errUnidentifiedSource = errors.New("unidentified source")
	errTooManyAttempts    = errors.New("too many connection attempts using an invalid token")
)

// Failed Connection Tracker Command Type
type fctCommandType int

// Failed Connection Tracker Commands
const (
	ValidateClient = 0
	ClearClient    = 1
)

type fctCommand struct {
	ty        fctCommandType
	src       string
	replyChan chan int
}

// Failed Connection Tracker Source Info
type fctClientInfo struct {
	attempts int64
	last     int64
}

// Server represents a simple-upload server.
type Server struct {
	// MaxUploadSize limits the size of the uploaded content, specified with "byte".
	DocumentRoot         string
	MaxUploadSize        int64
	SecureTokens         []string
	EnableCORS           bool
	MaxAttempts          int64
	LimitDiskFreeSpace   int
	LimitWaitingFiles    int
	FailedConTrackerChan chan<- fctCommand
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

// conTrackerManager starts a goroutine that serves as a manager for our
// connection tracking. Returns a channel that's used to send commands to the
// manager.
func ConTrackerManager(maxAttempts int64) chan<- fctCommand {
	failedConTracker := make(map[string]fctClientInfo)
	lastClean := time.Now().Unix()
	cmds := make(chan fctCommand)

	go func() {
		for cmd := range cmds {
			switch cmd.ty {
			case ValidateClient:
				currentTime := time.Now().Unix()
				sinceLastClean := currentTime - lastClean

				// // Clean failed connection tracker every 10 min
				if sinceLastClean >= 10*60 {
					logger.Info("Cleanning tracked connections")
					cutOffTime := currentTime - 30*60
					lastClean = currentTime

					// For each tracked client, if tracker is too old, clear its tracker
					for c, t := range failedConTracker {
						if t.last < cutOffTime {
							delete(failedConTracker, c)
						}
					}
				}

				tracker, trackerExists := failedConTracker[cmd.src]
				if trackerExists {
					sinceLastAttempt := currentTime - tracker.last
					tracker.attempts = tracker.attempts + 1

					if tracker.attempts > maxAttempts {
						if sinceLastAttempt < 5*60 {
							tracker.last = currentTime
							failedConTracker[cmd.src] = tracker
							logger.WithFields(logrus.Fields{
								"srcIP":    cmd.src,
								"attempts": tracker.attempts,
							}).Error("Too many connections attempts using an invalid token")
							time.Sleep(time.Second * 4)
							// Deny the Connection
							cmd.replyChan <- 0
						} else {
							// Allow the Connection
							tracker.last = currentTime
							failedConTracker[cmd.src] = tracker
							logger.WithFields(logrus.Fields{
								"srcIP":    cmd.src,
								"attempts": tracker.attempts,
							}).Debug("Allowing connection")
							cmd.replyChan <- 1
						}
					} else {
						// Allow the Connection
						tracker.last = currentTime
						failedConTracker[cmd.src] = tracker
						logger.WithFields(logrus.Fields{
							"srcIP":    cmd.src,
							"attempts": tracker.attempts,
						}).Debug("Allowing connection")
						cmd.replyChan <- 1
					}
				} else {
					// Allow first connection
					failedConTracker[cmd.src] = fctClientInfo{
						last:     currentTime,
						attempts: 1,
					}
					logger.WithFields(logrus.Fields{
						"srcIP":    cmd.src,
						"attempts": 1,
					}).Debug("Allowing connection")
					cmd.replyChan <- 1
				}
			case ClearClient:
				tracker, trackerExists := failedConTracker[cmd.src]

				if trackerExists {
					logger.WithFields(logrus.Fields{
						"srcIP":    cmd.src,
						"attempts": tracker.attempts,
					}).Debug("Clearing client tracker")
					tracker.attempts = 0
					failedConTracker[cmd.src] = tracker
				}

				cmd.replyChan <- 0
			default:
				cmd.replyChan <- -1
			}
		}
	}()
	return cmds
}

// NewServer creates a new simple-upload server.
func NewServer(documentRoot string, maxUploadSize int64,
	token_file string, enableCORS bool, MaxAttempts int64,
	LimitDiskFreeSpace int, LimitWaitingFiles int) Server {
	return Server{
		DocumentRoot:         documentRoot,
		MaxUploadSize:        maxUploadSize,
		SecureTokens:         LoadTokens(token_file),
		EnableCORS:           enableCORS,
		MaxAttempts:          MaxAttempts,
		LimitDiskFreeSpace:   LimitDiskFreeSpace,
		LimitWaitingFiles:    LimitWaitingFiles,
		FailedConTrackerChan: ConTrackerManager(MaxAttempts),
	}
}

func (s Server) handleGet(w http.ResponseWriter, r *http.Request) {
	// Validate the path
	if !rePathStatus.MatchString(r.URL.Path) {
		w.WriteHeader(http.StatusNotFound)
		writeError(w, fmt.Errorf("\"%s\" is not found", r.URL.Path))
		return
	}
	if s.EnableCORS {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.WriteHeader(http.StatusOK)
	writeOKStatus(w)
}

func (s Server) handlePost(w http.ResponseWriter, r *http.Request) {
	// Validate the path
	if !rePathUpload.MatchString(r.URL.Path) {
		w.WriteHeader(http.StatusNotFound)
		writeError(w, fmt.Errorf("\"%s\" is not found", r.URL.Path))
		return
	}

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
			logger.WithError(err).WithFields(logrus.Fields{"token": token[:min(len(token), tokenDumpLentgh)]}).Error("Failed to create upload directory for the given token")
			w.WriteHeader(http.StatusInternalServerError)
			writeError(w, err)
			return
		}
	}

	// Retrieve filesystem information
	diskFreeSpace, waitingFiles := getDiskUsage(uploadDir)

	// Check available disk space
	if diskFreeSpace < s.LimitDiskFreeSpace {
		logger.WithFields(logrus.Fields{
			"freeSpaceLimit": s.LimitDiskFreeSpace,
			"freeSpace":      diskFreeSpace,
		}).Error("Failed to upload, too many files")
		w.WriteHeader(http.StatusInsufficientStorage)
		writeError(w, fmt.Errorf("Failed to upload: disk space too low"))
		return
	}

	// Check amount of waiting files
	if waitingFiles > s.LimitWaitingFiles {
		logger.WithFields(logrus.Fields{
			"waitingFilesLimit": s.LimitWaitingFiles,
			"waitingFiles":      waitingFiles,
		}).Error("Failed to upload, too many files")
		w.WriteHeader(http.StatusInsufficientStorage)
		writeError(w, fmt.Errorf("Failed to upload: too many files"))
		return
	}

	// Retrieve the form file
	srcFile, info, err := r.FormFile("file")
	if err != nil {
		logger.WithError(err).WithFields(logrus.Fields{"token": token[:min(len(token), tokenDumpLentgh)]}).Error("Failed to acquire the uploaded content")
		w.WriteHeader(http.StatusInternalServerError)
		writeError(w, err)
		return
	}

	defer srcFile.Close()

	logger.Debug(info)
	size, err := getSize(srcFile)

	if err != nil {
		logger.WithError(err).WithFields(logrus.Fields{"token": token[:min(len(token), tokenDumpLentgh)]}).Error("Failed to get the size of the uploaded content")
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
		logger.WithError(err).WithFields(logrus.Fields{"token": token[:min(len(token), tokenDumpLentgh)]}).Error("Failed to read the uploaded content")
		w.WriteHeader(http.StatusInternalServerError)
		writeError(w, err)
		return
	}

	filename := info.Filename

	if filename == "" {
		filename = fmt.Sprintf("%x", sha1.Sum(body))
	}

	dstPath := path.Join(uploadDir, filepath.Base(filename))
	dstFile, err := os.OpenFile(dstPath+".tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)

	if err != nil {
		logger.WithError(err).WithField("path", dstPath).Error("Failed to open the file")
		w.WriteHeader(http.StatusInternalServerError)
		writeError(w, err)
		return
	}

	defer dstFile.Close()

	if written, err := dstFile.Write(body); err != nil {
		logger.WithError(err).WithField("path", dstPath).Error("Failed to write file content")
		w.WriteHeader(http.StatusInternalServerError)
		writeError(w, err)
		return
	} else if int64(written) != size {
		logger.WithFields(logrus.Fields{
			"size":    size,
			"written": written,
			"token":   token[:min(len(token), tokenDumpLentgh)],
		}).Error("Uploaded file size and written size differ")
		w.WriteHeader(http.StatusInternalServerError)
		writeError(w, fmt.Errorf("The size of uploaded content is %d, but %d bytes written", size, written))
	}

	err = os.Rename(dstPath+".tmp", dstPath)

	if err != nil {
		logger.Error("Unable to rename temporary upload file")
	}

	uploadedURL := strings.TrimPrefix(dstPath, s.DocumentRoot)

	if !strings.HasPrefix(uploadedURL, "/") {
		uploadedURL = "/" + uploadedURL
	}

	uploadedURL = "/files" + uploadedURL

	logger.WithFields(logrus.Fields{
		"path":  dstPath,
		"url":   uploadedURL,
		"size":  size,
		"token": token[:min(len(token), tokenDumpLentgh)],
	}).Info("File uploaded by POST")

	if s.EnableCORS {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.WriteHeader(http.StatusOK)
	writeSuccess(w, uploadedURL)
}

func (s Server) handleOptions(w http.ResponseWriter, r *http.Request) {
	var allowedMethods []string

	if rePathStatus.MatchString(r.URL.Path) {
		allowedMethods = []string{http.MethodGet}
	} else if rePathUpload.MatchString(r.URL.Path) {
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
	srcIP, srcIPError := getSrcIP(r)
	replyChan := make(chan int)

	if srcIPError == nil {
		s.FailedConTrackerChan <- fctCommand{
			ty:        ValidateClient,
			src:       srcIP,
			replyChan: replyChan,
		}

		switch reply := <-replyChan; reply {
		case 1:
			// Connection is granted, do nothing here
		case 0:
			return errTooManyAttempts
		default:
			return errServerError
		}
	} else {
		return errUnidentifiedSource
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
				s.FailedConTrackerChan <- fctCommand{
					ty:        ClearClient,
					src:       srcIP,
					replyChan: replyChan,
				}

				// Beware !!! Need to read the channel to prevent process lock
				if reply := <-replyChan; reply > 0 {
					logger.Error("Error clearing client tracker")
				}

				logger.Debug("Authentication succeed")
				return nil
			}
		}

		logger.WithFields(logrus.Fields{
			"token": token[:min(len(token), tokenDumpLentgh)],
		}).Error("Authentication attempt using unkown token")
		return errTokenMismatch
	}

	logger.WithFields(logrus.Fields{
		"token": token[:min(len(token), tokenDumpLentgh)],
	}).Error("Authentication attempt using invalid token format")
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
	case http.MethodGet:
		s.handleGet(w, r)
	case http.MethodPost:
		s.handlePost(w, r)
	default:
		w.Header().Add("Allow", "GET,POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		writeError(w, fmt.Errorf("HTTP Method \"%s\" is not allowed", r.Method))
	}
}
