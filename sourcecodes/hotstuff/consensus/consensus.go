package consensus

import (
	"encoding/csv"
	"fmt"
	"github.com/relab/hotstuff"
	"github.com/relab/hotstuff/eventloop"
	"github.com/relab/hotstuff/logging"
	"github.com/relab/hotstuff/modules"
	"github.com/relab/hotstuff/synchronizer"
	"math/rand"
	"math"
	"os"
	"strconv"
	"sync"
	"time"
)

// Rules is the minimum interface that a consensus implementations must implement.
// Implementations of this interface can be wrapped in the ConsensusBase struct.
// Together, these provide an implementation of the main Consensus interface.
// Implementors do not need to verify certificates or interact with other modules,
// as this is handled by the ConsensusBase struct.
type Rules interface {
	// VoteRule decides whether to vote for the block.
	VoteRule(proposal hotstuff.ProposeMsg) bool
	// CommitRule decides whether any ancestor of the block can be committed.
	// Returns the youngest ancestor of the block that can be committed.
	CommitRule(*hotstuff.Block) *hotstuff.Block
	// ChainLength returns the number of blocks that need to be chained together in order to commit.
	ChainLength() int
}

// ProposeRuler is an optional interface that adds a ProposeRule method.
// This allows implementors to specify how new blocks are created.
type ProposeRuler interface {
	// ProposeRule creates a new proposal.
	ProposeRule(cert hotstuff.SyncInfo, cmd hotstuff.Command) (proposal hotstuff.ProposeMsg, ok bool)
}

// consensusBase provides a default implementation of the Consensus interface
// for implementations of the ConsensusImpl interface.
type consensusBase struct {
	impl Rules

	acceptor       modules.Acceptor
	blockChain     modules.BlockChain
	commandQueue   modules.CommandQueue
	configuration  modules.Configuration
	crypto         modules.Crypto
	eventLoop      *eventloop.EventLoop
	executor       modules.ExecutorExt
	forkHandler    modules.ForkHandlerExt
	leaderRotation modules.LeaderRotation
	logger         logging.Logger
	opts           *modules.Options
	synchronizer   modules.Synchronizer
	Count          int64

	handel modules.Handel

	lastVote hotstuff.View

	mut   sync.Mutex
	bExec *hotstuff.Block
}

// New returns a new Consensus instance based on the given Rules implementation.
func New(impl Rules) modules.Consensus {
	return &consensusBase{
		impl:     impl,
		lastVote: 0,
		bExec:    hotstuff.GetGenesis(),
	}
}

// InitModule initializes the module.
func (cs *consensusBase) InitModule(mods *modules.Core) {
	mods.Get(
		&cs.acceptor,
		&cs.blockChain,
		&cs.commandQueue,
		&cs.configuration,
		&cs.crypto,
		&cs.eventLoop,
		&cs.executor,
		&cs.forkHandler,
		&cs.leaderRotation,
		&cs.logger,
		&cs.opts,
		&cs.synchronizer,
	)

	mods.TryGet(&cs.handel)

	if mod, ok := cs.impl.(modules.Module); ok {
		mod.InitModule(mods)
	}

	cs.eventLoop.RegisterHandler(hotstuff.ProposeMsg{}, func(event any) {
		cs.OnPropose(event.(hotstuff.ProposeMsg))
	})
}

func (cs *consensusBase) CommittedBlock() *hotstuff.Block {
	cs.mut.Lock()
	defer cs.mut.Unlock()
	return cs.bExec
}

// StopVoting ensures that no voting happens in a view earlier than `view`.
func (cs *consensusBase) StopVoting(view hotstuff.View) {
	if cs.lastVote < view {
		cs.lastVote = view
	}
}

func gaussianRandom() float64 {
	// 生成两个均匀分布的随机数
	u1 := rand.Float64()
	u2 := rand.Float64()

	// Box-Muller 变换公式
	z0 := math.Sqrt(-2 * math.Log(u1)) * math.Cos(2*math.Pi*u2)
	return z0
}

