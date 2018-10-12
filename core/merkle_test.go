package core

import (
	"container/list"
	"os"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"

	log "github.com/sirupsen/logrus"
)

func TestMerkleBuildTree(t *testing.T) {
	log.SetOutput(os.Stdout)

	var hashs [][]byte
	// empty tree
	mtree := NewMerkleTree(hashs)
	if mtree != nil {
		t.Fatal("should be an empty tree")
	}

	// single leaf, calculate with an extra empty hash
	var hash1 = make([]byte, hashLen)
	for i := 0; i < hashLen; i++ {
		hash1[i] = byte(i)
	}

	hashs = append(hashs, hash1)
	t.Logf("hashs:\n%+v", hashs)
	mtree = NewMerkleTree(hashs)
	t.Logf("tree root:%v", mtree.root)
	log.Println("****tree one****")
	printTree(mtree.root)

	// two leaves
	var hash2 = make([]byte, hashLen)
	for i := 0; i < hashLen; i++ {
		hash2[i] = byte(i * 2)
	}
	hashs = append(hashs, hash2)
	t.Logf("hashs:\n%+v", hashs)
	mtree = NewMerkleTree(hashs)
	t.Logf("tree root:%v", mtree.root)
	log.Println("****Tree Two****")
	printTree(mtree.root)

	// 4 leaves
	var hash3 = make([]byte, hashLen)
	for i := 0; i < hashLen; i++ {
		hash3[i] = byte(i * 3)
	}
	hashs = append(hashs, hash3)
	var hash4 = make([]byte, hashLen)
	for i := 0; i < hashLen; i++ {
		hash4[i] = byte(i * 4)
	}
	hashs = append(hashs, hash4)
	t.Logf("hashs:\n%+v", hashs)
	mtree = NewMerkleTree(hashs)
	t.Logf("tree root:%v", mtree.root)
	log.Println("****Tree Three****")
	printTree(mtree.root)
}

func TestMerkleProof(t *testing.T) {
	var hashs [][]byte
	hashs = append(hashs, getTestHash(1))
	hashs = append(hashs, getTestHash(2))
	hashs = append(hashs, getTestHash(3))
	hashs = append(hashs, getTestHash(4))
	mtree := NewMerkleTree(hashs)
	// printTree(mtree.root)

	var proofExpected [][]byte
	proofExpected = append(proofExpected, hashs[0])
	proofExpected = append(proofExpected, crypto.Keccak256(hashs[2], hashs[3]))
	proof := mtree.Proof(1)

	// manually verify
	if !reflect.DeepEqual(proof, proofExpected) {
		t.Fatalf("merkle proof wrong, expect:\n%v\nactual:\n%v", proofExpected, proof)
	}

	// auto verify
	if !mtree.Verify(hashs[1], mtree.root.hash, 1, proof) {
		t.Fatalf("merkle proof verification failed")
	}
}

func getTestHash(factor int) []byte {
	var hash = make([]byte, hashLen)
	for i := 0; i < hashLen; i++ {
		hash[i] = byte(i * factor)
	}
	return hash
}

// BFS the tree and print each level of nodes
func printTree(root *tnode) {
	if root == nil {
		return
	}

	parent := list.New()
	parent.PushBack(root)
	children := list.New()
	for {
		node := parent.Front()
		if node == nil {
			if children.Len() == 0 {
				log.Println("done")
				break
			}
			parent, children = children, parent
			log.Println()
			continue
		}

		tnode := node.Value.(*tnode)
		if tnode.l != nil {
			children.PushBack(tnode.l)
		}

		if tnode.r != nil {
			children.PushBack(tnode.r)
		}

		log.Print(tnode.hash)
		parent.Remove(node)
	}
}
