package main

import (
	"bytes"
	"container/ring"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const RingSize = 10

type Event struct {
	Timestamp time.Time
	Message   string
}

type DebugHandler struct {
	Prefix string
	Ring   *ring.Ring
	Lock   sync.Mutex
}

func NewDebugHandler(Prefix string) *DebugHandler {
	return &DebugHandler{
		Prefix: Prefix,
		Ring:   ring.New(RingSize),
	}
}

func writeTextResponse(response http.ResponseWriter, status int, message string) {
	response.Header()["Content-Type"] = []string{"text/plain"}
	response.Header()["X-Version"] = []string{Version}
	response.WriteHeader(status)
	io.WriteString(response, message)
}

func (handler *DebugHandler) HandleWebhook(response http.ResponseWriter, request *http.Request) {
	handler.Lock.Lock()
	defer handler.Lock.Unlock()

	buffer := new(bytes.Buffer)
	buffer.ReadFrom(request.Body)

	handler.Ring = handler.Ring.Prev()
	handler.Ring.Value = &Event{
		Timestamp: time.Now(),
		Message:   buffer.String(),
	}

	writeTextResponse(response, 200, "Acknowledged")
}

func (handler *DebugHandler) HandleDisplay(response http.ResponseWriter, request *http.Request) {
	response.Header()["Content-Type"] = []string{"text/plain"}
	response.Header()["X-Version"] = []string{Version}
	response.WriteHeader(200)

	handler.Ring.Do(func(value interface{}) {
		if value == nil {
			return
		}
		event := value.(*Event)
		fmt.Fprintf(response, "== %s ==\n%s\n\n", event.Timestamp.Format(time.UnixDate), event.Message)
	})
}

func (handler *DebugHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/ping/" {
		writeTextResponse(response, 200, "Pong!")
		return
	}

	if !strings.HasPrefix(request.URL.Path, handler.Prefix) {
		writeTextResponse(response, 404, "Not found")
		return
	}

	localPath := request.URL.Path[len(handler.Prefix):]

	if localPath == "webhook/" {
		if request.Method == "POST" {
			handler.HandleWebhook(response, request)
		} else {
			writeTextResponse(response, 405, "Unsupported method")
		}
		return
	}

	if localPath == "" {
		handler.HandleDisplay(response, request)
		return
	}

	writeTextResponse(response, 404, "Not found")
}

func main() {
	prefix := os.Getenv("URL_PREFIX")
	if prefix == "" {
		panic("URL_PREFIX env variable is required")
	}

	if prefix[0] != '/' {
		prefix = "/" + prefix
	}

	if prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}

	if err := http.ListenAndServe("0.0.0.0:8000", NewDebugHandler(prefix)); err != nil {
		panic(err.Error())
	}
}
