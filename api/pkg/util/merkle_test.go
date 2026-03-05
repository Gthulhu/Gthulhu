package util

import "testing"

func TestBuildMerkleTreeEmpty(t *testing.T) {
	root := BuildMerkleTree(nil)
	if root == nil {
		t.Fatalf("expected root node")
	}
	if root.Hash != HashStringSHA256Hex("") {
		t.Fatalf("unexpected hash for empty tree")
	}
}

func TestMerkleTreeTraverseAndTruncate(t *testing.T) {
	leaves := []string{
		HashStringSHA256Hex("a"),
		HashStringSHA256Hex("b"),
		HashStringSHA256Hex("c"),
	}
	root := BuildMerkleTree(leaves)
	if root == nil || root.Hash == "" {
		t.Fatalf("expected root hash")
	}

	found := FindMerkleNode(root, root.Hash)
	if found == nil || found.Hash != root.Hash {
		t.Fatalf("expected to find root by hash")
	}

	truncated := TruncateMerkleTree(root, 0)
	if truncated.Left != nil || truncated.Right != nil {
		t.Fatalf("expected depth 0 to have no children")
	}
}
