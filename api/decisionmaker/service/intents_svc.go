package service

import (
	"context"
	"errors"

	"github.com/Gthulhu/api/pkg/util"
)

type TraverseIntentMerkleTreeOptions struct {
	RootHash string
	Depth    int64
}

type Node struct {
	Hash  string
	Left  *Node
	Right *Node
}

type TraverseIntentMerkleTreeResp struct {
	RootNode *Node
}

func (svc *Service) TraverseIntentMerkleTree(ctx context.Context, req *TraverseIntentMerkleTreeOptions) (resp *TraverseIntentMerkleTreeResp, err error) {
	if req == nil {
		return nil, errors.New("nil request")
	}

	svc.refreshIntentMerkleTreeIfNeeded()

	svc.intentCacheMu.RLock()
	root := svc.intentMerkleRoot
	svc.intentCacheMu.RUnlock()
	if req.RootHash != "" && root != nil {
		found := util.FindMerkleNode(root, req.RootHash)
		if found == nil {
			return &TraverseIntentMerkleTreeResp{RootNode: nil}, nil
		}
		root = found
	}

	truncated := util.TruncateMerkleTree(root, req.Depth)
	return &TraverseIntentMerkleTreeResp{RootNode: convertMerkleNode(truncated)}, nil
}

func convertMerkleNode(node *util.MerkleNode) *Node {
	if node == nil {
		return nil
	}
	return &Node{
		Hash:  node.Hash,
		Left:  convertMerkleNode(node.Left),
		Right: convertMerkleNode(node.Right),
	}
}