// 生成具有指定均值 (mu) 和方差 (variance) 的高斯随机数
// 方差是标准差的平方
func gaussianRandomWithMeanAndVariance(mu, variance float64) float64 {
	sigma := math.Sqrt(variance) // 标准差是方差的平方根
	ans := mu + sigma * gaussianRandom()
	if ans < 0{
		return -1*ans
	}
	return ans
}

// Propose creates a new proposal.
func (cs *consensusBase) Propose(cert hotstuff.SyncInfo) {
	//cs.logger.Info("propose")
	qc, ok := cert.QC()
	if ok {
		// tell the acceptor that the previous proposal succeeded.
		if qcBlock, ok := cs.blockChain.Get(qc.BlockHash()); ok { //通过qc值从链上去取对应的block，如果成功了，则说明上一个提案已经上链了。
			cs.acceptor.Proposed(qcBlock.Command()) //把上一个提案的内容加到已经成功里面。
		} else {
			cs.logger.Errorf("Could not find block for QC: %s", qc)
		}
	}

	ctx, cancel := synchronizer.TimeoutContext(cs.eventLoop.Context(), cs.eventLoop)
	defer cancel()
	cmd, ok := cs.commandQueue.Get(ctx) //从指令队列里面取出一个指令。
	if !ok {
		cs.logger.Info("Propose: no command ")
	}

	var proposal hotstuff.ProposeMsg
	if proposer, ok := cs.impl.(ProposeRuler); ok {
		proposal, ok = proposer.ProposeRule(cert, cmd) //根据cert和cmd生成一个新的proposal
		if !ok {
			cs.logger.Info("Propose: No block")
			return
		}
	} else { //如果获取proposeRuler失败，那么就直接赋值。
		proposal = hotstuff.ProposeMsg{
			ID: cs.opts.ID(),
			Block: hotstuff.NewBlock(
				qc.BlockHash(),
				qc,
				cmd,
				cs.synchronizer.View(),
				cs.opts.ID(),
			),
		}

		if aggQC, ok := cert.AggQC(); ok && cs.opts.ShouldUseAggQC() { //如果存在联合签名并且需要使用联合签名，那么就把联合签名加入到提案中
			proposal.AggregateQC = &aggQC
		}
	}

	cs.blockChain.Store(proposal.Block) //将新生成的proposal.block上链
	
	cs.configuration.Propose(proposal)
	// self vote
	cs.synchronizer.UpdateLeaderTimer()
	cs.OnPropose(proposal)
	//writing files
	//var str_list []string
	// str_list = append(str_list, "ID")
	// str_list = append(str_list, strconv.FormatInt(int64(cs.opts.ID()), 10))
	// str_list = append(str_list, "view")
	// str_list = append(str_list, strconv.FormatInt(int64(proposal.Block.View()), 10))
	// str_list = append(str_list, "Sleep_Time")
	// str_list = append(str_list, strconv.FormatInt(int64(sleep_time), 10))
	// WriteCSV("sleep_time.csv", str_list)
}

