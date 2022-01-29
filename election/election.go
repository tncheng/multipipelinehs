package election

import (
	"github.com/tncheng/multipipelinehs/identity"
	"github.com/tncheng/multipipelinehs/types"
)

type Election interface {
	IsLeader(id identity.NodeID, view types.View) bool
	FindLeaderFor(view types.View) identity.NodeID
}
