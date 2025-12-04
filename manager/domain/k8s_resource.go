package domain

type DecisionMakerPod struct {
	NodeID string
	Port   int
	Host   string
	State  NodeState
}

type Pod struct {
	K8SNamespace string
	Labels       map[string]string
	PodID        string
	NodeID       string
	Containers   []Container
}

type Container struct {
	ContainerID string
	Name        string
	Command     []string
}
