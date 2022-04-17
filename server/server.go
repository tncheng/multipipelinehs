package main

import (
	"flag"
	"strconv"
	"sync"

	multipipelinehs "github.com/tncheng/multipipelinehs"
	"github.com/tncheng/multipipelinehs/config"
	"github.com/tncheng/multipipelinehs/crypto"
	"github.com/tncheng/multipipelinehs/identity"
	"github.com/tncheng/multipipelinehs/log"
	"github.com/tncheng/multipipelinehs/replica"
)

var algorithm = flag.String("algorithm", "mhotstuff", "BFT consensus algorithm")
var id = flag.String("id", "", "NodeID of the node")
var simulation = flag.Bool("sim", false, "simulation mode")

func initReplica(id identity.NodeID, isByz bool) {
	log.Infof("node %v starting...", id)
	if isByz {
		log.Infof("node %v is Byzantine", id)
	}
	r := replica.NewReplica(id, *algorithm, isByz)
	r.Start()
}

func main() {
	multipipelinehs.Init()
	// the private and public keys are generated here
	errCrypto := crypto.SetKeys()
	if errCrypto != nil {
		log.Fatal("Could not generate keys:", errCrypto)
	}
	if *simulation {
		var wg sync.WaitGroup
		wg.Add(1)
		config.Simulation()
		for id := range config.GetConfig().Addrs {
			isByz := false
			// if id.Node() <= config.GetConfig().ByzNo {
			// 	isByz = true
			// }
			go initReplica(id, isByz)
		}
		wg.Wait()
	} else {
		setupDebug()
		isByz := false
		i, _ := strconv.Atoi(*id)
		if i <= config.GetConfig().ByzNo {
			isByz = true
		}
		initReplica(identity.NodeID(*id), isByz)
	}
}
