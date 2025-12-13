package service

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/Gthulhu/api/pkg/logger"
)

func NewService() Service {
	return Service{}
}

type Service struct {
}

const (
	procDir = "/proc"
)

func (svc *Service) ProcessIntents(ctx context.Context, intents []*domain.Intent) error {
	// Placeholder for processing intents
	podInfos, err := svc.GetAllPodInfos(ctx)
	if err != nil {
		return err
	}
	for _, intent := range intents {
		podInfo := podInfos[intent.PodID]
		logger.Logger(ctx).Info().Msgf("Processing intent for PodName:%s PodID: %s on NodeID: %s, Process:%+v", intent.PodName, intent.PodID, intent.NodeID, podInfo)
		// Add logic to handle the intent

	}
	logger.Logger(ctx).Info().Msgf("Discovered pods: %+v", podInfos)
	return nil
}

// GetAllPodInfos retrieves all pod information by scanning the /proc filesystem
func (svc *Service) GetAllPodInfos(ctx context.Context) (map[string]*domain.PodInfo, error) {
	return svc.FindPodInfoFrom(ctx, procDir)
}

// FindPodInfoFrom scans the given rootDir (e.g., /proc) to find pod information
func (svc *Service) FindPodInfoFrom(ctx context.Context, rootDir string) (map[string]*domain.PodInfo, error) {
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
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to open cgroup file for pid %d", pid)
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			logger.Logger(ctx).Debug().Msgf("cgroup line for pid %d: %s", pid, line)
			if strings.Contains(line, "kubepods") {
				err = svc.parseCgroupToPodInfo(rootDir, line, pid, podMap)
				if err != nil {
					logger.Logger(ctx).Warn().Err(err).Msgf("failed to parse cgroup line for pid %d, line:%s", pid, line)
					break
				}
			}
		}
		if err := scanner.Err(); err != nil {
		}
		_ = file.Close()
	}

	return podMap, nil
}

// parseCgroupToPodInfo parses a cgroup line (e.g // 0::/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-pod20da609e_6973_4463_a1f9_2db9bcc5becc.slice/cri-containerd-10ec3c89629f71226b227e6510b2d465168b24005bbdcc5d7940517080830635.scope) to extract pod info and updates the podInfoMap
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
		process.ContainerID = containerID

		// Create or update pod info
		if podInfo, exists := podInfoMap[podUID]; exists {
			podInfo.Processes = append(podInfo.Processes, process)
		} else {
			podInfoMap[podUID] = &domain.PodInfo{
				PodUID:    podUID,
				Processes: []domain.PodProcess{process},
			}
		}
	}
	return nil
}

var (
	podRegex = regexp.MustCompile(`pod([0-9a-fA-F_]+)(?:\.slice)?`)
)

// getPodInfoFromCgroup extracts pod information from cgroup path
func (svc *Service) getPodInfoFromCgroup(cgroupPath string) (podUID string, containerID string, err error) {
	// Parse cgroup path to extract pod information
	// 0::/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-pod20da609e_6973_4463_a1f9_2db9bcc5becc.slice/cri-containerd-10ec3c89629f71226b227e6510b2d465168b24005bbdcc5d7940517080830635.scope
	parts := strings.Split(cgroupPath, "/")
	for _, part := range parts {
		if podRegex.MatchString(part) {
			podUID = podRegex.FindStringSubmatch(part)[1]
			podUID = strings.ReplaceAll(podUID, "_", "-")
		}
		if strings.HasPrefix(part, "cri-containerd-") && strings.HasSuffix(part, ".scope") {
			containerID = strings.TrimPrefix(part, "cri-containerd-")
			containerID = strings.TrimSuffix(containerID, ".scope")
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
