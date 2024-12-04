package consensus

import (
	"sync"

	"github.com/relab/hotstuff"
	"github.com/relab/hotstuff/eventloop"
	"github.com/relab/hotstuff/logging"
	"github.com/relab/hotstuff/modules"
)

// VotingMachine collects votes.
type VotingMachine struct {
	blockChain    modules.BlockChain
	configuration modules.Configuration
	crypto        modules.Crypto
	eventLoop     *eventloop.EventLoop
	logger        logging.Logger
	synchronizer  modules.Synchronizer
	opts          *modules.Options

	mut           sync.Mutex
	verifiedVotes map[hotstuff.Hash][]hotstuff.PartialCert // verified votes that could become a QC
}

// NewVotingMachine returns a new VotingMachine.
func NewVotingMachine() *VotingMachine {
	return &VotingMachine{
		verifiedVotes: make(map[hotstuff.Hash][]hotstuff.PartialCert),
	}
}

// InitModule initializes the VotingMachine.
func (vm *VotingMachine) InitModule(mods *modules.Core) {
	mods.Get(
		&vm.blockChain,
		&vm.configuration,
		&vm.crypto,
		&vm.eventLoop,
		&vm.logger,
		&vm.synchronizer,
		&vm.opts,
	)

	vm.eventLoop.RegisterHandler(hotstuff.VoteMsg{}, func(event any) { vm.OnVote(event.(hotstuff.VoteMsg)) })
}

// OnVote handles an incoming vote.
func (vm *VotingMachine) OnVote(vote hotstuff.VoteMsg) {
	cert := vote.PartialCert
	vm.logger.Debugf("OnVote(%d): %.8s", vote.ID, cert.BlockHash())

	var (
		block *hotstuff.Block
		ok    bool
	)

	if !vote.Deferred {
		// first, try to get the block from the local cache
		block, ok = vm.blockChain.LocalGet(cert.BlockHash())
		if !ok {
			// if that does not work, we will try to handle this event later.
			// hopefully, the block has arrived by then.
			vm.logger.Debugf("Local cache miss for block: %.8s", cert.BlockHash())
			vote.Deferred = true
			vm.eventLoop.DelayUntil(hotstuff.ProposeMsg{}, vote)
			return
		}
	} else {
		// if the block has not arrived at this point we will try to fetch it.
		block, ok = vm.blockChain.Get(cert.BlockHash())
		if !ok {
			vm.logger.Debugf("Could not find block for vote: %.8s.", cert.BlockHash())
			return
		}
	}

	//vm.logger.Info("block.View: ", block.View(), " vm.View: ", vm.synchronizer.HighQC().View())
	if block.View() <= vm.synchronizer.HighQC().View() {
		// too old
		return
	}

	if vm.opts.ShouldVerifyVotesSync() {
		vm.verifyCert(cert, block)
	} else {
		go vm.verifyCert(cert, block)
	}
}

func (vm *VotingMachine) verifyCert(cert hotstuff.PartialCert, block *hotstuff.Block) {
	if !vm.crypto.VerifyPartialCert(cert) {
		vm.logger.Info("OnVote: Vote could not be verified!")
		return
	}

	vm.mut.Lock()
	defer vm.mut.Unlock()

	// this defer will clean up any old votes in verifiedVotes
	//把block高度低于当前高度的投票全部删除（似乎删除的是多余的票）
	defer func() {
		// delete any pending QCs with lower height than bLeaf
		for k := range vm.verifiedVotes {
			if block, ok := vm.blockChain.LocalGet(k); ok {
				if block.View() <= vm.synchronizer.HighQC().View() {
					delete(vm.verifiedVotes, k)
				}
			} else {
				delete(vm.verifiedVotes, k)
			}
		}
	}()

	//疑似生成QC
	votes := vm.verifiedVotes[cert.BlockHash()]
	votes = append(votes, cert)
	vm.verifiedVotes[cert.BlockHash()] = votes

	//如果收到的投票不够，那么就继续等待
	if len(votes) < vm.configuration.QuorumSize() {
		return
	}

	//如果收到足够多的投票，那么就生成一个新的QC
	qc, err := vm.crypto.CreateQuorumCert(block, votes)
	if err != nil {
		vm.logger.Info("OnVote: could not create QC for block: ", err)
		return
	}
	//生成一个新的QC过后就删除掉所有该高度的投票。
	delete(vm.verifiedVotes, cert.BlockHash())
	vm.eventLoop.AddEvent(hotstuff.NewViewMsg{ID: vm.opts.ID(), SyncInfo: hotstuff.NewSyncInfo().WithQC(qc)})
}
