package util

import (
	"crypto/sha256"
	"encoding/hex"
)

type MerkleNode struct {
	Hash  string
	Left  *MerkleNode
	Right *MerkleNode
}

func HashSHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func HashStringSHA256Hex(value string) string {
	return HashSHA256Hex([]byte(value))
}

func BuildMerkleTree(leafHashes []string) *MerkleNode {
	if len(leafHashes) == 0 {
		return &MerkleNode{Hash: HashStringSHA256Hex("")}
	}

	nodes := make([]*MerkleNode, 0, len(leafHashes))
	for _, hash := range leafHashes {
		nodes = append(nodes, &MerkleNode{Hash: hash})
	}

	for len(nodes) > 1 {
		nextLevel := make([]*MerkleNode, 0, (len(nodes)+1)/2)
		for i := 0; i < len(nodes); i += 2 {
			left := nodes[i]
			right := left
			if i+1 < len(nodes) {
				right = nodes[i+1]
			}
			parentHash := hashMerklePair(left.Hash, right.Hash)
			nextLevel = append(nextLevel, &MerkleNode{
				Hash:  parentHash,
				Left:  left,
				Right: right,
			})
		}
		nodes = nextLevel
	}

	return nodes[0]
}

func FindMerkleNode(root *MerkleNode, hash string) *MerkleNode {
	if root == nil {
		return nil
	}
	if root.Hash == hash {
		return root
	}
	if node := FindMerkleNode(root.Left, hash); node != nil {
		return node
	}
	return FindMerkleNode(root.Right, hash)
}

func TruncateMerkleTree(root *MerkleNode, depth int64) *MerkleNode {
	if root == nil {
		return nil
	}
	if depth <= 0 {
		return &MerkleNode{Hash: root.Hash}
	}
	return &MerkleNode{
		Hash:  root.Hash,
		Left:  TruncateMerkleTree(root.Left, depth-1),
		Right: TruncateMerkleTree(root.Right, depth-1),
	}
}

func hashMerklePair(leftHash, rightHash string) string {
	leftBytes, errLeft := hex.DecodeString(leftHash)
	rightBytes, errRight := hex.DecodeString(rightHash)
	if errLeft != nil || errRight != nil {
		return HashStringSHA256Hex(leftHash + rightHash)
	}
	merged := make([]byte, 0, len(leftBytes)+len(rightBytes))
	merged = append(merged, leftBytes...)
	merged = append(merged, rightBytes...)
	return HashSHA256Hex(merged)
}
