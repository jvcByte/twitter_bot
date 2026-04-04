package content

import (
	"net/http"
	"time"
)

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
	}
}
