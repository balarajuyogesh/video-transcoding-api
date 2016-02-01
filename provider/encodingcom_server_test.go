package provider

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/NYTimes/encoding-wrapper/encodingcom"
)

const encodingComDateFormat = "2006-01-02 15:04:05"

var errMediaNotFound = errors.New("media not found")

type request struct {
	Action  string               `json:"action"`
	MediaID string               `json:"mediaid"`
	Source  []string             `json:"source"`
	Format  []encodingcom.Format `json:"format"`
}

type errorResponse struct {
	Message string    `json:"message"`
	Errors  errorList `json:"errors"`
}

type errorList struct {
	Error []string `json:"error"`
}

type fakeMedia struct {
	ID       string
	Request  request
	Created  time.Time
	Started  time.Time
	Finished time.Time
	Status   string
}

// encodingComFakeServer is a fake version of the Encoding.com API.
type encodingComFakeServer struct {
	*httptest.Server
	medias map[string]*fakeMedia
}

func newEncodingComFakeServer() *encodingComFakeServer {
	server := encodingComFakeServer{medias: make(map[string]*fakeMedia)}
	server.Server = httptest.NewServer(&server)
	return &server
}

func (s *encodingComFakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestData := r.FormValue("json")
	if requestData == "" {
		s.Error(w, "json is required")
		return
	}
	var m map[string]request
	err := json.Unmarshal([]byte(requestData), &m)
	if err != nil {
		s.Error(w, err.Error())
		return
	}
	req := m["query"]
	switch req.Action {
	case "AddMedia":
		s.addMedia(w, req)
	case "GetStatus":
		s.getStatus(w, req)
	default:
		s.Error(w, "invalid action")
	}
}

func (s *encodingComFakeServer) addMedia(w http.ResponseWriter, req request) {
	id := generateID()
	created := time.Now().In(time.UTC)
	s.medias[id] = &fakeMedia{
		ID:      id,
		Request: req,
		Created: created,
		Started: created.Add(time.Second),
	}
	resp := map[string]encodingcom.AddMediaResponse{
		"response": {MediaID: id, Message: "it worked"},
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *encodingComFakeServer) getStatus(w http.ResponseWriter, req request) {
	media, err := s.getMedia(req.MediaID)
	if err != nil {
		s.Error(w, err.Error())
		return
	}
	now := time.Now().In(time.UTC)
	status := "Saving"
	if media.Status != "Finished" && now.Sub(media.Started) > time.Second {
		media.Finished = now
		status = "Finished"
		media.Status = status
	} else if media.Status != "" {
		status = media.Status
	}
	resp := map[string]map[string][]map[string]interface{}{
		"response": {
			"job": []map[string]interface{}{
				{
					"id":         media.ID,
					"sourcefile": "http://some.source.file",
					"userid":     "someuser",
					"status":     status,
					"progress":   "100.0",
					"time_left":  "1",
					"created":    media.Created.Format(encodingComDateFormat),
					"started":    media.Started.Format(encodingComDateFormat),
					"finished":   media.Finished.Format(encodingComDateFormat),
				},
			},
		},
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *encodingComFakeServer) Error(w http.ResponseWriter, message string) {
	m := map[string]errorResponse{"response": {
		Errors: errorList{Error: []string{message}},
	}}
	json.NewEncoder(w).Encode(m)
}

func (s *encodingComFakeServer) getMedia(id string) (*fakeMedia, error) {
	media, ok := s.medias[id]
	if !ok {
		return nil, errMediaNotFound
	}
	return media, nil
}

func generateID() string {
	var id [16]byte
	rand.Read(id[:])
	return fmt.Sprintf("%x", id[:])
}