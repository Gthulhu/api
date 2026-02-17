package client

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/Gthulhu/api/manager/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetIntentMerkleRootSuccess(t *testing.T) {
	const cachedToken = "cached-token"
	const rootHash = "root-hash-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/intents/merkle", r.URL.Path)
		assert.Equal(t, "Bearer "+cachedToken, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"data":{"rootHash":"` + rootHash + `"},"timestamp":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	dm := newDecisionMakerPodFromServerURL(t, server.URL)
	client := newDecisionMakerClientWithCachedToken(dm.NodeID, cachedToken, server.Client())

	got, err := client.GetIntentMerkleRoot(context.Background(), dm)
	require.NoError(t, err)
	assert.Equal(t, rootHash, got)
}

func TestGetIntentMerkleRootNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	dm := newDecisionMakerPodFromServerURL(t, server.URL)
	client := newDecisionMakerClientWithCachedToken(dm.NodeID, "cached-token", server.Client())

	_, err := client.GetIntentMerkleRoot(context.Background(), dm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "returned non-OK status")
}

func TestGetIntentMerkleRootEmptyData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"data":null,"timestamp":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	dm := newDecisionMakerPodFromServerURL(t, server.URL)
	client := newDecisionMakerClientWithCachedToken(dm.NodeID, "cached-token", server.Client())

	_, err := client.GetIntentMerkleRoot(context.Background(), dm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "returned empty merkle root")
}

func newDecisionMakerClientWithCachedToken(nodeID, token string, httpClient *http.Client) *DecisionMakerClient {
	tokenCache := cache.New[string, string]()
	tokenCache.Set(nodeID, token)
	return &DecisionMakerClient{
		Client:     httpClient,
		tokenCache: tokenCache,
	}
}

func newDecisionMakerPodFromServerURL(t *testing.T, rawURL string) *domain.DecisionMakerPod {
	t.Helper()
	parsedURL, err := url.Parse(rawURL)
	require.NoError(t, err)
	host, portStr, err := net.SplitHostPort(parsedURL.Host)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)
	return &domain.DecisionMakerPod{
		NodeID: "node-1",
		Host:   host,
		Port:   port,
		State:  domain.NodeStateOnline,
	}
}
