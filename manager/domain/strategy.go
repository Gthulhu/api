package domain

import "go.mongodb.org/mongo-driver/v2/bson"

type ScheduleStrategy struct {
	BaseEntity
	StrategyNamespace string          `bson:"strategyNamespace,omitempty"`
	LabelSelectors    []LabelSelector `bson:"labelSelectors,omitempty"`
	K8sNamespaces     []string        `bson:"k8sNamespaces,omitempty"`
	CommandRegex      string          `bson:"commandRegex,omitempty"`
	Priority          int             `bson:"priority,omitempty"`
	ExecutionTime     int64           `bson:"executionTime,omitempty"`
}

type ScheduleIntent struct {
	ID             bson.ObjectID   `bson:"_id,omitempty"`
	StrategyID     bson.ObjectID   `bson:"strategyID,omitempty"`
	PodID          string          `bson:"podID,omitempty"`
	NodeID         string          `bson:"nodeID,omitempty"`
	K8sNamespace   string          `bson:"k8sNamespace,omitempty"`
	CommandRegex   string          `bson:"commandRegex,omitempty"`
	Priority       int             `bson:"priority,omitempty"`
	ExecutionTime  int64           `bson:"executionTime,omitempty"`
	LabelSelectors []LabelSelector `bson:"labelSelectors,omitempty"`
	State          IntentState     `bson:"state,omitempty"`
	CreatedTime    int64           `bson:"createdTime,omitempty"`
	SentTime       int64           `bson:"sentTime,omitempty"`
}

type LabelSelector struct {
	Key   string `bson:"key,omitempty"`
	Value string `bson:"value,omitempty"`
}
