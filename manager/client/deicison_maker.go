package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	dmrest "github.com/Gthulhu/api/decisionmaker/rest"
	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/pkg/logger"
)

func NewDecisionMakerClient() domain.DecisionMakerAdapter {
	return &DecisionMakerClient{
		Client: http.DefaultClient,
	}
}

type DecisionMakerClient struct {
	*http.Client
}

func (dm DecisionMakerClient) SendSchedulingIntent(ctx context.Context, decisionMaker *domain.DecisionMakerPod, intents []*domain.ScheduleIntent) error {
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
	// TODO: add authentication headers if needed

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
