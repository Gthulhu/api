package service

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/Gthulhu/api/domain"
	"github.com/Gthulhu/api/util"
)

const (
	procDir = "/proc"
)

// GetAllPodInfos retrieves all pod information by scanning the /proc filesystem
func (svc *Service) GetAllPodInfos(ctx context.Context) ([]*domain.PodInfo, error) {
	return svc.FindPodInfoFrom(ctx, procDir)
}

// FindPodInfoFrom scans the given rootDir (e.g., /proc) to find pod information
func (svc *Service) FindPodInfoFrom(ctx context.Context, rootDir string) ([]*domain.PodInfo, error) {
	podMap := make(map[string]*domain.PodInfo)

	// Walk through /proc to find all processes
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if directory name is a PID (numeric)
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			// Not a numeric PID directory (e.g., "acpi", "bus", etc.) — skip
			continue
		}

		// Read cgroup information for this process
		cgroupPath := fmt.Sprintf("%s/%d/cgroup", rootDir, pid)
		file, err := os.Open(cgroupPath)
		if err != nil {
			util.GetLogger().Error("failed to open cgroup file", slog.String("path", cgroupPath), util.LogErrAttr(err))
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "kubepods") {
				err = svc.parseCgroupToPodInfo(rootDir, line, pid, podMap)
				if err != nil {
					// If we fail to parse for this process, log and continue with others
					util.GetLogger().Error("failed to parse cgroup to pod info", slog.String("line", line), util.LogErrAttr(err))
					break
				}
			}
		}
		if err := scanner.Err(); err != nil {
			util.GetLogger().Error("scanner error reading cgroup file", slog.String("path", cgroupPath), util.LogErrAttr(err))
		}
		_ = file.Close()
	}

	// Convert map to slice
	var pods []*domain.PodInfo
	for _, podInfo := range podMap {
		pods = append(pods, podInfo)
	}

	return pods, nil
}

// parseCgroupToPodInfo parses a cgroup line (e.g 9:cpuset:/kubepods/burstable/pod123abc-456def/docker-abcdef.scope) to extract pod info and updates the podInfoMap
func (svc *Service) parseCgroupToPodInfo(rootDir string, line string, pid int, podInfoMap map[string]*domain.PodInfo) error {
	parts := strings.Split(line, ":")
	if len(parts) >= 3 {
		cgroupHierarchy := parts[2]

		// Extract pod information
		podUID, containerID, err := svc.getPodInfoFromCgroup(cgroupHierarchy)
		if err != nil {
			return err
		}

		// Get process information
		process, err := svc.getProcessInfo(rootDir, pid)
		if err != nil {
			return err
		}

		// Create or update pod info
		if podInfo, exists := podInfoMap[podUID]; exists {
			podInfo.Processes = append(podInfo.Processes, process)
			if containerID != "" && podInfo.ContainerID == "" {
				podInfo.ContainerID = containerID
			}
		} else {
			podInfoMap[podUID] = &domain.PodInfo{
				PodUID:      podUID,
				ContainerID: containerID,
				Processes:   []domain.PodProcess{process},
			}
		}
	}
	return nil
}

// getPodInfoFromCgroup extracts pod information from cgroup path
func (svc *Service) getPodInfoFromCgroup(cgroupPath string) (podUID string, containerID string, err error) {
	// Parse cgroup path to extract pod information
	// Format: /kubepods/burstable/pod<pod-uid>/<container-id>
	// or: /kubepods/pod<pod-uid>/<container-id>
	parts := strings.Split(cgroupPath, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "pod") {
			podUID = strings.TrimPrefix(part, "pod")
			podUID = strings.ReplaceAll(podUID, "_", "-")
			if i+1 < len(parts) {
				containerID = parts[i+1]
			}
			break
		}
	}

	if podUID == "" {
		return "", "", fmt.Errorf("pod UID not found in cgroup path")
	}

	return podUID, containerID, nil
}

// getProcessInfo reads process information from /proc/<pid>/
func (svc *Service) getProcessInfo(rootDir string, pid int) (domain.PodProcess, error) {
	process := domain.PodProcess{PID: pid}

	// Read command from /proc/<pid>/comm
	commPath := fmt.Sprintf("/%s/%d/comm", rootDir, pid)
	if data, err := os.ReadFile(commPath); err == nil {
		process.Command = strings.TrimSpace(string(data))
	}

	// Read PPID from /proc/<pid>/stat
	statPath := fmt.Sprintf("/%s/%d/stat", rootDir, pid)
	if data, err := os.ReadFile(statPath); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 4 {
			if ppid, err := strconv.Atoi(fields[3]); err == nil {
				process.PPID = ppid
			}
		}
	}

	return process, nil
}
