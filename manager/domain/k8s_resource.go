package domain

type DecisionMakerPod struct {
	NodeID string
	Port   int
	Host   string
	State  NodeState
}

func (d *DecisionMakerPod) String() string {
	return "(" + d.NodeID + ")" + d.Host + ":" + string(rune(d.Port))
}

type Pod struct {
	Name         string
	K8SNamespace string
	Labels       map[string]string
	PodID        string
	NodeID       string
	Containers   []Container
}

func (p *Pod) LabelsToSelectors() []LabelSelector {
	selectors := make([]LabelSelector, 0, len(p.Labels))
	for k, v := range p.Labels {
		selectors = append(selectors, LabelSelector{
			Key:   k,
			Value: v,
		})
	}
	return selectors
}

type Container struct {
	ContainerID string
	Name        string
	Command     []string
}