func (cs *consensusBase) OnPropose(proposal hotstuff.ProposeMsg) { //nolint:gocyclo
	// TODO: extract parts of this method into helper functions maybe?
	cs.logger.Debugf("OnPropose: %v", proposal.Block)

	block := proposal.Block

	if cs.opts.ShouldUseAggQC() && proposal.AggregateQC != nil {
		highQC, ok := cs.crypto.VerifyAggregateQC(*proposal.AggregateQC)
		if !ok {
			cs.logger.Warn("OnPropose: failed to verify aggregate QC")
			return
		}
		// NOTE: for simplicity, we require that the highQC found in the AggregateQC equals the QC embedded in the block.
		if !block.QuorumCert().Equals(highQC) {
			cs.logger.Warn("OnPropose: block QC does not equal highQC")
			return
		}
	}

	if !cs.crypto.VerifyQuorumCert(block.QuorumCert()) {
		cs.logger.Info("OnPropose: invalid QC")
		return
	}

	// ensure the block came from the leader.
	if proposal.ID != cs.leaderRotation.GetLeader(block.View()) {
		cs.logger.Info("OnPropose: block was not proposed by the expected leader")
		return
	}

	if !cs.impl.VoteRule(proposal) {
		cs.logger.Info("OnPropose: Block not voted for")
		return
	}

	if qcBlock, ok := cs.blockChain.Get(block.QuorumCert().BlockHash()); ok {
		cs.acceptor.Proposed(qcBlock.Command())
	} else {
		cs.logger.Info("OnPropose: Failed to fetch qcBlock")
	}

	if !cs.acceptor.Accept(block.Command()) {
		cs.logger.Info("OnPropose: command not accepted")
		return
	}
	// if cs.synchronizer.CheckTimeout(block.View()){
	// 	return
	// }

	// block is safe and was accepted
	cs.blockChain.Store(block)

	if b := cs.impl.CommitRule(block); b != nil {
		cs.commit(b)
	}
	cs.logger.Info("Block View: ",block.View()," arrived,while View is ",cs.synchronizer.View())
	cs.synchronizer.AdvanceView(hotstuff.NewSyncInfo().WithQC(block.QuorumCert()))
	if block.View() <= cs.lastVote {
		cs.logger.Info("OnPropose: block view too old")
		return	
	}

	pc, err := cs.crypto.CreatePartialCert(block)
	if err != nil {
		cs.logger.Error("OnPropose: failed to sign block: ", err)
		return
	}

	cs.lastVote = block.View()

	if cs.handel != nil {
		// let Handel handle the voting
		cs.handel.Begin(pc)
		return
	}

	leaderID := cs.leaderRotation.GetLeader(cs.lastVote + 1)
	if leaderID == cs.opts.ID() {
		cs.eventLoop.AddEvent(hotstuff.VoteMsg{ID: cs.opts.ID(), PartialCert: pc})
		return
	}

	leader, ok := cs.configuration.Replica(leaderID)
	if !ok {
		cs.logger.Warnf("Replica with ID %d was not found!", leaderID)
		return
	}

	leader.Vote(pc)
}

func (cs *consensusBase) commit(block *hotstuff.Block) {
	cs.mut.Lock()
	// can't recurse due to requiring the mutex, so we use a helper instead.
	err := cs.commitInner(block)
	cs.mut.Unlock()

	if err != nil {
		cs.logger.Warnf("failed to commit: %v", err)
		return
	}

	// prune the blockchain and handle forked blocks
	forkedBlocks := cs.blockChain.PruneToHeight(block.View(), cs.opts.ID())
	//cs.synchronizer.SetPDT(cs.blockChain.GetPDT())
	for _, block := range forkedBlocks {
		cs.forkHandler.Fork(block)
	}
}

// recursive helper for commit

func (cs *consensusBase) commitInner(block *hotstuff.Block) error {
	if cs.bExec.View() >= block.View() {
		return nil
	}
	if parent, ok := cs.blockChain.Get(block.Parent()); ok {
		err := cs.commitInner(parent)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("failed to locate block: %s", block.Parent())
	}
	cs.logger.Debug("EXEC: ", block)
	cs.executor.Exec(block)
	cs.Count++
	cs.logger.Info("EXECNUM: ", cs.Count)
	cs.bExec = block
	return nil
}

// ChainLength returns the number of blocks that need to be chained together in order to commit.
func (cs *consensusBase) ChainLength() int {
	return cs.impl.ChainLength()
}
func WriteCSV(filePath string, data []string) error {
	// 打开文件，如果不存在则创建
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开文件: %v", err)
	}
	defer file.Close()

	// 创建 CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入数据
	if err := writer.Write(data); err != nil {
		return fmt.Errorf("写入数据失败: %v", err)
	}

	return nil
}
