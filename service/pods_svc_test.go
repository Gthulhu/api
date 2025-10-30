package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Gthulhu/api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupFakeProcDir creates a temporary fake /proc directory structure for testing
func setupFakeProcDir(t *testing.T) string {
	root := t.TempDir()
	// fake pid 1234
	pidDir := filepath.Join(root, "1234")
	if err := os.Mkdir(pidDir, 0755); err != nil {
		t.Fatal(err)
	}

	// cgroup
	cgroupContent := "9:cpuset:/kubepods/burstable/pod123abc-456def/docker-abcdef.scope\n"
	if err := os.WriteFile(filepath.Join(pidDir, "cgroup"), []byte(cgroupContent), 0644); err != nil {
		t.Fatal(err)
	}

	// comm
	if err := os.WriteFile(filepath.Join(pidDir, "comm"), []byte("nginx\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// stat
	statLine := "1234 (nginx) S 1 2 3 4 5 0 0 0 0 0 0 0 0 0 0 0 0 0 0"
	if err := os.WriteFile(filepath.Join(pidDir, "stat"), []byte(statLine), 0644); err != nil {
		t.Fatal(err)
	}

	return root
}

// TestFindPodInfoFrom tests the FindPodInfoFrom function
func TestFindPodInfoFrom(t *testing.T) {
	fakeProc := setupFakeProcDir(t)
	svc := &service.Service{}

	pods, err := svc.FindPodInfoFrom(context.Background(), fakeProc)
	assert.NoError(t, err, "FindPodInfoFrom should not return error")
	require.Len(t, pods, 1, "should find one pod info")

	p := pods[0]
	assert.EqualValues(t, p.PodUID, "123abc-456def", "unexpected podUID")
	require.Len(t, p.Processes, 1, "should have one process")
	assert.EqualValues(t, p.Processes[0].Command, "nginx", "unexpected command")
}
