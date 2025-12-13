package rest

import (
	"net/http"

	"github.com/Gthulhu/api/manager/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type LabelSelector struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type CreateScheduleStrategyRequest struct {
	StrategyNamespace string          `json:"strategyNamespace,omitempty"`
	LabelSelectors    []LabelSelector `json:"labelSelectors,omitempty"`
	K8sNamespace      []string        `json:"k8sNamespace,omitempty"`
	CommandRegex      string          `json:"commandRegex,omitempty"`
	Priority          int             `json:"priority,omitempty"`
	ExecutionTime     int64           `json:"executionTime,omitempty"`
}

func (h *Handler) CreateScheduleStrategy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req CreateScheduleStrategyRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	strategy := &domain.ScheduleStrategy{
		StrategyNamespace: req.StrategyNamespace,
		LabelSelectors:    make([]domain.LabelSelector, len(req.LabelSelectors)),
		K8sNamespace:      req.K8sNamespace,
		CommandRegex:      req.CommandRegex,
		Priority:          req.Priority,
		ExecutionTime:     req.ExecutionTime,
	}
	for i, ls := range req.LabelSelectors {
		strategy.LabelSelectors[i] = domain.LabelSelector{
			Key:   ls.Key,
			Value: ls.Value,
		}
	}

	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	err = h.Svc.CreateScheduleStrategy(ctx, &claims, strategy)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	response := NewSuccessResponse[string](nil)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type ListSchedulerStrategiesResponse struct {
	Strategies []*ScheduleStrategy `json:"strategies"`
}

type ScheduleStrategy struct {
	ID                bson.ObjectID   `bson:"_id,omitempty"`
	StrategyNamespace string          `bson:"strategyNamespace,omitempty"`
	LabelSelectors    []LabelSelector `bson:"labelSelectors,omitempty"`
	K8sNamespace      []string        `bson:"k8sNamespace,omitempty"`
	CommandRegex      string          `bson:"commandRegex,omitempty"`
	Priority          int             `bson:"priority,omitempty"`
	ExecutionTime     int64           `bson:"executionTime,omitempty"`
}

func (h *Handler) ListSelfScheduleStrategies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	uid, err := claims.GetBsonObjectUID()
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid user ID in token", err)
		return
	}
	queryOpt := &domain.QueryStrategyOptions{
		CreatorIDs: []bson.ObjectID{uid},
	}

	err = h.Svc.ListScheduleStrategies(ctx, queryOpt)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	resp := ListSchedulerStrategiesResponse{
		Strategies: make([]*ScheduleStrategy, len(queryOpt.Result)),
	}
	for i, ds := range queryOpt.Result {
		resp.Strategies[i] = h.convertDomainStrategyToResponseStrategy(ds)
	}
	response := NewSuccessResponse[ListSchedulerStrategiesResponse](&resp)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

func (h *Handler) convertDomainStrategyToResponseStrategy(domainStrategy *domain.ScheduleStrategy) *ScheduleStrategy {
	return &ScheduleStrategy{
		ID:                domainStrategy.ID,
		StrategyNamespace: domainStrategy.StrategyNamespace,
		LabelSelectors:    convertDomainLabelSelectorsToResponseLabelSelectors(domainStrategy.LabelSelectors),
		K8sNamespace:      domainStrategy.K8sNamespace,
		CommandRegex:      domainStrategy.CommandRegex,
		Priority:          domainStrategy.Priority,
		ExecutionTime:     domainStrategy.ExecutionTime,
	}
}

func convertDomainLabelSelectorsToResponseLabelSelectors(domainLabelSelectors []domain.LabelSelector) []LabelSelector {
	responseLabelSelectors := make([]LabelSelector, len(domainLabelSelectors))
	for i, dls := range domainLabelSelectors {
		responseLabelSelectors[i] = LabelSelector{
			Key:   dls.Key,
			Value: dls.Value,
		}
	}
	return responseLabelSelectors
}

type ListScheduleIntentsResponse struct {
	Intents []*ScheduleIntent `json:"intents"`
}

type ScheduleIntent struct {
	ID            bson.ObjectID      `bson:"_id,omitempty"`
	StrategyID    bson.ObjectID      `bson:"strategyID,omitempty"`
	PodID         string             `bson:"podID,omitempty"`
	NodeID        string             `bson:"nodeID,omitempty"`
	K8sNamespace  string             `bson:"k8sNamespace,omitempty"`
	CommandRegex  string             `bson:"commandRegex,omitempty"`
	Priority      int                `bson:"priority,omitempty"`
	ExecutionTime int64              `bson:"executionTime,omitempty"`
	PodLabels     map[string]string  `bson:"podLabels,omitempty"`
	State         domain.IntentState `bson:"state,omitempty"`
}

func (h *Handler) ListSelfScheduleIntents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	uid, err := claims.GetBsonObjectUID()
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid user ID in token", err)
		return
	}
	queryOpt := &domain.QueryIntentOptions{
		CreatorIDs: []bson.ObjectID{uid},
	}

	err = h.Svc.ListScheduleIntents(ctx, queryOpt)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	resp := ListScheduleIntentsResponse{
		Intents: make([]*ScheduleIntent, len(queryOpt.Result)),
	}
	for i, di := range queryOpt.Result {
		resp.Intents[i] = h.convertDomainIntentToResponseIntent(di)
	}
	response := NewSuccessResponse[ListScheduleIntentsResponse](&resp)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

func (h *Handler) convertDomainIntentToResponseIntent(domainIntent *domain.ScheduleIntent) *ScheduleIntent {
	return &ScheduleIntent{
		ID:            domainIntent.ID,
		StrategyID:    domainIntent.StrategyID,
		PodID:         domainIntent.PodID,
		NodeID:        domainIntent.NodeID,
		K8sNamespace:  domainIntent.K8sNamespace,
		CommandRegex:  domainIntent.CommandRegex,
		Priority:      domainIntent.Priority,
		ExecutionTime: domainIntent.ExecutionTime,
		PodLabels:     domainIntent.PodLabels,
		State:         domainIntent.State,
	}
}
