package metrics_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hamba/avro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traefik/neo-agent/pkg/metrics"
	"github.com/traefik/neo-agent/pkg/metrics/protocol"
)

func TestClient_GetPreviousData(t *testing.T) {
	schema, err := avro.Parse(protocol.MetricsV2Schema)
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/data", r.URL.Path)
		assert.Equal(t, "Bearer some_test_token", r.Header.Get("Authorization"))
		assert.Equal(t, "avro/binary;v2", r.Header.Get("Accept"))

		data := map[string][]metrics.DataPointGroup{
			"1m": {
				{
					Ingress: "bar",
					Service: "baz",
					DataPoints: []metrics.DataPoint{
						{
							Timestamp: 21,
						},
					},
				},
			},
		}
		err = avro.NewEncoderForSchema(schema, w).Encode(data)
		require.NoError(t, err)
	}))
	t.Cleanup(func() {
		srv.Close()
	})

	client, err := metrics.NewClient(http.DefaultClient, srv.URL, "some_test_token")
	require.NoError(t, err)

	got, err := client.GetPreviousData(context.Background(), true)
	require.NoError(t, err)

	want := map[string][]metrics.DataPointGroup{
		"1m": {
			{
				Ingress: "bar",
				Service: "baz",
				DataPoints: []metrics.DataPoint{
					{
						Timestamp: 21,
					},
				},
			},
		},
	}
	assert.Equal(t, want, got)
}

func TestClient_GetPreviousDataHandlesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "test error", http.StatusInternalServerError)
	}))
	t.Cleanup(func() {
		srv.Close()
	})

	client, err := metrics.NewClient(http.DefaultClient, srv.URL, "some_test_token")
	require.NoError(t, err)

	_, err = client.GetPreviousData(context.Background(), true)

	assert.Error(t, err)
}

func TestClient_Send(t *testing.T) {
	schema, err := avro.Parse(protocol.MetricsV2Schema)
	require.NoError(t, err)

	data := map[string][]metrics.DataPointGroup{
		"1m": {
			{
				Ingress: "bar",
				Service: "baz",
				DataPoints: []metrics.DataPoint{
					{
						Timestamp: 21,
					},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/metrics", r.URL.Path)
		assert.Equal(t, "Bearer some_test_token", r.Header.Get("Authorization"))
		assert.Equal(t, "avro/binary;v2", r.Header.Get("Content-Type"))

		got := map[string][]metrics.DataPointGroup{}
		err = avro.NewDecoderForSchema(schema, r.Body).Decode(&got)

		if assert.NoError(t, err) {
			assert.Equal(t, data, got)
		}
	}))
	t.Cleanup(func() {
		srv.Close()
	})

	client, err := metrics.NewClient(http.DefaultClient, srv.URL, "some_test_token")
	require.NoError(t, err)

	err = client.Send(context.Background(), data)

	assert.NoError(t, err)
}

func TestClient_SendHandlesHTTPError(t *testing.T) {
	data := map[string][]metrics.DataPointGroup{
		"1m": {
			{
				Ingress: "bar",
				Service: "baz",
				DataPoints: []metrics.DataPoint{
					{
						Timestamp: 21,
					},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "test error", http.StatusInternalServerError)
	}))
	t.Cleanup(func() {
		srv.Close()
	})

	client, err := metrics.NewClient(http.DefaultClient, srv.URL, "some_test_token")
	require.NoError(t, err)

	err = client.Send(context.Background(), data)

	assert.Error(t, err)
}
