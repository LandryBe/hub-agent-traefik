package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "123"

func TestClient_Link(t *testing.T) {
	tests := []struct {
		desc             string
		returnClusterID  string
		returnStatusCode int
		wantErr          assert.ErrorAssertionFunc
	}{
		{
			desc:             "cluster successfully linked",
			returnClusterID:  "clusterID",
			returnStatusCode: http.StatusOK,
			wantErr:          assert.NoError,
		},
		{
			desc:             "failed to link cluster",
			returnStatusCode: http.StatusTeapot,
			wantErr:          assert.Error,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			var callCount int

			mux := http.NewServeMux()
			mux.HandleFunc("/link", func(rw http.ResponseWriter, req *http.Request) {
				callCount++

				if req.Method != http.MethodPost {
					http.Error(rw, fmt.Sprintf("unsupported to method: %s", req.Method), http.StatusMethodNotAllowed)
					return
				}

				if req.Header.Get("Authorization") != "Bearer "+testToken {
					http.Error(rw, "Invalid token", http.StatusUnauthorized)
					return
				}

				b, err := io.ReadAll(req.Body)
				if err != nil {
					http.Error(rw, err.Error(), http.StatusInternalServerError)
					return
				}

				if !bytes.Equal([]byte(`{"platform":"other"}`), b) {
					http.Error(rw, fmt.Sprintf("invalid body: %s", string(b)), http.StatusBadRequest)
					return
				}

				rw.WriteHeader(test.returnStatusCode)
				err = json.NewEncoder(rw).Encode(linkClusterResp{ClusterID: test.returnClusterID})
				require.NoError(t, err)
			})

			srv := httptest.NewServer(mux)

			t.Cleanup(srv.Close)

			c, err := NewClient(srv.URL, testToken)
			require.NoError(t, err)
			c.httpClient = srv.Client()

			clusterID, err := c.Link(context.Background())
			test.wantErr(t, err)

			if test.returnStatusCode == http.StatusOK {
				require.Equal(t, clusterID, test.returnClusterID)
			}
			require.Equal(t, 1, callCount)
		})
	}
}

func TestClient_GetConfig(t *testing.T) {
	tests := []struct {
		desc             string
		returnStatusCode int
		wantConfig       Config
		wantErr          assert.ErrorAssertionFunc
	}{
		{
			desc:             "get config succeeds",
			returnStatusCode: http.StatusOK,
			wantConfig: Config{
				Metrics: MetricsConfig{
					Interval: time.Minute,
					Tables:   []string{"1m", "10m"},
				},
			},
			wantErr: assert.NoError,
		},
		{
			desc:             "get config fails",
			returnStatusCode: http.StatusTeapot,
			wantConfig:       Config{},
			wantErr:          assert.Error,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			var callCount int

			mux := http.NewServeMux()
			mux.HandleFunc("/config", func(rw http.ResponseWriter, req *http.Request) {
				callCount++

				if req.Method != http.MethodGet {
					http.Error(rw, fmt.Sprintf("unsupported to method: %s", req.Method), http.StatusMethodNotAllowed)
					return
				}

				if req.Header.Get("Authorization") != "Bearer "+testToken {
					http.Error(rw, "Invalid token", http.StatusUnauthorized)
					return
				}

				rw.WriteHeader(test.returnStatusCode)
				_ = json.NewEncoder(rw).Encode(test.wantConfig)
			})

			srv := httptest.NewServer(mux)

			t.Cleanup(srv.Close)

			c, err := NewClient(srv.URL, testToken)
			require.NoError(t, err)
			c.httpClient = srv.Client()

			agentCfg, err := c.GetConfig(context.Background())
			test.wantErr(t, err)

			require.Equal(t, 1, callCount)

			assert.Equal(t, test.wantConfig, agentCfg)
		})
	}
}

func TestClient_Ping(t *testing.T) {
	tests := []struct {
		desc             string
		returnStatusCode int
		wantErr          assert.ErrorAssertionFunc
	}{
		{
			desc:             "ping successfully sent",
			returnStatusCode: http.StatusOK,
			wantErr:          assert.NoError,
		},
		{
			desc:             "ping sent for an unknown cluster",
			returnStatusCode: http.StatusNotFound,
			wantErr:          assert.Error,
		},
		{
			desc:             "error on ping",
			returnStatusCode: http.StatusInternalServerError,
			wantErr:          assert.Error,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			var callCount int

			mux := http.NewServeMux()
			mux.HandleFunc("/ping", func(rw http.ResponseWriter, req *http.Request) {
				callCount++

				if req.Method != http.MethodPost {
					http.Error(rw, fmt.Sprintf("unsupported to method: %s", req.Method), http.StatusMethodNotAllowed)
					return
				}

				if req.Header.Get("Authorization") != "Bearer "+testToken {
					http.Error(rw, "Invalid token", http.StatusUnauthorized)
					return
				}

				rw.WriteHeader(test.returnStatusCode)
			})

			srv := httptest.NewServer(mux)

			t.Cleanup(srv.Close)

			c, err := NewClient(srv.URL, testToken)
			require.NoError(t, err)
			c.httpClient = srv.Client()

			err = c.Ping(context.Background())
			test.wantErr(t, err)

			require.Equal(t, 1, callCount)
		})
	}
}
