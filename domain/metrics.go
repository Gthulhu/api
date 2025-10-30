package domain

import "time"

// BssData represents the metrics data structure
type BssData struct {
	Usersched_last_run_at uint64    `json:"usersched_last_run_at"` // The PID of the userspace scheduler
	Nr_queued             uint64    `json:"nr_queued"`             // Number of tasks queued in the userspace scheduler
	Nr_scheduled          uint64    `json:"nr_scheduled"`          // Number of tasks scheduled by the userspace scheduler
	Nr_running            uint64    `json:"nr_running"`            // Number of tasks currently running in the userspace scheduler
	Nr_online_cpus        uint64    `json:"nr_online_cpus"`        // Number of online CPUs in the system
	Nr_user_dispatches    uint64    `json:"nr_user_dispatches"`    // Number of user-space dispatches
	Nr_kernel_dispatches  uint64    `json:"nr_kernel_dispatches"`  // Number of kernel-space dispatches
	Nr_cancel_dispatches  uint64    `json:"nr_cancel_dispatches"`  // Number of cancelled dispatches
	Nr_bounce_dispatches  uint64    `json:"nr_bounce_dispatches"`  // Number of bounce dispatches
	Nr_failed_dispatches  uint64    `json:"nr_failed_dispatches"`  // Number of failed dispatches
	Nr_sched_congested    uint64    `json:"nr_sched_congested"`    // Number of times the scheduler was congested
	UpdatedTime           time.Time `json:"-"`                     // Timestamp of the last update
}
