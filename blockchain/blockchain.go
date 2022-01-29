package blockchain

import (
	"fmt"

	"github.com/tncheng/multipipelinehs/crypto"
	"github.com/tncheng/multipipelinehs/types"
)

type BlockChain struct {
	forrest          *LevelledForest
	quorum           *Quorum
	longestTailBlock *Block
	// measurement
	highestComitted     int
	committedBlockNo    int
	totalBlockIntervals int
	prunedBlockNo       int
}

func NewBlockchain(n int) *BlockChain {
	bc := new(BlockChain)
	bc.forrest = NewLevelledForest()
	bc.quorum = NewQuorum(n)
	return bc
}

func (bc *BlockChain) Exists(id crypto.Identifier) bool {
	return bc.forrest.HasVertex(id)
}

func (bc *BlockChain) AddBlock(block *Block) {
	blockContainer := &BlockContainer{block}
	bc.forrest.AddVertex(blockContainer)
}

func (bc *BlockChain) AddVote(vote *Vote) (bool, *QC) {
	return bc.quorum.Add(vote)
}

func (bc *BlockChain) GetBlockByID(id crypto.Identifier) (*Block, error) {
	vertex, exists := bc.forrest.GetVertex(id)
	if !exists {
		return nil, fmt.Errorf("the block does not exist, id: %x", id)
	}
	return vertex.GetBlock(), nil
}

func (bc *BlockChain) GetParentBlock(id crypto.Identifier) (*Block, error) {
	vertex, exists := bc.forrest.GetVertex(id)
	if !exists {
		return nil, fmt.Errorf("the block does not exist, id: %x", id)
	}
	parentID, _ := vertex.Parent()
	parentVertex, exists := bc.forrest.GetVertex(parentID)
	if !exists {
		return nil, fmt.Errorf("parent block does not exist, id: %x", parentID)
	}
	return parentVertex.GetBlock(), nil
}

func (bc *BlockChain) GetGrandParentBlock(id crypto.Identifier) (*Block, error) {
	parentBlock, err := bc.GetParentBlock(id)
	if err != nil {
		return nil, fmt.Errorf("cannot get parent block: %w", err)
	}
	return bc.GetParentBlock(parentBlock.ID)
}

// CommitBlock prunes blocks and returns committed blocks up to the last committed one and prunedBlocks
func (bc *BlockChain) CommitBlock(id crypto.Identifier, seq types.Seq) ([]*Block, []*Block, error) {
	vertex, ok := bc.forrest.GetVertex(id)
	if !ok {
		return nil, nil, fmt.Errorf("cannot find the block, id: %x", id)
	}
	committedSeq := vertex.GetBlock().Seq
	bc.highestComitted = int(vertex.GetBlock().Seq)
	var committedBlocks []*Block
	for block := vertex.GetBlock(); uint64(block.Seq) > bc.forrest.LowestLevel; {
		committedBlocks = append(committedBlocks, block)
		// _, ok := bc.quorum.votes[block.ID]
		if bc.quorum.VotesExist(block.ID) {
			// delete(bc.quorum.votes, block.ID)
			bc.quorum.RemoveVote(block.ID)
		}
		bc.committedBlockNo++
		bc.totalBlockIntervals += int(seq - block.Seq)
		vertex, exists := bc.forrest.GetVertex(block.PrevID)
		if !exists {
			break
		}
		block = vertex.GetBlock()
	}
	forkedBlocks, prunedNo, err := bc.forrest.PruneUpToLevel(uint64(committedSeq))
	if err != nil {
		return nil, nil, fmt.Errorf("cannot prune the blockchain to the committed block, id: %w", err)
	}
	bc.prunedBlockNo += prunedNo

	return committedBlocks, forkedBlocks, nil
}

func (bc *BlockChain) GetChildrenBlocks(id crypto.Identifier) []*Block {
	var blocks []*Block
	iterator := bc.forrest.GetChildren(id)
	for I := iterator; I.HasNext(); {
		blocks = append(blocks, I.NextVertex().GetBlock())
	}
	return blocks
}

func (bc *BlockChain) GetChainGrowth() float64 {
	return float64(bc.committedBlockNo) / float64(bc.prunedBlockNo+1)
}

func (bc *BlockChain) GetBlockIntervals() float64 {
	return float64(bc.totalBlockIntervals) / float64(bc.committedBlockNo)
}

func (bc *BlockChain) GetHighestCommitted() int {
	return bc.highestComitted
}

func (bc *BlockChain) GetCommittedBlocks() int {
	return bc.committedBlockNo
}

func (bc *BlockChain) GetBlockByView(view types.View) *Block {
	iterator := bc.forrest.GetVerticesAtLevel(uint64(view))
	return iterator.next.GetBlock()
}
