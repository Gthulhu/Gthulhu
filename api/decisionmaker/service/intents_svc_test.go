package service

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/Gthulhu/api/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTraverseIntentMerkleTreeNilRequest(t *testing.T) {
	svc := &Service{}

	resp, err := svc.TraverseIntentMerkleTree(context.Background(), nil)
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestTraverseIntentMerkleTreeDepthZero(t *testing.T) {
	root := util.BuildMerkleTree([]string{
		util.HashStringSHA256Hex("leaf-a"),
		util.HashStringSHA256Hex("leaf-b"),
	})
	svc := &Service{intentMerkleRoot: root}

	resp, err := svc.TraverseIntentMerkleTree(context.Background(), &TraverseIntentMerkleTreeOptions{
		Depth: 0,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.RootNode)
	assert.Equal(t, root.Hash, resp.RootNode.Hash)
	assert.Nil(t, resp.RootNode.Left)
	assert.Nil(t, resp.RootNode.Right)
}

func TestTraverseIntentMerkleTreeFindSubTreeByRootHash(t *testing.T) {
	root := util.BuildMerkleTree([]string{
		util.HashStringSHA256Hex("leaf-a"),
		util.HashStringSHA256Hex("leaf-b"),
		util.HashStringSHA256Hex("leaf-c"),
		util.HashStringSHA256Hex("leaf-d"),
	})
	require.NotNil(t, root)
	require.NotNil(t, root.Left)
	require.NotNil(t, root.Right)

	svc := &Service{intentMerkleRoot: root}
	resp, err := svc.TraverseIntentMerkleTree(context.Background(), &TraverseIntentMerkleTreeOptions{
		RootHash: root.Left.Hash,
		Depth:    1,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.RootNode)
	assert.Equal(t, root.Left.Hash, resp.RootNode.Hash)
	require.NotNil(t, resp.RootNode.Left)
	require.NotNil(t, resp.RootNode.Right)
	assert.Nil(t, resp.RootNode.Left.Left)
	assert.Nil(t, resp.RootNode.Left.Right)
	assert.Nil(t, resp.RootNode.Right.Left)
	assert.Nil(t, resp.RootNode.Right.Right)
}

func TestTraverseIntentMerkleTreeRootHashNotFound(t *testing.T) {
	svc := &Service{
		intentMerkleRoot: util.BuildMerkleTree([]string{
			util.HashStringSHA256Hex("leaf-a"),
			util.HashStringSHA256Hex("leaf-b"),
		}),
	}

	resp, err := svc.TraverseIntentMerkleTree(context.Background(), &TraverseIntentMerkleTreeOptions{
		RootHash: "missing-hash",
		Depth:    0,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Nil(t, resp.RootNode)
}

func TestTraverseIntentMerkleTreeRefreshesRootFromIntentCache(t *testing.T) {
	intentA := &domain.Intent{
		PodName:       "pod-a",
		PodID:         "pod-id-a",
		NodeID:        "node-a",
		K8sNamespace:  "default",
		CommandRegex:  "nginx",
		Priority:      1,
		ExecutionTime: 100,
		PodLabels: map[string]string{
			"z": "2",
			"a": "1",
		},
	}
	intentB := &domain.Intent{
		PodName:       "pod-b",
		PodID:         "pod-id-b",
		NodeID:        "node-b",
		K8sNamespace:  "kube-system",
		CommandRegex:  "busybox",
		Priority:      0,
		ExecutionTime: 200,
		PodLabels: map[string]string{
			"k2": "v2",
		},
	}
	svc := &Service{
		intentCache: []*domain.Intent{
			nil, // ensure nil input is normalized away
			intentB,
			intentA,
		},
	}

	resp, err := svc.TraverseIntentMerkleTree(context.Background(), &TraverseIntentMerkleTreeOptions{
		Depth: 0,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.RootNode)
	require.NotNil(t, svc.intentMerkleRoot)
	assert.Equal(t, svc.intentMerkleRoot.Hash, resp.RootNode.Hash)
	assert.Equal(t, svc.intentMerkleRoot.Hash, svc.intentMerkleRootHash)
}

func TestHashIntentLabelOrderIndependent(t *testing.T) {
	intentA := &domain.Intent{
		PodName:       "pod",
		PodID:         "pod-id",
		NodeID:        "node-id",
		K8sNamespace:  "default",
		CommandRegex:  "nginx",
		Priority:      1,
		ExecutionTime: 42,
		PodLabels: map[string]string{
			"b": "2",
			"a": "1",
		},
	}
	intentB := &domain.Intent{
		PodName:       "pod",
		PodID:         "pod-id",
		NodeID:        "node-id",
		K8sNamespace:  "default",
		CommandRegex:  "nginx",
		Priority:      1,
		ExecutionTime: 42,
		PodLabels: map[string]string{
			"a": "1",
			"b": "2",
		},
	}

	hashA := hashIntent(intentA)
	hashB := hashIntent(intentB)
	assert.Equal(t, hashA, hashB)
	assert.Equal(t, util.HashStringSHA256Hex("podName=pod|podID=pod-id|nodeID=node-id|k8sNamespace=default|commandRegex=nginx|priority=1|executionTime=42|podLabels=a=1,b=2"), hashA)
}

func TestTraverseIntentMerkleTreeConcurrentReadWrite(t *testing.T) {
	svc := &Service{
		intentMerkleRoot: util.BuildMerkleTree([]string{util.HashStringSHA256Hex("initial")}),
	}

	const workers = 4
	const iterations = 200

	errCh := make(chan error, workers*iterations)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			root := util.BuildMerkleTree([]string{util.HashStringSHA256Hex(fmt.Sprintf("leaf-%d", i))})
			svc.intentCacheMu.Lock()
			svc.intentMerkleRoot = root
			svc.intentCacheMu.Unlock()
		}
	}()

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				resp, err := svc.TraverseIntentMerkleTree(context.Background(), &TraverseIntentMerkleTreeOptions{Depth: 0})
				if err != nil {
					errCh <- err
					return
				}
				if resp == nil || resp.RootNode == nil || resp.RootNode.Hash == "" {
					errCh <- fmt.Errorf("unexpected nil/empty root node: %+v", resp)
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
}
