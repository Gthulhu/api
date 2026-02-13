package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/Gthulhu/api/config"
	dmrest "github.com/Gthulhu/api/decisionmaker/rest"
	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/pkg/logger"
)

func NewDecisionMakerClient(keyConfig config.KeyConfig) domain.DecisionMakerAdapter {
	return &DecisionMakerClient{
		Client:         http.DefaultClient,
		tokenPublicKey: keyConfig.DMPublicKeyPem.Value(),
		clientID:       keyConfig.ClientID,
		tokenCache:     cache.New[string, string](),
	}
}

type DecisionMakerClient struct {
	*http.Client

	tokenPublicKey string
	clientID       string
	tokenCache     *cache.Cache[string, string]
}

func (dm *DecisionMakerClient) SendSchedulingIntent(ctx context.Context, decisionMaker *domain.DecisionMakerPod, intents []*domain.ScheduleIntent) error {
	token, err := dm.GetToken(ctx, decisionMaker)
	if err != nil {
		return err
	}

	logger.Logger(ctx).Debug().Msgf("Sending %d scheduling intents to decision maker pod (host:%s nodeID:%s port:%d)", len(intents), decisionMaker.Host, decisionMaker.NodeID, decisionMaker.Port)

	reqPayload := dmrest.HandleIntentsRequest{
		Intents: make([]dmrest.Intent, 0, len(intents)),
	}
	for _, intent := range intents {
		reqPayload.Intents = append(reqPayload.Intents, dmrest.Intent{
			PodName:       intent.PodName,
			PodID:         intent.PodID,
			NodeID:        intent.NodeID,
			K8sNamespace:  intent.K8sNamespace,
			CommandRegex:  intent.CommandRegex,
			Priority:      intent.Priority,
			ExecutionTime: intent.ExecutionTime,
			PodLabels:     intent.PodLabels,
		})
	}

	jsonBody, err := json.Marshal(reqPayload)
	if err != nil {
		return err
	}
	endpoint := "http://" + decisionMaker.Host + ":" + strconv.Itoa(decisionMaker.Port) + "/api/v1/intents"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := dm.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("decision maker %s returned non-OK status: %s", decisionMaker, resp.Status)
	}
	return nil
}

func (dm *DecisionMakerClient) GetIntentMerkleRoot(ctx context.Context, decisionMaker *domain.DecisionMakerPod) (string, error) {
	token, err := dm.GetToken(ctx, decisionMaker)
	if err != nil {
		return "", err
	}

	endpoint := "http://" + decisionMaker.Host + ":" + strconv.Itoa(decisionMaker.Port) + "/api/v1/intents/merkle"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := dm.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("decision maker %s returned non-OK status: %s", decisionMaker, resp.Status)
	}

	var merkleResp dmrest.SuccessResponse[dmrest.MerkleRootResponse]
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&merkleResp); err != nil {
		return "", err
	}
	if merkleResp.Data == nil {
		return "", fmt.Errorf("decision maker %s returned empty merkle root", decisionMaker)
	}
	return merkleResp.Data.RootHash, nil
}

func (dm *DecisionMakerClient) GetToken(ctx context.Context, decisionMaker *domain.DecisionMakerPod) (string, error) {
	if token, ok := dm.tokenCache.Get(decisionMaker.NodeID); ok {
		return token, nil
	}

	req := dmrest.TokenRequest{
		PublicKey: dm.tokenPublicKey,
		ClientID:  dm.clientID,
	}
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	endpoint := "http://" + decisionMaker.Host + ":" + strconv.Itoa(decisionMaker.Port) + "/api/v1/auth/token"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := dm.Client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("decision maker %s returned non-OK status: %s", decisionMaker, resp.Status)
	}
	var tokenResp dmrest.SuccessResponse[dmrest.TokenResponse]
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&tokenResp)
	if err != nil {
		return "", err
	}

	ttl := time.Now().Unix() - tokenResp.Data.ExpiredAt - 60

	dm.tokenCache.Set(decisionMaker.NodeID, tokenResp.Data.Token, cache.WithExpiration(time.Duration(ttl)*time.Second))
	return tokenResp.Data.Token, nil

}

func (dm *DecisionMakerClient) DeleteSchedulingIntents(ctx context.Context, decisionMaker *domain.DecisionMakerPod, req *domain.DeleteIntentsRequest) error {
	token, err := dm.GetToken(ctx, decisionMaker)
	if err != nil {
		return err
	}

	logger.Logger(ctx).Debug().Msgf("Deleting scheduling intents from decision maker pod (host:%s nodeID:%s port:%d)", decisionMaker.Host, decisionMaker.NodeID, decisionMaker.Port)

	// If All is true, delete all intents; otherwise delete by PodIDs one by one
	if req.All {
		deleteReq := dmrest.DeleteIntentRequest{
			All: true,
		}
		jsonBody, err := json.Marshal(deleteReq)
		if err != nil {
			return err
		}
		endpoint := "http://" + decisionMaker.Host + ":" + strconv.Itoa(decisionMaker.Port) + "/api/v1/intents"
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, bytes.NewBuffer(jsonBody))
		if err != nil {
			return err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+token)
		resp, err := dm.Client.Do(httpReq)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("decision maker %s returned non-OK status: %s", decisionMaker, resp.Status)
		}
		return nil
	}

	// Delete intents by PodID
	for _, podID := range req.PodIDs {
		deleteReq := dmrest.DeleteIntentRequest{
			PodID: podID,
		}
		jsonBody, err := json.Marshal(deleteReq)
		if err != nil {
			return err
		}
		endpoint := "http://" + decisionMaker.Host + ":" + strconv.Itoa(decisionMaker.Port) + "/api/v1/intents"
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, bytes.NewBuffer(jsonBody))
		if err != nil {
			return err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+token)
		resp, err := dm.Client.Do(httpReq)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("decision maker %s returned non-OK status for podID %s: %s", decisionMaker, podID, resp.Status)
		}
	}

	return nil
}
