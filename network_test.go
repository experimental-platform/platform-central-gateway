package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	"github.com/docker/engine-api/types/network"
)

var testContainers = []types.Container{
	{ID: "ff08c45d1a40", Names: []string{"/someapp"}},
	{ID: "8fb6d8595f23", Names: []string{"/foobarapp"}},
	{ID: "1ce60012a101", Names: []string{"/anotherapp"}},
}

var testContainersDetails = map[string]types.ContainerJSON{
	"8fb6d8595f23": types.ContainerJSON{
		NetworkSettings: &types.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				"protonet": &network.EndpointSettings{
					IPAddress: "512.412.912.331",
				},
			},
		},
	},
}

func createTestMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/v1.22/containers/json", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		f, err := filters.FromParam(req.FormValue("filters"))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
		}

		var containersToSend []types.Container

		for _, testContainer := range testContainers {
			for _, n := range testContainer.Names {
				if f.Match("name", n) {
					containersToSend = append(containersToSend, testContainer)
					break
				}
			}
		}

		encoder := json.NewEncoder(rw)
		encoder.Encode(containersToSend)
	}))

	mux.Handle("/v1.22/containers/8fb6d8595f23/json", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		encoder := json.NewEncoder(rw)
		encoder.Encode(testContainersDetails["8fb6d8595f23"])
	}))

	mux.Handle("/", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		fmt.Printf("REQUEST: %+v\n", req)
	}))

	return mux
}

func TestMain(m *testing.M) {
	os.Setenv("DOCKER_API_VERSION", "1.22")
	os.Exit(m.Run())
}

func TestGetAppIP(t *testing.T) {
	mux := createTestMux()
	testserver := httptest.NewServer(mux)
	defer testserver.Close()
	os.Setenv("DOCKER_HOST", testserver.URL)

	ip, err := getAppIP("foobarapp")
	if err != nil {
		t.Fatal(err)
	}

	if ip != testContainersDetails["8fb6d8595f23"].NetworkSettings.Networks["protonet"].IPAddress {
		t.Fatalf("Expected ip %s, got %s", testContainersDetails["8fb6d8595f23"].NetworkSettings.Networks["protonet"].IPAddress, ip)
	}
}
