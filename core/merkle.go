package core

import (
	"bytes"

	"github.com/ethereum/go-ethereum/crypto"
)

type tnode struct {
	hash []byte
	// parent, left and right children
	p, l, r *tnode
}

// MerkleTree represents a Merkle tree
type MerkleTree struct {
	leaves []*tnode // keep tracking of leaf nodes
	root   *tnode
}

const (
	hashLen = 32
)

var emptyHash = make([]byte, hashLen)

// NewMerkleTree create a merkle tree from the given hash values
// Note:
// 		if an odd length of hash values passed in, an extra empty hash value (32 bytes) will be used for padding (as the last leaf)
func NewMerkleTree(hashs [][]byte) *MerkleTree {
	if len(hashs) == 0 {
		return nil
	}

	if len(hashs)%2 != 0 {
		hashs = append(hashs, emptyHash)
	}

	mtree := &MerkleTree{}
	mtree.root = mtree.buildTree(hashs)
	return mtree
}

// Proof returns the hash values for a merkle proof of a leaf
// Params:
// 		leafIdx: the index of leaf; using the same order with the order of hash values buiding the merkle tree
func (mt MerkleTree) Proof(leafIdx int) [][]byte {
	if leafIdx >= len(mt.leaves) {
		return nil
	}

	leaf := mt.leaves[leafIdx]
	var proof [][]byte
	for {
		p := leaf.p
		if p == nil {
			break
		}
		if leaf == p.l {
			proof = append(proof, p.r.hash)
		} else {
			proof = append(proof, p.l.hash)
		}
		leaf = p
	}

	return proof
}

// Verify check whether the merkle proof is valid.
// This method can be used on an empty Merkle Tree, as we have passed all the data needed.
func (mt MerkleTree) Verify(leafHash, rootHash []byte, leafIdx int, proof [][]byte) bool {
	computedHash := leafHash

	for _, proofElement := range proof {
		if leafIdx%2 == 0 {
			computedHash = crypto.Keccak256(computedHash, proofElement)
		} else {
			computedHash = crypto.Keccak256(proofElement, computedHash)
		}
		leafIdx = leafIdx / 2
	}
	return bytes.Equal(computedHash, rootHash)
}

func (mt *MerkleTree) buildTree(hashs [][]byte) *tnode {
	if len(hashs) == 1 {
		leaf := &tnode{
			hash: hashs[0],
		}
		// track leaf
		mt.leaves = append(mt.leaves, leaf)
		return leaf
	}

	l := mt.buildTree(hashs[:len(hashs)/2]) // left subtree
	r := mt.buildTree(hashs[len(hashs)/2:]) // right subtree
	rootHash := crypto.Keccak256(l.hash, r.hash)
	root := &tnode{}
	root.l, l.p = l, root
	root.r, r.p = r, root
	root.hash = rootHash
	return root
}
