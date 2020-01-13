package rchttptest

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

const (
	JsonContentType   = "application/json"
	FormEncodedType   = "application/x-www-form-urlencoded"
	ContentTypeHeader = "Content-Type"
)

type TestServer struct {
	Server    *httptest.Server
	URL       string
	CallCount int
	Mutex     sync.Mutex
}

type Responder struct {
	Body      string              `json:"body"`
	CallCount int                 `json:"call_count"`
	Method    string              `json:"method"`
	Error     string              `json:"error"`
	Headers   map[string][]string `json:"headers"`
	Path      string              `json:"path"`
}

type RequestTester struct {
	Delay         string `json:"delay"`
	DelayDuration time.Duration
	DelayOnCall   int `json:"delay_on_call"`
}

func NewTestServer(statusCode int) *TestServer {
	ts := &TestServer{
		CallCount: 0,
		Mutex:     sync.Mutex{},
	}

	hts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.GetCalls(1)

		w.WriteHeader(statusCode)
		contentType := r.Header.Get(ContentTypeHeader)
		b := GetBody(r.Body)
		headers := make(map[string][]string)
		for h, v := range r.Header {
			headers[h] = v
		}
		jsonResponse, err := json.Marshal(Responder{
			Method:    r.Method,
			CallCount: ts.GetCalls(0),
			Body:      string(b),
			Headers:   headers,
			Path:      r.URL.Path,
		})
		if err != nil {
			convertErrorToOutput(w, contentType, err)
			return
		}

		// when we see JSON, decode it to see if we need to sleep
		if contentType == JsonContentType {
			reqTest := &RequestTester{}
			if err := json.Unmarshal(b, reqTest); err != nil {
				convertErrorToOutput(w, contentType, err)
				return
			}
			if reqTest.Delay != "" {
				delayDuration, err := time.ParseDuration(reqTest.Delay)
				if err != nil {
					convertErrorToOutput(w, contentType, err)
					return
				}
				callCountNow := ts.GetCalls(0)
				if reqTest.DelayOnCall == callCountNow {
					time.Sleep(delayDuration)
				}
			}
		}

		fmt.Fprint(w, string(jsonResponse))
	}))

	ts.Server = hts
	ts.URL = hts.URL

	return ts
}

func (ts *TestServer) Close() {
	ts.Server.Close()
}

func (ts *TestServer) GetCalls(delta int) int {
	ts.Mutex.Lock()
	ts.CallCount += delta
	defer ts.Mutex.Unlock()
	return ts.CallCount
}

func convertErrorToOutput(w io.Writer, contentType string, err error) {
	if contentType != JsonContentType {
		fmt.Fprint(w, err)
	} else {
		errJson := `{"error":"` + strings.Replace(err.Error(), `"`, "`", -1) + `"}` // replaces " with `
		fmt.Fprint(w, errJson)
	}
}

func GetBody(body io.ReadCloser) []byte {
	b, err := ioutil.ReadAll(body)
	if err != nil {
		panic(err)
	}
	body.Close()
	return b
}
