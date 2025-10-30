package domain

// LabelSelector represents a key-value pair for pod label selection
type LabelSelector struct {
	Key   string `json:"key"`   // Label key
	Value string `json:"value"` // Label value
}

// SchedulingStrategy represents a strategy for process scheduling
type SchedulingStrategy struct {
	Priority      bool            `json:"priority"`                // If true, set vtime to minimum vtime
	ExecutionTime uint64          `json:"execution_time"`          // Time slice for this process in nanoseconds
	PID           int             `json:"pid,omitempty"`           // Process ID to apply this strategy to
	Selectors     []LabelSelector `json:"selectors,omitempty"`     // Label selectors to match pods
	CommandRegex  string          `json:"command_regex,omitempty"` // Regex to match process command
}
