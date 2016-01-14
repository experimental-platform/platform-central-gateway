package main

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedirectToAdminOnSlash(t *testing.T) {
	req, err := http.NewRequest("GET", "http://example.com", nil)
	assert.Nil(t, err)

	w := httptest.NewRecorder()
	defaultHandler(w, req)
	assert.Equal(t, w.Code, http.StatusMovedPermanently)
}

func TestNoRedirectToAdminOnSlash(t *testing.T) {
	req, err := http.NewRequest("GET", "http://example.com/", nil)
	assert.Nil(t, err)

	w := httptest.NewRecorder()
	defaultHandler(w, req)
	assert.Equal(t, http.StatusMovedPermanently, w.Code)
	assert.Equal(t, "/admin/", w.Header().Get("Location"))
}

func TestAppAccess(t *testing.T) {
	req, err := http.NewRequest("GET", "http://example.com/foobar", nil)
	assert.Nil(t, err)

	w := httptest.NewRecorder()
	defaultHandler(w, req)
	assert.NotEqual(t, http.StatusMovedPermanently, w.Code)
}

func TestAdminAccess(t *testing.T) {
	req, err := http.NewRequest("GET", "http://example.com/admin/", nil)
	assert.Nil(t, err)

	w := httptest.NewRecorder()
	defaultHandler(w, req)
	assert.NotEqual(t, http.StatusMovedPermanently, w.Code)
}

func TestAdminAccessRedirectSlash(t *testing.T) {
	req, err := http.NewRequest("GET", "http://example.com/admin", nil)
	assert.Nil(t, err)

	w := httptest.NewRecorder()
	defaultHandler(w, req)
	assert.Equal(t, http.StatusMovedPermanently, w.Code)
	assert.Equal(t, "/admin/", w.Header().Get("Location"))
}

func TestMain(m *testing.M) {
	target_url, _ := url.Parse("http://example.com/")
	apps_proxy = newSwitchingProxy(target_url)
	management_proxy = httputil.NewSingleHostReverseProxy(target_url)
	exit := m.Run()
	os.Exit(exit)
}
