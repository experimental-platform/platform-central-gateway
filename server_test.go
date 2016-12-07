package main

import (
	"io/ioutil"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/experimental-platform/platform-skvs/client"
	"github.com/experimental-platform/platform-skvs/server"
)

func TestGetSSLCert(t *testing.T) {
	testDataPath, err := ioutil.TempDir("", "")
	assert.Nil(t, err)

	srv := httptest.NewServer(server.NewServerHandler(testDataPath, nil, nil))
	c := client.NewFromURL(srv.URL)

	pemPath, keyPath, err := getSSLCert(c)
	assert.Nil(t, err)
	assert.Equal(t, "/data/ssl/pem", pemPath)
	assert.Equal(t, "/data/ssl/key", keyPath)

	err = c.Set("ssl/pem", "foo")
	assert.Nil(t, err)
	err = c.Set("ssl/key", "bar")
	assert.Nil(t, err)

	defer os.Remove("/tmp/gateway_pem")
	defer os.Remove("/tmp/gateway_key")
	pemPath, keyPath, err = getSSLCert(c)
	assert.Nil(t, err)
	assert.Equal(t, "/tmp/gateway_pem", pemPath)
	assert.Equal(t, "/tmp/gateway_key", keyPath)

	pemData, err := ioutil.ReadFile("/tmp/gateway_pem")
	assert.Nil(t, err)
	assert.Equal(t, "foo", string(pemData))
	keyData, err := ioutil.ReadFile("/tmp/gateway_key")
	assert.Nil(t, err)
	assert.Equal(t, "bar", string(keyData))
}
