package service

import (
	"context"
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

// TODO: TraverseIntentMerkleTree
func (svc *Service) TraverseIntentMerkleTree(ctx context.Context, req *TraverseIntentMerkleTreeOptions) (resp *TraverseIntentMerkleTreeResp, err error) {
	// inmemory get all intents, sort by intent_id and build merkle tree
	return nil, nil
}
