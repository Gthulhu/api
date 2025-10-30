package service_test

import (
	"context"
	"testing"

	"github.com/Gthulhu/api/adapter/kubernetes"
	"github.com/Gthulhu/api/cache"
	"github.com/Gthulhu/api/domain"
	"github.com/Gthulhu/api/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestFindSchedulingStrategiesWithPID tests the FindSchedulingStrategiesWithPID function
func TestFindSchedulingStrategiesWithPID(t *testing.T) {
	fakeProc := setupFakeProcDir(t)

	mockK8SAdapter := kubernetes.NewMockK8sAdapter(t)
	svc := &service.Service{
		StrategyCache: cache.NewStrategyCache(),
		K8sAdapter:    mockK8SAdapter,
	}

	strategies := []*domain.SchedulingStrategy{{
		Priority:      true,
		ExecutionTime: 1,
		Selectors: []domain.LabelSelector{
			{
				Key:   "test",
				Value: "test",
			},
		},
	}}

	mockK8SAdapter.EXPECT().GetPodByPodUID(mock.Anything, "123abc-456def").Return(v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"test": "test",
			},
		},
	}, nil)
	res, fromCache, err := svc.FindSchedulingStrategiesWithPID(context.Background(), fakeProc, strategies)
	require.False(t, fromCache, "result should not be from cache")
	require.NoError(t, err, "FindSchedulingStrategiesWithPID should not return error")
	require.Len(t, res, 1, "should find one scheduling strategy")
	require.EqualValues(t, 1234, res[0].PID, "unexpected PID in scheduling strategy")

	res, fromCache, err = svc.FindSchedulingStrategiesWithPID(context.Background(), fakeProc, strategies)
	require.True(t, fromCache, "result should not be from cache")
	require.NoError(t, err, "FindSchedulingStrategiesWithPID should not return error")
	require.Len(t, res, 1, "should find one scheduling strategy")
	require.EqualValues(t, 1234, res[0].PID, "unexpected PID in scheduling strategy")
}
