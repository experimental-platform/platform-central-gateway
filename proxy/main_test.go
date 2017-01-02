package proxy

import (
	"testing"
	"net/http"
	"net/url"
)

type FakeTripper struct {
	request  *http.Request
	response *http.Response
	e        error
}

func (trip *FakeTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	trip.request = request
	return trip.response, trip.e
}

func TestGetRequest(t *testing.T) {
	var t = &FakeTripper{
		response: http.Response{},
		request: http.Request{},
	}
	var p = Proxy{
		backend:          &url.URL{},
		transport:        t,
	}
	p.ServeHTTP()

}

