package pacemaker

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tncheng/multipipelinehs/blockchain"
)

// receive only one tmo
func TestRemoteTmo1(t *testing.T) {
	pm := NewPacemaker(4)
	tmo1 := &blockchain.TMO{
		View:   2,
		NodeID: "1",
	}
	isBuilt, ftc, stc := pm.ProcessRemoteTmo(tmo1)
	require.False(t, isBuilt)
	require.Nil(t, ftc, stc)
}

// receive only two tmo
func TestRemoteTmo2(t *testing.T) {
	pm := NewPacemaker(4)
	tmo1 := &blockchain.TMO{
		View:   2,
		NodeID: "1",
	}
	isBuilt, ftc, stc := pm.ProcessRemoteTmo(tmo1)
	tmo2 := &blockchain.TMO{
		View:   2,
		NodeID: "2",
	}
	isBuilt, ftc, stc = pm.ProcessRemoteTmo(tmo2)
	require.False(t, isBuilt)
	require.Nil(t, ftc, stc)
}

// receive only three tmo
func TestRemoteTmo3(t *testing.T) {
	pm := NewPacemaker(4)
	tmo1 := &blockchain.TMO{
		View:   2,
		NodeID: "1",
	}
	isBuilt, ftc, stc := pm.ProcessRemoteTmo(tmo1)
	tmo2 := &blockchain.TMO{
		View:   2,
		NodeID: "2",
	}
	isBuilt, ftc, stc = pm.ProcessRemoteTmo(tmo2)
	tmo3 := &blockchain.TMO{
		View:   2,
		NodeID: "3",
	}
	isBuilt, ftc, stc = pm.ProcessRemoteTmo(tmo3)
	require.True(t, isBuilt)
	require.NotNil(t, ftc, stc)
}

// receive four tmo
func TestRemoteTmo4(t *testing.T) {
	pm := NewPacemaker(4)
	tmo1 := &blockchain.TMO{
		View:   2,
		NodeID: "1",
	}
	isBuilt, ftc, stc := pm.ProcessRemoteTmo(tmo1)
	tmo2 := &blockchain.TMO{
		View:   2,
		NodeID: "2",
	}
	isBuilt, ftc, stc = pm.ProcessRemoteTmo(tmo2)
	tmo3 := &blockchain.TMO{
		View:   2,
		NodeID: "3",
	}
	isBuilt, ftc, stc = pm.ProcessRemoteTmo(tmo3)
	tmo4 := &blockchain.TMO{
		View:   2,
		NodeID: "4",
	}
	isBuilt, ftc, stc = pm.ProcessRemoteTmo(tmo4)
	require.False(t, isBuilt)
	require.NotNil(t, ftc, stc)
}
