package consensus

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	pb "github.com/tendermint/tendermint/proto/tendermint/consensus"
	"io"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"time"

	"github.com/gogo/protobuf/proto"
	cfg "github.com/tendermint/tendermint/config"
	pto "github.com/tendermint/tendermint/consensus/proposetimeout"
	pbb "github.com/tendermint/tendermint/consensus/proposetimeout/block-base"
	peb "github.com/tendermint/tendermint/consensus/proposetimeout/epoch-base"
	"github.com/tendermint/tendermint/consensus/proposetimeout/merge"
	"github.com/tendermint/tendermint/consensus/proposetimeout/pcc"
	cstypes "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/crypto"
	tmevents "github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/fail"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/libs/log"
	tmmath "github.com/tendermint/tendermint/libs/math"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/libs/service"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/p2p"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// TODO 对于追赶机制，不参与共识追赶其他节点时，也是有提交区块，和高度改变的，那么其实也是可以拿到一个相应的bct
// TODO 怎样能够表明当前区块处于一个追赶机制需要立一个flag
// TODO 也许不会进入propose，我们可以使用Newheight近似替代

// Consensus sentinel errors
var (
	ErrInvalidProposalSignature   = errors.New("error invalid proposal signature")  //错误的提案签名
	ErrInvalidProposalPOLRound    = errors.New("error invalid proposal POL round")  //错误的POL锁的轮次
	ErrAddingVote                 = errors.New("error adding vote")                 //增加投票时出现错误
	ErrSignatureFoundInPastBlocks = errors.New("found signature from the same key") //发现相同的签名

	errPubKeyIsNotSet = errors.New("pubkey is not set. Look for \"Can't get private validator pubkey\" errors") //节点的公钥没有设置
)

var msgQueueSize = 1000 //消息队列的大小

// msgs from the reactor which may update the state //收到的更新状态机的消息
type msgInfo struct {
	Msg    Message `json:"msg"`
	PeerID p2p.ID  `json:"peer_key"`
}

// internally generated messages which may update the state //内部产生的更新状态机的消息 超时消息。
type timeoutInfo struct {
	Duration time.Duration         `json:"duration"`
	Height   int64                 `json:"height"`
	Round    int32                 `json:"round"`
	Step     cstypes.RoundStepType `json:"step"`
}

func (ti *timeoutInfo) String() string { //超时消息的序列化
	return fmt.Sprintf("%v ; %d/%d %v", ti.Duration, ti.Height, ti.Round, ti.Step)
}

// interface to the mempool  mempool的接口通知是否有可用的消息
type txNotifier interface {
	TxsAvailable() <-chan struct{}
}

// interface to the evidence pool 将有冲突的消息记录到证据池中
type evidencePool interface {
	// reports conflicting votes to the evidence pool to be processed into evidence
	ReportConflictingVotes(voteA, voteB *types.Vote)
}

// State handles execution of the consensus algorithm.
// It processes votes and proposals, and upon reaching agreement,
// commits blocks to the chain and executes them against the application.
// The internal state machine receives input from peers, the internal validator, and from a timer.
// 状态处理共识算法的执行。
// 它处理投票和提案，并在达成一致后，将区块提交到区块链，并对其执行应用程序。
// 内部状态机接收来自对等节点、内部验证器和定时器的输入
type State struct {
	service.BaseService

	// config details
	config        *cfg.ConsensusConfig //节点共识的配置
	privValidator types.PrivValidator  // for signing votes  //节点的私钥 用来签名

	// store blocks and commits
	blockStore sm.BlockStore //存储区块和提交

	// create and execute blocks
	blockExec *sm.BlockExecutor //创建和执行区块

	// notify us if txs are available
	txNotifier txNotifier //如果内存池中有事务，则通知我们

	// add evidence to the pool
	// when it's detected
	evpool evidencePool //检测vote时，出现冲突则驾到证据池中

	// internal state
	mtx                tmsync.RWMutex //
	cstypes.RoundState                //共识状态（HRS）
	state              sm.State       // State until height-1.//中间状态
	// privValidator pubkey, memoized for the duration of one block
	// to avoid extra requests to HSM
	privValidatorPubKey crypto.PubKey

	// state changes may be triggered by: msgs from peers,
	// msgs from ourself, or by timeouts
	peerMsgQueue     chan msgInfo
	internalMsgQueue chan msgInfo
	timeoutTicker    TimeoutTicker

	// information about about added votes and block parts are written on this channel
	// so statistics can be computed by reactor
	statsMsgQueue chan msgInfo

	//we use eventBus to trigger msg broadcasts in the reactor,
	//and to notify external subscribers, eg. through a websocket
	// eventBus 触发reactor消息广播，和通知外部的订阅者
	eventBus *types.EventBus //这个应该是单节点的

	// a Write-Ahead Log ensures we can recover from any kind of crash
	// and helps us avoid signing conflicting votes
	// 预写式日志（Write-Ahead Log）确保我们可以从任何类型的崩溃中恢复，并帮助我们避免签名冲突的投票。
	wal          WAL
	replayMode   bool // so we don't log signing errors during replay  因此，在重放期间我们不会记录签名错误。
	doWALCatchup bool // determines if we even try to do the catchup   确定我们是否尝试进行追赶

	// for tests where we want to limit the number of transitions the state makes 跳转的步骤数
	nSteps int

	// some functions can be overwritten for testing 一些函数可以被重写以进行测试。
	decideProposal func(height int64, round int32)
	doPrevote      func(height int64, round int32)
	setProposal    func(proposal *types.Proposal) error

	// closed when we finish shutting down
	done chan struct{}

	// synchronous pubsub between consensus state and reactor.
	// state only emits EventNewRoundStep and EventVote
	//共识状态和反应器之间的同步发布-订阅（pubsub）机制。
	//状态只会发出 EventNewRoundStep 和 EventVote 事件。
	evsw                     tmevents.EventSwitch
	pt                       pto.ProposeTime //存放当前区块的时间,当前的区块对应的时间。
	ptl                      pto.Timeout     //计算下一个block/epcoh的timeout值
	blt                      BlockLatency    //存放当前高度区块的延迟
	ltl                      LatencyTimeList //区块延迟列表
	crl                      CommitRoundList //存放提交轮次的队列
	flag                     bool            //表明第一次收到了整个区块  更新end时间  (记录整个区块的结束.)
	startflag                bool            //表明第一次进入propose阶段  更新start时间 (记录整个区块的开始.)
	is_byz_node              bool            //当前节点是否是拜占庭节点
	is_crash_node            bool            //当前节点是否是慢恢复节点
	id_int                   int             //当前节点的索引
	proposer                 int             //当前区块的leader
	view_change_number_group int             //一个group内viewchange的个数，具体的view-change的个数怎样去计算，应该是进入到新的轮次后
	pcc_service              pcc.PCCTimeout  //调用pcc
	mi_number                int
	cost                     *pto.EpochCost
	prevote_flag             bool
	// for reporting metrics
	LastBlockCommitTime    time.Time
	metrics                *Metrics
	tps                    *Tps
	HeightFlag             bool
	TimeoutOFHeight        int64
	proposerListofMI       []int
	bctListofMI            []float64
	smallMiofMi            int64
	validatorPDT           map[string]float64
	delay_time             float64
	block_bct_hash         string
	block_bct_delay_time   float64
	pingcmd                *exec.Cmd
	out                    bytes.Buffer
	deliver_block_time_map map[string]int64
}

// StateOption sets an optional parameter on the State.
type StateOption func(*State)

// NewState returns a new State.
func NewState(
	config *cfg.ConsensusConfig,
	state sm.State,
	blockExec *sm.BlockExecutor,
	blockStore sm.BlockStore,
	txNotifier txNotifier,
	evpool evidencePool,
	options ...StateOption,
) *State {
	cs := &State{
		config:                   config,
		blockExec:                blockExec,
		blockStore:               blockStore,
		txNotifier:               txNotifier,
		peerMsgQueue:             make(chan msgInfo, msgQueueSize),
		internalMsgQueue:         make(chan msgInfo, msgQueueSize),
		timeoutTicker:            NewTimeoutTicker(),
		statsMsgQueue:            make(chan msgInfo, msgQueueSize),
		done:                     make(chan struct{}),
		doWALCatchup:             true,
		wal:                      nilWAL{},
		evpool:                   evpool,
		evsw:                     tmevents.NewEventSwitch(),
		ltl:                      makeLatencyTimeList(config.Epoch, config.LatencyFilePath),
		crl:                      makeCommitRoundList(config.Epoch, config.RoundFilePath),
		flag:                     false,
		LastBlockCommitTime:      time.Now(),
		startflag:                false,
		is_byz_node:              false,
		prevote_flag:             false,
		view_change_number_group: 0,
		mi_number:                0,
		smallMiofMi:              0,
		validatorPDT:             make(map[string]float64),
		deliver_block_time_map:   make(map[string]int64),
		delay_time:               0.0,
		metrics:                  NopMetrics(),
	}
	if cs.config.UpdateLevel == "block" {
		cs.ptl = pbb.NewBlockProposeTimeout(cs.Height, int(cs.config.SmallMI), cs.config.TimeoutPropose.Seconds(), config.ProposeFilePath)
	} else if cs.config.UpdateLevel == "merge" {
		cs.ptl = merge.NewMergeTimeout(int64(1), cs.config.TimeoutPropose.Seconds(), config.ProposeFilePath)
	} else {
		cs.ptl = peb.NewEpochProposeTimeout(config.Epoch, config.ProposeFilePath)
	}

	cs.cost = pto.NewEpochCost(cs.config.Epoch, cs.config.CostFilePath)
	cs.tps = &Tps{MI: cs.config.MI, TxCommitNum: int64(0), Discretionary: 0.0}
	cs.HeightFlag = false
	cs.TimeoutOFHeight = cs.config.TimeoutPropose.Milliseconds()
	cs.proposerListofMI = []int{}
	cs.bctListofMI = []float64{}
	// set function defaults (may be overwritten before calling Start)
	cs.decideProposal = cs.defaultDecideProposal
	cs.doPrevote = cs.defaultDoPrevote
	cs.setProposal = cs.defaultSetProposal
	cs.validatorPDT = make(map[string]float64)

	// We have no votes, so reconstruct LastCommit from SeenCommit.
	if state.LastBlockHeight > 0 {
		cs.reconstructLastCommit(state)
	}

	cs.updateToState(state)
	fmt.Println("当前的高度", cs.Height)
	// NOTE: we do not call scheduleRound0 yet, we do that upon Start()

	cs.BaseService = *service.NewBaseService(nil, "State", cs)
	for _, option := range options {
		option(cs)
	}
	return cs
}

// SetLogger implements Service.
func (cs *State) SetLogger(l log.Logger) {
	cs.BaseService.Logger = l
	cs.timeoutTicker.SetLogger(l)
}

// SetEventBus sets event bus.
func (cs *State) SetEventBus(b *types.EventBus) {
	cs.eventBus = b
	cs.blockExec.SetEventBus(b)
}

// StateMetrics sets the metrics.
func StateMetrics(metrics *Metrics) StateOption {
	return func(cs *State) { cs.metrics = metrics }
}

// String returns a string.
func (cs *State) String() string {
	// better not to access shared variables
	return "ConsensusState"
}

// GetState returns a copy of the chain state.
func (cs *State) GetState() sm.State {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return cs.state.Copy()
}

// GetLastHeight returns the last height committed.
// If there were no blocks, returns 0.
func (cs *State) GetLastHeight() int64 {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return cs.RoundState.Height - 1
}

// GetRoundState returns a shallow copy of the internal consensus state.
func (cs *State) GetRoundState() *cstypes.RoundState {
	cs.mtx.RLock()
	rs := cs.RoundState // copy
	cs.mtx.RUnlock()
	return &rs
}

// GetRoundStateJSON returns a json of RoundState.
func (cs *State) GetRoundStateJSON() ([]byte, error) {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return tmjson.Marshal(cs.RoundState)
}

// GetRoundStateSimpleJSON returns a json of RoundStateSimple
func (cs *State) GetRoundStateSimpleJSON() ([]byte, error) {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return tmjson.Marshal(cs.RoundState.RoundStateSimple())
}

// GetValidators returns a copy of the current validators.
func (cs *State) GetValidators() (int64, []*types.Validator) {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return cs.state.LastBlockHeight, cs.state.Validators.Copy().Validators
}

// SetPrivValidator sets the private validator account for signing votes. It
// immediately requests pubkey and caches it.
func (cs *State) SetPrivValidator(priv types.PrivValidator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	cs.privValidator = priv

	if err := cs.updatePrivValidatorPubKey(); err != nil {
		cs.Logger.Error("failed to get private validator pubkey", "err", err)
	}
}

// 设置GRPC服务
func (cs *State) SetGrpcService() {
	//初始化的参数cs.config.MI 表示大MI cs.config.SmallMI表示小MI cs.config.K表示要学习的K值,
	//TODO 是否是拜占庭节点,冷启动阶段需要到高度多少
	cold_start_phase := int64(30)
	version := cs.config.Version
	is_byz := false
	id_int, _ := cs.Validators.GetByAddress(cs.privValidatorPubKey.Address())
	cs.Logger.Info("SetGrpcService", "the node's type", id_int)
	cs.id_int = int(id_int)
	for _, node_id := range cs.config.ByzantineNodeList {
		if node_id == cs.id_int {
			cs.is_byz_node = true
			is_byz = true
			cs.Logger.Info("The node is a byz node", "the node's index in validatorset", id_int)
		}
	}
	for _, node_id := range cs.config.CrashNodeList {
		if node_id == cs.id_int {
			cs.is_crash_node = true
			cs.Logger.Info("The node is a crash node", "the node's index in validatorset", id_int)
		}
	}
	delay_time := 2000.0
	fmt.Println("当前节点是否是拜占庭节点: ", cs.is_byz_node)
	cs.pcc_service = pcc.InitPCC(cs.config.MI, cs.config.SmallMI, cs.config.K, cs.config.TimeoutPropose.Milliseconds(), is_byz, cs.is_crash_node, cold_start_phase, version, delay_time, cs.config.Method, cs.config.ParaA)
	cs.delay_time = cs.pcc_service.DelayTime
	cs.deliver_block_time_map = cs.ReadDeliverBlockTimeList()
	fmt.Println("当前的MI的长度", cs.pcc_service.SizeNumber, len(cs.pcc_service.SizeList), "当前执行的个数", cs.pcc_service.ExcutedNumber)
}

func (cs *State) ReadDeliverBlockTimeList() map[string]int64 {
	deliver_block_time_list := make(map[string]int64)
	file, err := os.Open(cs.config.CreateBlockFilePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return deliver_block_time_list
	}
	defer file.Close()

	// 创建 CSV 读取器
	reader := csv.NewReader(file)

	// 读取所有行
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println("Error reading CSV:", err)
		return deliver_block_time_list
	}

	// 打印每一行
	for _, record := range records {
		deliver_time, err := strconv.ParseFloat(record[1], 64) // 将年龄从字符串转换为int64
		if err != nil {
			fmt.Println("Error converting age:", err)
			continue // 跳过此行
		}
		deliver_block_time_list[record[0]] = int64(deliver_time)
	}

	return deliver_block_time_list
}

// SetTimeoutTicker sets the local timer. It may be useful to overwrite for
// testing.
func (cs *State) SetTimeoutTicker(timeoutTicker TimeoutTicker) {
	cs.mtx.Lock()
	cs.timeoutTicker = timeoutTicker
	cs.mtx.Unlock()
}

// LoadCommit loads the commit for a given height.  //这个是一个区块提交的凭证.
func (cs *State) LoadCommit(height int64) *types.Commit {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	if height == cs.blockStore.Height() {
		return cs.blockStore.LoadSeenCommit(height)
	}

	return cs.blockStore.LoadBlockCommit(height)
}

// OnStart loads the latest state via the WAL, and starts the proposetimeout and
// receive routines.  启动超时和接收的进程
func (cs *State) OnStart() error {
	// We may set the WAL in testing before calling Start, so only OpenWAL if its
	// still the nilWAL.
	if _, ok := cs.wal.(nilWAL); ok {
		if err := cs.loadWalFile(); err != nil {
			return err
		}
	}

	// we need the timeoutRoutine for replay so
	// we don't block on the tick chan.
	// NOTE: we will get a build up of garbage go routines
	// firing on the tockChan until the receiveRoutine is started
	// to deal with them (by that point, at most one will be valid)
	if err := cs.timeoutTicker.Start(); err != nil {
		return err
	}

	// We may have lost some votes if the process crashed reload from consensus
	// log to catchup.
	if cs.doWALCatchup {
		repairAttempted := false

	LOOP:
		for {
			err := cs.catchupReplay(cs.Height)
			switch {
			case err == nil:
				break LOOP

			case !IsDataCorruptionError(err):
				cs.Logger.Error("error on catchup replay; proceeding to start state anyway", "err", err)
				break LOOP

			case repairAttempted:
				return err
			}

			cs.Logger.Error("the WAL file is corrupted; attempting repair", "err", err)

			// 1) prep work
			if err := cs.wal.Stop(); err != nil {
				return err
			}

			repairAttempted = true

			// 2) backup original WAL file
			corruptedFile := fmt.Sprintf("%s.CORRUPTED", cs.config.WalFile())
			if err := tmos.CopyFile(cs.config.WalFile(), corruptedFile); err != nil {
				return err
			}

			cs.Logger.Debug("backed up WAL file", "src", cs.config.WalFile(), "dst", corruptedFile)

			// 3) try to repair (WAL file will be overwritten!)
			if err := repairWalFile(corruptedFile, cs.config.WalFile()); err != nil {
				cs.Logger.Error("the WAL repair failed", "err", err)
				return err
			}

			cs.Logger.Info("successful WAL repair")

			// reload WAL file
			if err := cs.loadWalFile(); err != nil {
				return err
			}
		}
	}

	if err := cs.evsw.Start(); err != nil {
		return err
	}

	// Double Signing Risk Reduction
	if err := cs.checkDoubleSigningRisk(cs.Height); err != nil {
		return err
	}

	// now start the receiveRoutine
	go cs.receiveRoutine(0)

	// schedule the first round!
	// use GetRoundState so we don't race the receiveRoutine for access
	cs.scheduleRound0(cs.GetRoundState())

	return nil
}

// timeoutRoutine: receive requests for timeouts on tickChan and fire timeouts on tockChan
// receiveRoutine: serializes processing of proposoals, block parts, votes; coordinates state transitions
func (cs *State) startRoutines(maxSteps int) {
	err := cs.timeoutTicker.Start()
	if err != nil {
		cs.Logger.Error("failed to start proposetimeout ticker", "err", err)
		return
	}

	go cs.receiveRoutine(maxSteps)
}

// loadWalFile loads WAL data from file. It overwrites cs.wal.
func (cs *State) loadWalFile() error {
	wal, err := cs.OpenWAL(cs.config.WalFile())
	if err != nil {
		cs.Logger.Error("failed to load state WAL", "err", err)
		return err
	}

	cs.wal = wal
	return nil
}

// OnStop implements service.Service.
func (cs *State) OnStop() {
	if err := cs.evsw.Stop(); err != nil {
		cs.Logger.Error("failed trying to stop eventSwitch", "error", err)
	}

	if err := cs.timeoutTicker.Stop(); err != nil {
		cs.Logger.Error("failed trying to stop timeoutTicket", "error", err)
	}
	// WAL is stopped in receiveRoutine.
}

// Wait waits for the the main routine to return.
// NOTE: be sure to Stop() the event switch and drain
// any event channels or this may deadlock
func (cs *State) Wait() {
	<-cs.done
}

// OpenWAL opens a file to log all consensus messages and timeouts for
// deterministic accountability.
func (cs *State) OpenWAL(walFile string) (WAL, error) {
	wal, err := NewWAL(walFile)
	if err != nil {
		cs.Logger.Error("failed to open WAL", "file", walFile, "err", err)
		return nil, err
	}

	wal.SetLogger(cs.Logger.With("wal", walFile))

	if err := wal.Start(); err != nil {
		cs.Logger.Error("failed to start WAL", "err", err)
		return nil, err
	}

	return wal, nil
}

//------------------------------------------------------------
// Public interface for passing messages into the consensus state, possibly causing a state transition.
// If peerID == "", the msg is considered internal.
// Messages are added to the appropriate queue (peer or internal).
// If the queue is full, the function may block.
// TODO: should these return anything or let callers just use events?

// AddVote inputs a vote.
func (cs *State) AddVote(vote *types.Vote, peerID p2p.ID) (added bool, err error) {
	if peerID == "" {
		cs.internalMsgQueue <- msgInfo{&VoteMessage{vote}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&VoteMessage{vote}, peerID}
	}

	// TODO: wait for event?!
	return false, nil
}

// SetProposal inputs a proposal.
func (cs *State) SetProposal(proposal *types.Proposal, peerID p2p.ID) error {
	if peerID == "" {
		cs.internalMsgQueue <- msgInfo{&ProposalMessage{proposal}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&ProposalMessage{proposal}, peerID}
	}

	// TODO: wait for event?!
	return nil
}

// AddProposalBlockPart inputs a part of the proposal block.
func (cs *State) AddProposalBlockPart(height int64, round int32, part *types.Part, peerID p2p.ID) error {
	if peerID == "" {
		cs.internalMsgQueue <- msgInfo{&BlockPartMessage{height, round, part}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&BlockPartMessage{height, round, part}, peerID}
	}

	// TODO: wait for event?!
	return nil
}

// SetProposalAndBlock inputs the proposal and all block parts.
func (cs *State) SetProposalAndBlock(
	proposal *types.Proposal,
	block *types.Block,
	parts *types.PartSet,
	peerID p2p.ID,
) error {
	if err := cs.SetProposal(proposal, peerID); err != nil {
		return err
	}

	for i := 0; i < int(parts.Total()); i++ {
		part := parts.GetPart(i)
		if err := cs.AddProposalBlockPart(proposal.Height, proposal.Round, part, peerID); err != nil {
			return err
		}
	}

	return nil
}

// ------------------------------------------------------------
// internal functions for managing the state
func (cs *State) updateProposeTime(height int64) {
	cs.pt = pto.MakeNewProposeTime()
	cs.pt.Height = height
}

func (cs *State) updateProposeStartTime(starttime time.Time, round int64) {
	cs.pt.Propose_start_time = starttime
	cs.pt.Start_round = round
}
func (cs *State) updateProposeEndTime(endtime time.Time, round int64) {
	cs.pt.Propose_end_time = endtime
	cs.pt.End_round = round
}

func (cs *State) addProposeStartTime(round string, starttime time.Time) {
	cs.pt.AddProposeStartTime(round, starttime)

}

func (cs *State) addProposeEndTime(block_hash string, endtime time.Time) {
	cs.pt.AddProposeEndTime(block_hash, endtime)

}

func (cs *State) updateLatencyTime(height int64) {
	cs.blt = BlockLatency{}
	cs.blt.height = height
}
func (cs *State) updateLatencyStartTime(starttime time.Time, round int64) {
	cs.blt.start_time = starttime
}
func (cs *State) updateLatencyEndTime(endtime time.Time, round int64) {
	cs.blt.end_time = endtime

}

// 更新高度
func (cs *State) updateHeight(height int64) {
	cs.metrics.Height.Set(float64(height))
	value := strconv.Itoa(int(height))
	if cs.config.CC {
		err := ioutil.WriteFile("/etc/tendermint/output.txt", []byte(value), 0644)
		if err != nil {
			fmt.Println("写入文件失败:", err)
			return
		}
	}

	//

	cs.Height = height
	cs.HeightFlag = false
	//if cs.Height > int64(1) {
	//	cs.Logger.Info("the time is updating block Height sleeping 1s")
	//}
}

// HeightFalg每个高度只是在ture，其余的更新仅在更新高度的时候置为false
func (cs *State) updateRoundStep(round int32, step cstypes.RoundStepType) {
	if cs.Round != round || !cs.HeightFlag {
		cs.HeightFlag = true
		cs.prevote_flag = false //TODO 如果有需要其实可以将这个东西和round绑定。估计会都进入 prevote的轮次不一定有
		cs.cost.CostList[len(cs.cost.CostList)-1].CrashFlag = false
		if round == int32(0) {
			cs.cost.CostList[len(cs.cost.CostList)-1].Round0StartTime = time.Now()
		}
		cs.cost.CostList[len(cs.cost.CostList)-1].RoundCommitStartTime = time.Now() //对应区块提交的时间。
		cs.cost.CostList[len(cs.cost.CostList)-1].AfterRound = round

		if round != int32(0) {
			cs.Logger.Info("The start time of new round:", "round", round)
			cs.cost.CostList[len(cs.cost.CostList)-1].EnterNewRoundTime = append(cs.cost.CostList[len(cs.cost.CostList)-1].EnterNewRoundTime, time.Now()) //进入到新的
			cs.cost.CostList[len(cs.cost.CostList)-1].NewRoundRound = append(cs.cost.CostList[len(cs.cost.CostList)-1].NewRoundRound, round)
		}

	}
	cs.Round = round
	cs.Step = step
	//TODO 许多变量是可以在改变的 在这改变是对的。统计的位置。
}

// enterNewRound(height, 0) at cs.StartTime.
// 在现在的这个时间点+timeoutCommit后其中
func (cs *State) scheduleRound0(rs *cstypes.RoundState) {
	// cs.Logger.Info("scheduleRound0", "now", tmtime.Now(), "startTime", cs.StartTime)
	sleepDuration := rs.StartTime.Sub(tmtime.Now()) //可以约等于自己的committimeout
	cs.scheduleTimeout(sleepDuration, rs.Height, 0, cstypes.RoundStepNewHeight)
}

// Attempt to schedule a proposetimeout (by sending timeoutInfo on the tickChan)
func (cs *State) scheduleTimeout(duration time.Duration, height int64, round int32, step cstypes.RoundStepType) {
	cs.timeoutTicker.ScheduleTimeout(timeoutInfo{duration, height, round, step})
}

// send.sh a msg into the receiveRoutine regarding our own proposal, block part, or vote
// 这个其实就是在异步的实现消息的发送，队列满了之后，要等着队列中的其他消息处理完成之后，再进入队列
func (cs *State) sendInternalMessage(mi msgInfo) {
	select {
	case cs.internalMsgQueue <- mi:
	default:
		// NOTE: using the go-routine means our votes can
		// be processed out of order.
		// TODO: use CList here for strict determinism and
		// attempt push to internalMsgQueue in receiveRoutine
		cs.Logger.Info("internal msg queue is full; using a go-routine")
		go func() { cs.internalMsgQueue <- mi }()
	}
}

// Reconstruct LastCommit from SeenCommit, which we saved along with the block,
// (which happens even before saving the state)
func (cs *State) reconstructLastCommit(state sm.State) {
	seenCommit := cs.blockStore.LoadSeenCommit(state.LastBlockHeight)
	if seenCommit == nil {
		panic(fmt.Sprintf(
			"failed to reconstruct last commit; seen commit for height %v not found",
			state.LastBlockHeight,
		))
	}

	lastPrecommits := types.CommitToVoteSet(state.ChainID, seenCommit, state.LastValidators)
	if !lastPrecommits.HasTwoThirdsMajority() {
		panic("failed to reconstruct last commit; does not have +2/3 maj")
	}

	cs.LastCommit = lastPrecommits
}

// Updates State and increments height to match that of state.
// The round becomes 0 and cs.Step becomes cstypes.RoundStepNewHeight.
// 通过更新cs状态和和增加相应的高度 并且将round=0 stepNewHeigth
// 如果提交轮次大于-1并且cs当前的高度不等于state的上一个高度，则希望是cs的高度，但是收到的state.Laststate的高度（说明提交之后要先更新sm.state）

func (cs *State) updateToState(state sm.State) {
	//不是第一个块的情况
	if cs.CommitRound > -1 && 0 < cs.Height && cs.Height != state.LastBlockHeight {
		panic(fmt.Sprintf(
			"updateToState() expected state height of %v but found %v",
			cs.Height, state.LastBlockHeight,
		))
	}

	if !cs.state.IsEmpty() {
		//这个是直接操作cs.state
		if cs.state.LastBlockHeight > 0 && cs.state.LastBlockHeight+1 != cs.Height {
			// This might happen when someone else is mutating cs.state.
			// Someone forgot to pass in state.Copy() somewhere?!
			panic(fmt.Sprintf(
				"inconsistent cs.state.LastBlockHeight+1 %v vs cs.Height %v",
				cs.state.LastBlockHeight+1, cs.Height,
			))
		}
		//已经提交了区块，但是cs现在的状态还在initialHeight
		if cs.state.LastBlockHeight > 0 && cs.Height == cs.state.InitialHeight {
			panic(fmt.Sprintf(
				"inconsistent cs.state.LastBlockHeight %v, expected 0 for initial height %v",
				cs.state.LastBlockHeight, cs.state.InitialHeight,
			))
		}

		// If state isn't further out than cs.state, just ignore.
		// This happens when SwitchToConsensus() is called in the reactor.
		// We don't want to reset e.g. the Votes, but we still want to
		// signal the new round step, because other services (eg. txNotifier)
		// depend on having an up-to-date peer state!
		// 如果输入的状态不如cs的状态提前，则忽略该输入状态。把当前的同步给你其他节点和
		if state.LastBlockHeight <= cs.state.LastBlockHeight {
			cs.Logger.Debug(
				"ignoring updateToState()",
				"new_height", state.LastBlockHeight+1,
				"old_height", cs.state.LastBlockHeight+1,
			)
			cs.newStep()
			return
		}
	}

	// Reset fields based on state.
	validators := state.Validators //重置验证者集合

	switch {
	//当前是提交的第一个区块 上次的提交置为空
	case state.LastBlockHeight == 0: // Very first commit should be empty.
		cs.LastCommit = (*types.VoteSet)(nil)
	//	验证如果不是第一提交的区块时，该提交的区块的提交票是否满足2/3
	case cs.CommitRound > -1 && cs.Votes != nil: // Otherwise, use cs.Votes
		if !cs.Votes.Precommits(cs.CommitRound).HasTwoThirdsMajority() {
			panic(fmt.Sprintf(
				"wanted to form a commit, but precommits (H/R: %d/%d) didn't have 2/3+: %v",
				state.LastBlockHeight, cs.CommitRound, cs.Votes.Precommits(cs.CommitRound),
			))
		}

		cs.LastCommit = cs.Votes.Precommits(cs.CommitRound)

	case cs.LastCommit == nil:
		// NOTE: when Tendermint starts, it has no votes. reconstructLastCommit
		// must be called to reconstruct LastCommit from SeenCommit.
		panic(fmt.Sprintf(
			"last commit cannot be empty after initial block (H:%d)",
			state.LastBlockHeight+1,
		))
	}

	// Next desired block height
	height := state.LastBlockHeight + 1
	if height == 1 {
		height = state.InitialHeight
	}

	fmt.Println("Height", height)
	// RoundState fields

	cs.updateHeight(height)
	fmt.Println("the height starting", "Height="+strconv.Itoa(int(height))+"starting")
	cs.cost.CostList = append(cs.cost.CostList, &pto.Cost{})
	cs.updateRoundStep(0, cstypes.RoundStepNewHeight) // TODO:进入到了newheight
	cs.block_bct_hash = ""
	cs.block_bct_delay_time = 0.0
	cs.out.Reset()

	cs.pingcmd = exec.Command("ping", "-i", "0.5", cs.config.PingNode)
	cs.pingcmd.Stdout = &cs.out
	// 启动 ping 命令
	if err := cs.pingcmd.Start(); err != nil {
		panic("the pingcmd error")

	}
	cs.updateProposeTime(height)
	cs.updateLatencyTime(height)

	//第一个区块是当前的时间+timeoutCommit 后面的区块的开始时间是上个区块的提交时间+timeoutCommit
	if cs.CommitTime.IsZero() {
		//"Now" makes it easier to sync up dev nodes.
		//We add timeoutCommit to allow transactions
		//to be gathered for the first block.
		//And alternative solution that relies on clocks:
		//cs.StartTime = state.LastBlockTime.Add(timeoutCommit)
		cs.StartTime = cs.config.Commit(tmtime.Now())
	} else {
		cs.StartTime = cs.config.Commit(cs.CommitTime)
	}

	cs.updateLatencyStartTime(cs.LastBlockCommitTime, int64(cs.Round))

	cs.Validators = validators
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockParts = nil
	cs.LockedRound = -1
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	cs.ValidRound = -1
	cs.ValidBlock = nil
	cs.ValidBlockParts = nil
	cs.Votes = cstypes.NewHeightVoteSet(state.ChainID, height, validators)
	cs.CommitRound = -1
	cs.LastValidators = state.LastValidators
	//这个flag是什么？也是为了解决空块的超时问题
	cs.TriggeredTimeoutPrecommit = false

	cs.state = state

	//TODO   这个需不需要改 不是第一个的时间。
	cs.flag = false
	cs.startflag = false

	if cs.Height%cs.config.MI == int64(1) {
		if cs.Height > 1 {
			cs.Logger.Info("the start time of the MI", "StartTime", time.Now())
		}
		cs.tps.StartTime = time.Now()
	}
	// Finally, broadcast RoundState
	cs.newStep()
}

func (cs *State) newStep() {
	rs := cs.RoundStateEvent()
	if err := cs.wal.Write(rs); err != nil {
		cs.Logger.Error("failed writing to WAL", "err", err)
	}

	cs.nSteps++ //步数

	// newStep is called by updateToState in NewState before the eventBus is set!
	if cs.eventBus != nil {
		if err := cs.eventBus.PublishEventNewRoundStep(rs); err != nil {
			cs.Logger.Error("failed publishing new round step", "err", err)
		}

		cs.evsw.FireEvent(types.EventNewRoundStep, &cs.RoundState)
	}
}

//-----------------------------------------
// the main go routines

// receiveRoutine handles messages which may cause state transitions.
// it's argument (n) is the number of messages to process before exiting - use 0 to run forever
// It keeps the RoundState and is the only thing that updates it.
// Updates (state transitions) happen on timeouts, complete proposals, and 2/3 majorities.
// State must be locked before any internal state is updated.
// 其实大体可以分为2种类型的投票，超时和其他类型
func (cs *State) receiveRoutine(maxSteps int) {
	onExit := func(cs *State) {
		// NOTE: the internalMsgQueue may have signed messages from our
		// priv_val that haven't hit the WAL, but its ok because
		// priv_val tracks LastSig

		// close wal now that we're done writing to it
		if err := cs.wal.Stop(); err != nil {
			cs.Logger.Error("failed trying to stop WAL", "error", err)
		}

		cs.wal.Wait()
		close(cs.done)
	}

	defer func() {
		if r := recover(); r != nil {
			cs.Logger.Error("CONSENSUS FAILURE!!!", "err", r, "stack", string(debug.Stack()))
			// stop gracefully
			//
			// NOTE: We most probably shouldn't be running any further when there is
			// some unexpected panic. Some unknown error happened, and so we don't
			// know if that will result in the validator signing an invalid thing. It
			// might be worthwhile to explore a mechanism for manual resuming via
			// some console or secure RPC system, but for now, halting the chain upon
			// unexpected consensus bugs sounds like the better option.
			onExit(cs)
		}
	}()

	for {
		if maxSteps > 0 {
			if cs.nSteps >= maxSteps {
				cs.Logger.Debug("reached max steps; exiting receive routine")
				cs.nSteps = 0
				return
			}
		}

		rs := cs.RoundState
		var mi msgInfo
		//fmt.Println("当前各个队列的长度: ", "peer队列的长度", len(cs.peerMsgQueue), "inter队列的长度", len(cs.internalMsgQueue), "超时队列的长度", len(cs.timeoutTicker.Chan()))
		select {
		case ti := <-cs.timeoutTicker.Chan(): // tockChan:
			cs.Logger.Info("处理超时")
			if err := cs.wal.Write(ti); err != nil {
				cs.Logger.Error("failed writing to WAL", "err", err)
			}

			// if the proposetimeout is r elevant to the rs
			// go to the next step
			cs.handleTimeout(ti, rs)
		default:

		}
		select {

		case <-cs.txNotifier.TxsAvailable():
			cs.Logger.Info("处理peer节点的消息")
			cs.handleTxsAvailable()

		case mi = <-cs.peerMsgQueue:
			//cs.Logger.Info("处理peer节点的消息")
			if err := cs.wal.Write(mi); err != nil {
				cs.Logger.Error("failed writing to WAL", "err", err)
			}

			// handles proposals, block parts, votes
			// may generate internal events (votes, complete proposals, 2/3 majorities)
			cs.handleMsg(mi)

		case mi = <-cs.internalMsgQueue:
			cs.Logger.Debug("处理本节点的消息")
			err := cs.wal.WriteSync(mi) // NOTE: fsync
			if err != nil {
				panic(fmt.Sprintf(
					"failed to write %v msg to consensus WAL due to %v; check your file system and restart the node",
					mi, err,
				))
			}

			if _, ok := mi.Msg.(*VoteMessage); ok {
				// we actually want to simulate failing during
				// the previous WriteSync, but this isn't easy to do.
				// Equivalent would be to fail here and manually remove
				// some bytes from the end of the wal.
				fail.Fail() // XXX
			}

			// handles proposals, block parts, votes
			cs.handleMsg(mi)

		case ti := <-cs.timeoutTicker.Chan(): // tockChan:
			cs.Logger.Info("处理超时")
			if err := cs.wal.Write(ti); err != nil {
				cs.Logger.Error("failed writing to WAL", "err", err)
			}

			// if the proposetimeout is r elevant to the rs
			// go to the next step
			cs.handleTimeout(ti, rs)

		case <-cs.Quit():
			onExit(cs)
			return
		}
	}
}

// 将hash值直接映射为string
func bytetostring(hash []byte) string {
	return hex.EncodeToString(hash)
	// return string(hash)
}

//// 将string转为hash值
//func stringtobyte(hash string) []byte {
//	result, _ := hex.DecodeString(hash)
//	return result
//}

// state transitions on complete-proposal, 2/3-any, 2/3-one
func (cs *State) handleMsg(mi msgInfo) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	var (
		added bool
		err   error
	)

	msg, peerID := mi.Msg, mi.PeerID

	switch msg := msg.(type) {
	case *ProposalMessage:
		// will not cause transition.
		// once proposal is set, we can receive block parts
		//peeer_index, _ := cs.Validators.GetPeerIndexbyID(string(peerID))
		cs.Logger.Debug("deliver proposal peer ", "id", peerID)
		err = cs.setProposal(msg.Proposal)

	case *BlockPartMessage:
		// if the proposal is complete, we'll enterPrevote or tryFinalizeCommit
		cs.Logger.Debug("deliver block part peer ", "id", peerID)
		added, err = cs.addProposalBlockPart(msg, peerID)
		// We unlock here to yield to any routines that need to read the the RoundState.
		// Previously, this code held the lock from the point at which the final block
		// part was received until the block executed against the application.
		// This prevented the reactor from being able to retrieve the most updated
		// version of the RoundState. The reactor needs the updated RoundState to
		// gossip the now completed block.
		//
		// This code can be further improved by either always operating on a copy
		// of RoundState and only locking when switching out State's copy of
		// RoundState with the updated copy or by emitting RoundState events in
		// more places for routines depending on it to listen for.
		cs.mtx.Unlock()

		cs.mtx.Lock()
		if added && cs.ProposalBlockParts.IsComplete() {
			cs.Logger.Info("add block hash", "block_hash", bytetostring(cs.ProposalBlockParts.Hash()))
			cs.addProposeEndTime(bytetostring(cs.ProposalBlockParts.Hash()), time.Now())
			if !cs.flag {
				cs.flag = true
				cs.block_bct_hash = bytetostring(cs.ProposalBlockParts.Hash())
				cs.block_bct_delay_time = cs.ProposalBlock.DelayTime
				cs.updateProposeEndTime(time.Now(), int64(cs.Round))
				cs.Logger.Debug("receive block time", "start_time", cs.pt.Propose_start_time,
					"start_round", cs.pt.Start_round, "end_time", cs.pt.Propose_end_time, "end_round", cs.pt.End_round)
			}

			cs.handleCompleteProposal(msg.Height)
		}
		if added {
			cs.statsMsgQueue <- mi
		}

		if err != nil && msg.Round != cs.Round {
			cs.Logger.Debug(
				"received block part from wrong round",
				"height", cs.Height,
				"cs_round", cs.Round,
				"block_round", msg.Round,
			)
			err = nil
		}

	case *VoteMessage:
		cs.Logger.Debug("add vote")
		// attempt to add the vote and dupeout the validator if its a duplicate signature
		// if the vote gives us a 2/3-any or 2/3-one, we transition  TODO 中的2/3-any和2/3-one的区别
		added, err = cs.tryAddVote(msg.Vote, peerID)
		if added {
			cs.statsMsgQueue <- mi
		}

		// if err == ErrAddingVote {
		// TODO: punish peer
		// We probably don't want to stop the peer here. The vote does not
		// necessarily comes from a malicious peer but can be just broadcasted by
		// a typical peer.
		// https://github.com/tendermint/tendermint/issues/1281
		// }

		// NOTE: the vote is broadcast to peers by the reactor listening
		// for vote events

		// TODO: If rs.Height == vote.Height && rs.Round < vote.Round,
		// the peer is sending us CatchupCommit precommits.   TODO CatchupCommit 的含义
		// We could make note of this and help filter in broadcastHasVoteMessage().

	default:

		cs.Logger.Error("unknown msg type", "type", fmt.Sprintf("%T", msg))
		return
	}

	if err != nil {
		cs.Logger.Error(
			"failed to process message",
			"height", cs.Height,
			"round", cs.Round,
			"peer", peerID,
			"msg_type", fmt.Sprintf("%T", msg),
			"err", err,
		)
	}
}

func (cs *State) handleTimeout(ti timeoutInfo, rs cstypes.RoundState) {
	cs.Logger.Debug("received tock", "proposetimeout", ti.Duration, "height", ti.Height, "round", ti.Round, "step", ti.Step)

	// timeouts must be for current height, round, step
	if ti.Height != rs.Height || ti.Round < rs.Round || (ti.Round == rs.Round && ti.Step < rs.Step) { //这句化也比较重要,怎样用进来呢
		cs.Logger.Debug("ignoring tock because we are ahead", "height", rs.Height, "round", rs.Round, "step", rs.Step)
		return
	}

	// the proposetimeout will now cause a state transition
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	switch ti.Step {
	case cstypes.RoundStepNewHeight: //这应该是第一次进入到newreound中
		// NewRound event fired from enterNewRound.
		// XXX: should we fire proposetimeout here (for proposetimeout commit)?
		cs.enterNewRound(ti.Height, 0)

	case cstypes.RoundStepNewRound:
		cs.enterPropose(ti.Height, 0)

	case cstypes.RoundStepPropose:
		if err := cs.eventBus.PublishEventTimeoutPropose(cs.RoundStateEvent()); err != nil {
			cs.Logger.Error("failed publishing proposetimeout propose", "err", err)
		}
		cs.view_change_number_group = cs.view_change_number_group + 1
		cs.Logger.Info("proposetimeout propose", "$$$$$$$$$$$$$$$$$$$$$$$$$$$", "#########################")
		fmt.Println("从非正常情况进入")
		cs.enterPrevote(ti.Height, ti.Round)

	case cstypes.RoundStepPrevoteWait:
		if err := cs.eventBus.PublishEventTimeoutWait(cs.RoundStateEvent()); err != nil {
			cs.Logger.Error("failed publishing proposetimeout wait", "err", err)
		}

		cs.enterPrecommit(ti.Height, ti.Round)

	case cstypes.RoundStepPrecommitWait:
		if err := cs.eventBus.PublishEventTimeoutWait(cs.RoundStateEvent()); err != nil {
			cs.Logger.Error("failed publishing proposetimeout wait", "err", err)
		}

		cs.enterPrecommit(ti.Height, ti.Round) //TODO 这里是不是提交个空块
		//TODO:这里为什么进入NewRound
		cs.enterNewRound(ti.Height, ti.Round+1) //这个也可能是

	default:
		panic(fmt.Sprintf("invalid proposetimeout step: %v", ti.Step))
	}
}

// 处理事务池中存在可用事务的消息
// 如果不是第一轮，则返回？，TODO 其他轮次会换主吗？
//
//	若是第一轮{
//	     如果是newheight阶段，
//	}
func (cs *State) handleTxsAvailable() {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	// We only need to do this for round 0.
	if cs.Round != 0 {
		return
	}

	switch cs.Step {
	case cstypes.RoundStepNewHeight: // timeoutCommit phase
		//这个应该是表明上个区块已经提交了  其实就是不需要交易 还是继续等待。
		if cs.needProofBlock(cs.Height) {
			// enterPropose will be called by enterNewRound
			return
		}
		//这个应该是上一个区块还没有完成提交，所以要等待一下上个区块进行提交之后再进行进入NewRound  TODO 为什么进入了新的高度还是appHash还是没有改变
		// +1ms to ensure RoundStepNewRound timeout always happens after RoundStepNewHeight
		timeoutCommit := cs.StartTime.Sub(tmtime.Now()) + 1*time.Millisecond
		cs.scheduleTimeout(timeoutCommit, cs.Height, 0, cstypes.RoundStepNewRound)

	case cstypes.RoundStepNewRound: // after timeoutCommit
		cs.enterPropose(cs.Height, 0)
	}
}

//-----------------------------------------------------------------------------
// State functions
// Used internally by handleTimeout and handleMsg to make state transitions

// Enter: `timeoutNewHeight` by startTime (commitTime+timeoutCommit),
//
//	or, if SkipTimeoutCommit==true, after receiving all precommits from (height,round-1)
//
// Enter: `timeoutPrecommits` after any +2/3 precommits from (height,round-1)
// Enter: +2/3 precommits for nil at (height,round-1)
// Enter: +2/3 prevotes any or +2/3 precommits for block or any from (height, round)
// NOTE: cs.StartTime was already set for height.
// 这个函数里面只是将提案置为空了。
func (cs *State) enterNewRound(height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)
	cs.addProposeStartTime(strconv.Itoa(int(round)), time.Now())
	if cs.Height != height || round < cs.Round || (cs.Round == round && cs.Step != cstypes.RoundStepNewHeight) {
		logger.Debug(
			"entering new round with invalid args",
			"current", log.NewLazySprintf("%v/%v/%v", cs.Height, cs.Round, cs.Step),
		)
		return
	}

	if now := tmtime.Now(); cs.StartTime.After(now) {
		logger.Debug("need to set a buffer and log message here for sanity", "start_time", cs.StartTime, "now", now)
	}

	logger.Info("entering new round", "current", log.NewLazySprintf("%v/%v/%v", cs.Height, cs.Round, cs.Step))

	// increment validators if necessary
	validators := cs.Validators
	if cs.Round < round {
		validators = validators.Copy()
		validators.IncrementProposerPriority(tmmath.SafeSubInt32(round, cs.Round))
	}

	// Setup new round
	// we don't fire newStep for this step,
	// but we fire an event, so update the round step first
	cs.updateRoundStep(round, cstypes.RoundStepNewRound)
	cs.Logger.Info("The start time of this round:", "round", round)
	//测量view-change的代价

	cs.Validators = validators
	addr := cs.Validators.GetProposer().Address
	idx, _ := cs.Validators.GetByAddress(addr)
	idex1, _ := cs.Validators.GetByAddress(cs.privValidatorPubKey.Address())
	cs.Logger.Info("Proposer's Info", "当前节点的索引", idex1, "当前HR所对应的proposer", idx, "height", cs.Height, "round", cs.Round)
	if round == 0 {
		// 每个新的轮次，都会重置提案之类
		// We've already reset these upon new height,
		// and meanwhile we might have received a proposal
		// for round 0.
		cs.proposer = int(idx) //我自己添加的
	} else {
		logger.Debug("resetting proposal info")
		cs.Proposal = nil
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = nil
	}

	cs.Votes.SetRound(tmmath.SafeAddInt32(round, 1)) // also track next round (round+1) to allow round-skipping
	cs.TriggeredTimeoutPrecommit = false

	if err := cs.eventBus.PublishEventNewRound(cs.NewRoundEvent()); err != nil {
		cs.Logger.Error("failed publishing new round", "err", err)
	}

	cs.metrics.Rounds.Set(float64(round))

	// Wait for txs to be available in the mempool
	// before we enterPropose in round 0. If the last block changed the app hash,
	// we may need an empty "proof" block, and enterPropose immediately.
	waitForTxs := cs.config.WaitForTxs() && round == 0 && !cs.needProofBlock(height)
	if waitForTxs {
		if cs.config.CreateEmptyBlocksInterval > 0 {
			cs.scheduleTimeout(cs.config.CreateEmptyBlocksInterval, height, round,
				cstypes.RoundStepNewRound)
		}
	} else {
		cs.enterPropose(height, round)
	}
}

// needProofBlock returns true on the first height (so the genesis app hash is signed right away)
// and where the last block (height-1) caused the app hash to change
// 这个函数就是证明上个区块已经上链了。
func (cs *State) needProofBlock(height int64) bool {
	if height == cs.state.InitialHeight {
		return true
	}

	lastBlockMeta := cs.blockStore.LoadBlockMeta(height - 1)
	if lastBlockMeta == nil {
		panic(fmt.Sprintf("needProofBlock: last block meta for height %d not found", height-1))
	}

	return !bytes.Equal(cs.state.AppHash, lastBlockMeta.Header.AppHash)
}

// Enter (CreateEmptyBlocks): from enterNewRound(height,round)
// Enter (CreateEmptyBlocks, CreateEmptyBlocksInterval > 0 ):
//
//	after enterNewRound(height,round), after proposetimeout of CreateEmptyBlocksInterval
//
// Enter (!CreateEmptyBlocks) : after enterNewRound(height,round), once txs are in the mempool
func (cs *State) enterPropose(height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cstypes.RoundStepPropose <= cs.Step) {
		logger.Debug(
			"entering propose step with invalid args",
			"current", log.NewLazySprintf("%v/%v/%v", cs.Height, cs.Round, cs.Step),
		)
		return
	}

	logger.Info("entering propose step", "current", log.NewLazySprintf("%v/%v/%v", cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPropose:
		cs.updateRoundStep(round, cstypes.RoundStepPropose)
		cs.newStep()

		// If we have the whole proposal + POL, then goto Prevote now.
		// else, we'll enterPrevote when the rest of the proposal is received (in AddProposalBlockPart),
		// or else after timeoutPropose
		//这是自己的就能达成
		if cs.isProposalComplete() {
			cs.enterPrevote(height, cs.Round)
		}
	}()

	// If we don't get the proposal and all block parts quick enough, enterPrevote
	// 这个要怎样增加一个指数退避
	flag_sleep := int32(0)
	if cs.config.ProposeTimeoutFlag == "Exponential Backoff" {
		flag_sleep = int32(cs.view_change_number_group)
	} else {
		flag_sleep = round
	}

	cs.scheduleTimeout(cs.config.Propose(flag_sleep), height, round, cstypes.RoundStepPropose)

	if !cs.startflag && cs.Round == int32(0) {
		cs.startflag = true
		cs.updateProposeStartTime(time.Now(), int64(cs.Round))
	}

	cs.Logger.Info("this height's propose proposetimeout", "本次高度的提案超时", cs.config.Propose(flag_sleep).Milliseconds(), "height", cs.Height)

	// Nothing more to do if we're not a validator
	if cs.privValidator == nil {
		logger.Debug("node is not a validator")
		return
	}

	logger.Debug("node is a validator")

	if cs.privValidatorPubKey == nil {
		// If this node is a validator & proposer in the current round, it will
		// miss the opportunity to create a block.
		logger.Error("propose step; empty priv validator public key", "err", errPubKeyIsNotSet)
		return
	}

	address := cs.privValidatorPubKey.Address()

	// if not a validator, we're done
	if !cs.Validators.HasAddress(address) {
		logger.Debug("node is not a validator", "addr", address, "vals", cs.Validators)
		return
	}

	if cs.isProposer(address) {
		logger.Debug("propose step; our turn to propose", "proposer", address)
		//time.Sleep(4 * time.Second) //每个节点睡5s

		//原始的作恶的方式
		//epoch_index := int(cs.Height) / int(cs.config.Epoch)
		//if cs.is_byz_node && epoch_index > 0 {
		//	dur := cs.config.TimeoutPropose.Milliseconds() - cs.config.CurrentProposeTime.Milliseconds()
		//	fmt.Println("node", cs.id_int, "当前的区块高度： ", cs.Height, "当前的提案超时：", cs.config.TimeoutPropose.Milliseconds(), "实际的提案时间: ", cs.config.CurrentProposeTime.Milliseconds(), "想要延误的时间：", dur)
		//	time.Sleep(time.Duration(dur) * time.Millisecond)
		//}
		//if cs.is_byz_node && epoch_index == 0 {
		//	dur := cs.config.TimeoutPropose.Milliseconds() - int64(1000)
		//	fmt.Println("node", cs.id_int, "当前的区块高度： ", cs.Height, "当前的提案超时：", cs.config.TimeoutPropose.Milliseconds(), "实际的提案时间: ", cs.config.CurrentProposeTime.Milliseconds(), "想要延误的时间：", dur)
		//	time.Sleep(time.Duration(dur) * time.Millisecond)
		//}

		if cs.is_byz_node {
			dur := cs.delay_time
			cs.Logger.Info("this is a byz node", "sleep_time", cs.delay_time)
			time.Sleep(time.Duration(dur) * time.Millisecond)
		}
		if cs.is_crash_node {
			cs.Logger.Info("this is a crash node sleeping")
			time.Sleep(time.Duration(10) * time.Second)
		}
		proposal_start_time := time.Now()
		cs.decideProposal(height, round)
		proposal_end_time := time.Now()
		cs.Logger.Info("proposal time", "制作提案的间隔：", proposal_end_time.Sub(proposal_start_time).Seconds(), "height=", cs.Height)
	} else {
		logger.Debug("propose step; not our turn to propose", "proposer", cs.Validators.GetProposer().Address)
	}
	//cs.Logger.Info("propose step complete")
}

func (cs *State) isProposer(address []byte) bool {
	return bytes.Equal(cs.Validators.GetProposer().Address, address)
}

//func (cs *State) isByzNode() bool {
//	flag := false
//	address := cs.privValidatorPubKey.Address()      //当前的节点的地址
//	id_int, _ := cs.Validators.GetByAddress(address) //当前节点在validatorset中的索引
//	cs.id_int = int(id_int)
//	//下面是判断当前节点是不是拜占庭节点
//	for _, node_id := range cs.config.ByzantineNodeList {
//		if node_id == cs.id_int {
//			//cs.is_byz_node = true
//			flag = true
//			cs.Logger.Info("The node is a byz node", "the node's index in validatorset", id_int)
//			break
//		}
//	}
//	return flag
//}

func (cs *State) ReadDeliverTime(height int64) int64 {
	var deliver_time int64
	if deliverTime, exists := cs.deliver_block_time_map[strconv.Itoa(int(height))]; exists {
		deliver_time = deliverTime
		cs.Logger.Info("the deliver block time", "height", height, "deliver_time", deliverTime)
	} else {
		cs.Logger.Info("height not found", "height", height)
	}
	return deliver_time
}

// 在提案阶段，如果当前是存在一个有效块的，则继续将这个区块进行提案。如果没有相应的有效区块则新提案区块
func (cs *State) defaultDecideProposal(height int64, round int32) {
	create_time := cs.ReadDeliverTime(height)
	cs.Logger.Info("the deliver block time2", "height", height, "deliver_time", create_time)
	var block *types.Block
	var blockParts *types.PartSet

	// Decide on block   //TODO 怎样的是ValidBlock 这个需要解释一下 是拥有POL的区块吗？
	if cs.ValidBlock != nil {
		// If there is valid block, choose that.
		block, blockParts = cs.ValidBlock, cs.ValidBlockParts
	} else {
		// Create a new proposal block from state/txs from the mempool.x

		block, blockParts = cs.createProposalBlock()

		if block == nil {
			return
		}
	}

	// Flush the WAL. Otherwise, we may not recompute the same proposal to sign,
	// and the privValidator will refuse to sign anything.
	startwtime := time.Now()
	if err := cs.wal.FlushAndSync(); err != nil {
		cs.Logger.Error("failed flushing WAL to disk")
	}
	endwtime := time.Now()
	cs.Logger.Info("wal flush", "cost time", endwtime.Sub(startwtime).Seconds(), "height:= ", cs.Height)

	// Make proposal
	propBlockID := types.BlockID{Hash: block.Hash(), PartSetHeader: blockParts.Header()}
	proposal := types.NewProposal(height, round, cs.ValidRound, propBlockID)
	cs.Logger.Info("complete new proposal")
	p := proposal.ToProto()
	cs.Logger.Info("complete new proposal's proto", "提案大小", p.Size(), "chainID", cs.state.ChainID)
	startTime := time.Now()
	startsignTime := time.Now()
	err := cs.privValidator.SignProposal(cs.state.ChainID, p)
	endsignTime := time.Now()
	cs.Logger.Info("sign time", "提案签名的耗时", endsignTime.Sub(startsignTime).Seconds(), "height", cs.Height)
	if err == nil {
		proposal.Signature = p.Signature

		// send.sh proposal and block parts on internal msg queue
		starttest1time := time.Now()
		cs.sendInternalMessage(msgInfo{&ProposalMessage{proposal}, ""})
		endtest1time := time.Now()
		cs.Logger.Info("send proposal", "单纯发送提案的时间", endtest1time.Sub(starttest1time).Seconds(), "height", cs.Height)

		for i := 0; i < int(blockParts.Total()); i++ {
			cs.Logger.Debug("send log trace", "当前区块总共包含快数", blockParts.Total(), "当前发送的块是", i, "当前队列长度", len(cs.internalMsgQueue), "默认队列长度", msgQueueSize)
			part := blockParts.GetPart(i)

			starttest2time := time.Now()
			cs.sendInternalMessage(msgInfo{&BlockPartMessage{cs.Height, cs.Round, part}, ""})
			endtest2time := time.Now()
			cs.Logger.Debug("send block time", "height"+strconv.Itoa(i)+"sendtime", endtest2time.Sub(starttest2time).Seconds())

		}

		cs.Logger.Debug("signed proposal", "height", height, "round", round, "proposal", proposal)
	} else if !cs.replayMode {
		cs.Logger.Error("propose step; failed signing proposal", "height", height, "round", round, "err", err)
	}
	endTime := time.Now()
	cs.Logger.Info("send peoposal and block", "提案和区块的发送时间", endTime.Sub(startTime).Seconds(), "height=", cs.Height)

}

// Returns true if the proposal block is complete &&
// (if POLRound was proposed, we have +2/3 prevotes from there).
// 这应该是收到整个区块的时间提案，这个函数的含义：

func (cs *State) isProposalComplete() bool {
	if cs.Proposal == nil || cs.ProposalBlock == nil {
		return false
	}
	// we have the proposal. if there's a POLRound,
	// make sure we have the prevotes from it too
	if cs.Proposal.POLRound < 0 {
		return true
	}
	//下面这个情况是存在POL的情况，如果我们没有相应的prevote集合，则当前的proposer是假的，或者我们还没有收到相应的投票。
	// if this is false the proposer is lying or we haven't received the POL yet
	return cs.Votes.Prevotes(cs.Proposal.POLRound).HasTwoThirdsMajority()
}

// Create the next block to propose and return it. Returns nil block upon error.
//
// We really only need to return the parts, but the block is returned for
// convenience so we can log the proposal block.
//
// NOTE: keep it side-effect free for clarity.
// CONTRACT: cs.privValidator is not nil.
func (cs *State) createProposalBlock() (block *types.Block, blockParts *types.PartSet) {
	if cs.privValidator == nil {
		panic("entered createProposalBlock with privValidator being nil")
	}

	var commit *types.Commit
	switch {
	case cs.Height == cs.state.InitialHeight:
		// We're creating a proposal for the first block.
		// The commit is empty, but not nil.
		commit = types.NewCommit(0, 0, types.BlockID{}, nil)

	case cs.LastCommit.HasTwoThirdsMajority():
		// Make the commit from LastCommit
		commit = cs.LastCommit.MakeCommit()

	default: // This shouldn't happen.
		cs.Logger.Error("propose step; cannot propose anything without commit for the previous block")
		return
	}

	if cs.privValidatorPubKey == nil {
		// If this node is a validator & proposer in the current round, it will
		// miss the opportunity to create a block.
		cs.Logger.Error("propose step; empty priv validator public key", "err", errPubKeyIsNotSet)
		return
	}

	proposerAddr := cs.privValidatorPubKey.Address()
	cs.Logger.Info("the write delay time to block", "delay_time", cs.delay_time)
	if !cs.is_byz_node {
		cs.delay_time = 0.0
	}
	return cs.blockExec.CreateProposalBlock(cs.Height, cs.delay_time, float64(cs.ReadDeliverTime(cs.Height)), cs.state, commit, proposerAddr)
}

// Enter: `timeoutPropose` after entering Propose.
// Enter: proposal block and POL is ready.
// Prevote for LockedBlock if we're locked, or ProposalBlock if valid.
// Otherwise vote nil.  //是对区块进行投票，而不是提案
func (cs *State) enterPrevote(height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cstypes.RoundStepPrevote <= cs.Step) {
		logger.Debug(
			"entering prevote step with invalid args",
			"current", log.NewLazySprintf("%v/%v/%v", cs.Height, cs.Round, cs.Step),
		)
		return
	}

	defer func() {
		// Done enterPrevote:
		cs.updateRoundStep(round, cstypes.RoundStepPrevote)
		cs.newStep()
	}()

	logger.Info("entering prevote step", "current", log.NewLazySprintf("%v/%v/%v", cs.Height, cs.Round, cs.Step))

	// Sign and broadcast vote as necessary
	cs.doPrevote(height, round)

	// Once `addVote` hits any +2/3 prevotes, we will go to PrevoteWait
	// (so we have more time to try and collect +2/3 prevotes for a single block)
}

func (cs *State) defaultDoPrevote(height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)

	// If a block is locked, prevote that.
	if cs.LockedBlock != nil {
		logger.Debug("prevote step; already locked on a block; prevoting locked block")
		cs.signAddVote(tmproto.PrevoteType, cs.LockedBlock.Hash(), cs.LockedBlockParts.Header())
		return
	}

	// If ProposalBlock is nil, prevote nil.
	if cs.ProposalBlock == nil { //是对整个区块的延迟.
		logger.Debug("prevote step: ProposalBlock is nil")
		cs.signAddVote(tmproto.PrevoteType, nil, types.PartSetHeader{})
		return
	}

	// Validate proposal block
	err := cs.blockExec.ValidateBlock(cs.state, cs.ProposalBlock)
	if err != nil {
		// ProposalBlock is invalid, prevote nil.
		logger.Error("prevote step: ProposalBlock is invalid", "err", err)
		cs.signAddVote(tmproto.PrevoteType, nil, types.PartSetHeader{})
		return
	}

	// Prevote cs.ProposalBlock
	// NOTE: the proposal signature is validated when it is received,
	// and the proposal block parts are validated as they are received (against the merkle hash in the proposal)
	logger.Debug("prevote step: ProposalBlock is valid")
	cs.signAddVote(tmproto.PrevoteType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
}

// Enter: any +2/3 prevotes at next round.
func (cs *State) enterPrevoteWait(height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)
	prevotes := cs.Votes.Prevotes(round) //拿到当前轮次的票
	if prevotes.HasAll() {
		cs.Logger.Info("the prevote hasall()")
	}
	if prevotes.HasAll() && !prevotes.HasTwoThirdsMajority() { //TODO 消除碰撞的影响
		cs.Logger.Info("the prevote hasall and is not has twothirdmaj")
		//cs.scheduleTimeout(0*time.Millisecond, height, round, cstypes.RoundStepPrevoteWait)
		//TODO 验证正确性
		cs.enterPrecommit(height, round)
	}

	if cs.Height != height || round < cs.Round || (cs.Round == round && cstypes.RoundStepPrevoteWait <= cs.Step) {
		logger.Debug(
			"entering prevote wait step with invalid args",
			"current", log.NewLazySprintf("%v/%v/%v", cs.Height, cs.Round, cs.Step),
		)
		return
	}

	if !cs.Votes.Prevotes(round).HasTwoThirdsAny() {
		panic(fmt.Sprintf(
			"entering prevote wait step (%v/%v), but prevotes does not have any +2/3 votes",
			height, round,
		))
	}

	logger.Info("entering prevote wait step", "current", log.NewLazySprintf("%v/%v/%v", cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrevoteWait:
		cs.updateRoundStep(round, cstypes.RoundStepPrevoteWait)
		cs.newStep()
	}()

	// Wait for some more prevotes; enterPrecommit
	cs.scheduleTimeout(cs.config.Prevote(round), height, round, cstypes.RoundStepPrevoteWait)
}

// Enter: `timeoutPrevote` after any +2/3 prevotes.
// Enter: `timeoutPrecommit` after any +2/3 precommits.
// Enter: +2/3 precomits for block or nil.
// Lock & precommit the ProposalBlock if we have enough prevotes for it (a POL in this round)
// else, unlock an existing lock and precommit nil if +2/3 of prevotes were nil,
// else, precommit nil otherwise.
func (cs *State) enterPrecommit(height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cstypes.RoundStepPrecommit <= cs.Step) {
		logger.Debug(
			"entering precommit step with invalid args",
			"current", log.NewLazySprintf("%v/%v/%v", cs.Height, cs.Round, cs.Step),
		)
		return
	}

	logger.Info("entering precommit step", "current", log.NewLazySprintf("%v/%v/%v", cs.Height, cs.Round, cs.Step))
	logger.Info("the starting time of precommit ", "round", round)
	if cs.cost.CostList[len(cs.cost.CostList)-1].CrashFlag {
		CrashNum1 := len(cs.cost.CostList[len(cs.cost.CostList)-1].ALLPrevoteRound)
		CrashNum2 := len(cs.cost.CostList[len(cs.cost.CostList)-1].PreCommitRound)
		if CrashNum1 == (CrashNum2 + 1) {
			if cs.cost.CostList[len(cs.cost.CostList)-1].ALLPrevoteRound[CrashNum1-1] == round {
				cs.cost.CostList[len(cs.cost.CostList)-1].EnterPrecommitTime = append(cs.cost.CostList[len(cs.cost.CostList)-1].EnterPrecommitTime, time.Now())
				cs.cost.CostList[len(cs.cost.CostList)-1].PreCommitRound = append(cs.cost.CostList[len(cs.cost.CostList)-1].PreCommitRound, round)
			}
		}
	}

	defer func() {
		// Done enterPrecommit:
		cs.updateRoundStep(round, cstypes.RoundStepPrecommit)
		cs.newStep()
	}()

	//其实将vote轮次 分为两种情况 有没有pol
	// check for a polka
	blockID, ok := cs.Votes.Prevotes(round).TwoThirdsMajority()

	// If we don't have a polka, we must precommit nil.
	// 如果当前的轮次没有pol，但是有可能其他轮次已经锁住了
	if !ok {
		if cs.LockedBlock != nil {
			logger.Debug("precommit step; no +2/3 prevotes during enterPrecommit while we are locked; precommitting nil")
		} else {
			logger.Debug("precommit step; no +2/3 prevotes during enterPrecommit; precommitting nil")
		}

		cs.signAddVote(tmproto.PrecommitType, nil, types.PartSetHeader{})
		return
	}

	// At this point +2/3 prevoted for a particular block or nil.
	if err := cs.eventBus.PublishEventPolka(cs.RoundStateEvent()); err != nil {
		logger.Error("failed publishing polka", "err", err)
	}

	// the latest POLRound should be this round.
	polRound, _ := cs.Votes.POLInfo()
	if polRound < round {
		panic(fmt.Sprintf("this POLRound should be %v but got %v", round, polRound))
	}

	// +2/3 prevoted nil. Unlock and precommit nil.
	if len(blockID.Hash) == 0 {
		if cs.LockedBlock == nil {
			logger.Debug("precommit step; +2/3 prevoted for nil")
		} else {
			logger.Debug("precommit step; +2/3 prevoted for nil; unlocking")
			cs.LockedRound = -1
			cs.LockedBlock = nil
			cs.LockedBlockParts = nil

			if err := cs.eventBus.PublishEventUnlock(cs.RoundStateEvent()); err != nil {
				logger.Error("failed publishing event unlock", "err", err)
			}
		}

		cs.signAddVote(tmproto.PrecommitType, nil, types.PartSetHeader{})
		return
	}

	// At this point, +2/3 prevoted for a particular block.

	// If we're already locked on that block, precommit it, and update the LockedRound
	if cs.LockedBlock.HashesTo(blockID.Hash) {
		logger.Debug("precommit step; +2/3 prevoted locked block; relocking")
		cs.LockedRound = round

		if err := cs.eventBus.PublishEventRelock(cs.RoundStateEvent()); err != nil {
			logger.Error("failed publishing event relock", "err", err)
		}

		cs.signAddVote(tmproto.PrecommitType, blockID.Hash, blockID.PartSetHeader)
		return
	}

	// If +2/3 prevoted for proposal block, stage and precommit it
	if cs.ProposalBlock.HashesTo(blockID.Hash) {
		logger.Debug("precommit step; +2/3 prevoted proposal block; locking", "hash", blockID.Hash)

		// Validate the block.
		if err := cs.blockExec.ValidateBlock(cs.state, cs.ProposalBlock); err != nil {
			panic(fmt.Sprintf("precommit step; +2/3 prevoted for an invalid block: %v", err))
		}

		cs.LockedRound = round
		cs.LockedBlock = cs.ProposalBlock
		cs.LockedBlockParts = cs.ProposalBlockParts

		if err := cs.eventBus.PublishEventLock(cs.RoundStateEvent()); err != nil {
			logger.Error("failed publishing event lock", "err", err)
		}

		cs.signAddVote(tmproto.PrecommitType, blockID.Hash, blockID.PartSetHeader)
		return
	}

	// There was a polka in this round for a block we don't have.
	// Fetch that block, unlock, and precommit nil.
	// The +2/3 prevotes for this round is the POL for our unlock.
	//
	logger.Debug("precommit step; +2/3 prevotes for a block we do not have; voting nil", "block_id", blockID)

	cs.LockedRound = -1
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil

	if !cs.ProposalBlockParts.HasHeader(blockID.PartSetHeader) {
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartSetHeader)
	}

	if err := cs.eventBus.PublishEventUnlock(cs.RoundStateEvent()); err != nil {
		logger.Error("failed publishing event unlock", "err", err)
	}

	cs.signAddVote(tmproto.PrecommitType, nil, types.PartSetHeader{})
}

// Enter: any +2/3 precommits for next round. TODO这个应该弄清楚commit的情况。
func (cs *State) enterPrecommitWait(height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)
	//这其实不需要管制，其实要弄清楚
	//TriggeredTimeoutPrecommit TODO：是否已经触发了timeoutPrecommit  //这个应该是同一个round不能触发两次commit
	if cs.Height != height || round < cs.Round || (cs.Round == round && cs.TriggeredTimeoutPrecommit) {
		logger.Debug(
			"entering precommit wait step with invalid args",
			"triggered_timeout", cs.TriggeredTimeoutPrecommit,
			"current", log.NewLazySprintf("%v/%v", cs.Height, cs.Round),
		)
		return
	}

	if !cs.Votes.Precommits(round).HasTwoThirdsAny() {
		panic(fmt.Sprintf(
			"entering precommit wait step (%v/%v), but precommits does not have any +2/3 votes",
			height, round,
		))
	}

	logger.Info("entering precommit wait step", "current", log.NewLazySprintf("%v/%v/%v", cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrecommitWait:
		cs.TriggeredTimeoutPrecommit = true
		cs.newStep()
	}()

	// wait for some more precommits; enterNewRound
	precommits := cs.Votes.Precommits(cs.Round)

	blockID, ok := precommits.TwoThirdsMajority()
	var precommit_timeout time.Duration
	if ok && len(blockID.Hash) == 0 {
		fmt.Println("dfdfdfdfddfdddddddddddddddddddddddddddddddddddddddddddddd")
		//TODO
		precommit_timeout = 0 * time.Millisecond
	} else {
		precommit_timeout = cs.config.Precommit(round)
	}
	//cs.scheduleTimeout(cs.config.Precommit(round), height, round, cstypes.RoundStepPrecommitWait)
	cs.scheduleTimeout(precommit_timeout, height, round, cstypes.RoundStepPrecommitWait)
}

// Enter: +2/3 precommits for block
func (cs *State) enterCommit(height int64, commitRound int32) {
	logger := cs.Logger.With("height", height, "commit_round", commitRound)

	if cs.Height != height || cstypes.RoundStepCommit <= cs.Step {
		logger.Debug(
			"entering commit step with invalid args",
			"current", log.NewLazySprintf("%v/%v/%v", cs.Height, cs.Round, cs.Step),
		)
		return
	}

	logger.Info("entering commit step", "current", log.NewLazySprintf("%v/%v/%v", cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterCommit:
		// keep cs.Round the same, commitRound points to the right Precommits set.
		cs.updateRoundStep(cs.Round, cstypes.RoundStepCommit)
		cs.CommitRound = commitRound
		cs.CommitTime = tmtime.Now()

		cs.newStep()

		// Maybe finalize immediately.
		fmt.Println("提交前的时间", cs.CommitTime.Sub(cs.blt.start_time).Seconds())
		cs.tryFinalizeCommit(height)

	}()

	blockID, ok := cs.Votes.Precommits(commitRound).TwoThirdsMajority()
	if !ok {
		panic("RunActionCommit() expects +2/3 precommits")
	}

	// The Locked* fields no longer matter.
	// Move them over to ProposalBlock if they match the commit hash,
	// otherwise they'll be cleared in updateToState.
	if cs.LockedBlock.HashesTo(blockID.Hash) {
		logger.Debug("commit is for a locked block; set ProposalBlock=LockedBlock", "block_hash", blockID.Hash)
		cs.ProposalBlock = cs.LockedBlock
		cs.ProposalBlockParts = cs.LockedBlockParts
	}

	// If we don't have the block being committed, set up to get it.
	if !cs.ProposalBlock.HashesTo(blockID.Hash) {
		if !cs.ProposalBlockParts.HasHeader(blockID.PartSetHeader) {
			logger.Info(
				"commit is for a block we do not know about; set ProposalBlock=nil",
				"proposal", log.NewLazyBlockHash(cs.ProposalBlock),
				"commit", blockID.Hash,
			)

			// We're getting the wrong block.
			// Set up ProposalBlockParts and keep waiting.
			cs.ProposalBlock = nil
			cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartSetHeader)

			if err := cs.eventBus.PublishEventValidBlock(cs.RoundStateEvent()); err != nil {
				logger.Error("failed publishing valid block", "err", err)
			}

			cs.evsw.FireEvent(types.EventValidBlock, &cs.RoundState)
		}
	}
}

// If we have the block AND +2/3 commits for it, finalize.
func (cs *State) tryFinalizeCommit(height int64) {
	logger := cs.Logger.With("height", height)

	if cs.Height != height {
		panic(fmt.Sprintf("tryFinalizeCommit() cs.Height: %v vs height: %v", cs.Height, height))
	}

	blockID, ok := cs.Votes.Precommits(cs.CommitRound).TwoThirdsMajority()
	if !ok || len(blockID.Hash) == 0 {
		logger.Error("failed attempt to finalize commit; there was no +2/3 majority or +2/3 was for nil")
		return
	}

	if !cs.ProposalBlock.HashesTo(blockID.Hash) {
		// TODO: this happens every time if we're not a validator (ugly logs)
		// TODO: ^^ wait, why does it matter that we're a validator?
		logger.Debug(
			"failed attempt to finalize commit; we do not have the commit block",
			"proposal_block", log.NewLazyBlockHash(cs.ProposalBlock),
			"commit_block", blockID.Hash,
		)
		return
	}

	cs.finalizeCommit(height)
}

// Increment height and goto cstypes.RoundStepNewHeight
func (cs *State) finalizeCommit(height int64) {
	logger := cs.Logger.With("height", height)

	if cs.Height != height || cs.Step != cstypes.RoundStepCommit {
		logger.Debug(
			"entering finalize commit step",
			"current", log.NewLazySprintf("%v/%v/%v", cs.Height, cs.Round, cs.Step),
		)
		return
	}

	cs.calculatePrevoteMessageDelayMetrics()

	blockID, ok := cs.Votes.Precommits(cs.CommitRound).TwoThirdsMajority()
	block, blockParts := cs.ProposalBlock, cs.ProposalBlockParts

	if !ok {
		panic("cannot finalize commit; commit does not have 2/3 majority")
	}
	if !blockParts.HasHeader(blockID.PartSetHeader) {
		panic("expected ProposalBlockParts header to be commit header")
	}
	if !block.HashesTo(blockID.Hash) {
		panic("cannot finalize commit; proposal block does not hash to commit hash")
	}

	if err := cs.blockExec.ValidateBlock(cs.state, block); err != nil {
		panic(fmt.Errorf("+2/3 committed an invalid block: %w", err))
	}

	logger.Info(
		"finalizing commit of block",
		"hash", log.NewLazyBlockHash(block),
		"root", block.AppHash,
		"num_txs", len(block.Txs),
	)
	logger.Debug("committed block", "block", log.NewLazySprintf("%v", block))

	fail.Fail() // XXX

	// Save to blockStore.
	if cs.blockStore.Height() < block.Height {
		// NOTE: the seenCommit is local justification to commit this block,
		// but may differ from the LastCommit included in the next block
		precommits := cs.Votes.Precommits(cs.CommitRound)
		seenCommit := precommits.MakeCommit()
		cs.blockStore.SaveBlock(block, blockParts, seenCommit)
	} else {
		// Happens during replay if we already saved the block but didn't commit
		logger.Debug("calling finalizeCommit on already stored block", "height", block.Height)
	}

	fail.Fail() // XXX

	// Write EndHeightMessage{} for this height, implying that the blockstore
	// has saved the block.
	//
	// If we crash before writing this EndHeightMessage{}, we will recover by
	// running ApplyBlock during the ABCI handshake when we restart.  If we
	// didn't save the block to the blockstore before writing
	// EndHeightMessage{}, we'd have to change WAL replay -- currently it
	// complains about replaying for heights where an #ENDHEIGHT entry already
	// exists.
	//
	// Either way, the State should not be resumed until we
	// successfully call ApplyBlock (ie. later here, or in Handshake after
	// restart).
	endMsg := EndHeightMessage{height}
	if err := cs.wal.WriteSync(endMsg); err != nil { // NOTE: fsync
		panic(fmt.Sprintf(
			"failed to write %v msg to consensus WAL due to %v; check your file system and restart the node",
			endMsg, err,
		))
	}

	fail.Fail() // XXX

	// Create a copy of the state for staging and an event cache for txs.
	stateCopy := cs.state.Copy()

	// Execute and commit the block, update and save the state, and update the mempool.
	// NOTE The block.AppHash wont reflect these txs until the next block.
	var (
		err          error
		retainHeight int64
	)
	startTime := time.Now()
	address := cs.privValidatorPubKey.Address()
	//更新bct
	test_flag := false
	if cs.id_int == (cs.proposer+int(0))%cs.Validators.Size() {
		test_flag = true
	}
	if cs.id_int != (cs.proposer+int(0))%cs.Validators.Size() {
		b_c_t := cs.pt.GetBCT(test_flag, cs.config.Mode, cs.block_bct_hash, strconv.Itoa(0))
		test_test_flag := false
		if bytetostring(cs.ProposalBlockParts.Hash()) == cs.block_bct_hash {
			test_test_flag = true
		}
		cs.Logger.Info("The info of bct", "ProposalBlockParts.Hash()", bytetostring(cs.ProposalBlockParts.Hash()), "block_bct_hash", cs.block_bct_hash, "round", cs.Round, "flag", test_test_flag)
		logger.Info("bct_time", "bct", b_c_t)
		if bytetostring(cs.ProposalBlockParts.Hash()) == cs.block_bct_hash && cs.Round != int32(0) {
			for i := 0; i < int(cs.Round); i++ {
				timeout_cost := float64(cs.config.Propose(int32(i)).Seconds())
				logger.Info("bct_time", "bct", b_c_t, "timeout_cost", timeout_cost, "round", i)
				b_c_t = b_c_t - timeout_cost
			}
		}
		logger.Info("bct_time", "bct", b_c_t)
		logger.Info("fire bct eventcommit", "height", cs.Height, "PDT", 0.0, "BCT", b_c_t, "crash_flash", cs.is_crash_node)
		write_pdt(cs.config.BctFilePath, cs.Height, 0.0, b_c_t, cs.is_crash_node)
	}

	//更新pdt
	if cs.id_int != (cs.proposer+int(cs.Round))%cs.Validators.Size() {
		p_d_t := cs.pt.GetPDT(cs.isProposer(address), cs.config.Mode, bytetostring(cs.ProposalBlockParts.Hash()), strconv.Itoa(0))
		logger.Info("fire pdt eventcommit", "height", cs.Height, "PDT", p_d_t, "BCT", 0.0, "crash_flash", cs.is_crash_node)
		pdt_info := types.MakePDTInfo(cs.Height, p_d_t, 0.0)
		//cs.evsw.FireEvent(types.EventCommit, pdt_info)
		ptd_height, ptd_time, bct_time := pdt_info.GetValue()
		write_pdt(cs.config.PdtFilePath, ptd_height, ptd_time, bct_time, cs.is_crash_node)
	}

	stateCopy, retainHeight, err = cs.blockExec.ApplyBlock(
		stateCopy,
		types.BlockID{
			Hash:          block.Hash(),
			PartSetHeader: blockParts.Header(),
		},
		block,
	)
	time.Sleep(2 * time.Second) //TODO 其实这块也是为了很好的适应的网络的变化，让节点在区块的开始发送条件一致。
	endTime := time.Now()

	//test_node_id := os.Getenv("ID")
	//cs.Logger.Info("the sleep time of commit phase is: 5s")
	//if test_node_id == "0" {
	//	cs.Logger.Info("before test input")
	//	time.Sleep(50 * time.Second)
	//	cs.Logger.Info("afer test input")
	//}
	fmt.Println("当前区块处理时间：", endTime.Sub(startTime).Seconds())
	if err != nil {
		logger.Error("failed to apply block", "err", err)
		return
	}

	fail.Fail() // XXX

	// Prune old heights, if requested by ABCI app.
	if retainHeight > 0 {
		pruned, err := cs.pruneBlocks(retainHeight)
		if err != nil {
			logger.Error("failed to prune blocks", "retain_height", retainHeight, "err", err)
		} else {
			logger.Debug("pruned blocks", "pruned", pruned, "retain_height", retainHeight)
		}
	}
	fmt.Println("当前区块高度是：", cs.Height, "对应的提交轮次：", cs.Round)
	// must be called before we update state
	cs.recordMetrics(height, block)
	cs.LastBlockCommitTime = time.Now()
	//更新延迟的时间
	cs.updateLatencyEndTime(cs.LastBlockCommitTime, int64(cs.Round))

	//在zt的统计方式之下, 将propose_time时间根据data_index放到固定长度列表中相应的位置,  这其实对block_based的方法基本是没有什么影响的
	//因为，在block_based的方法下是使用的队列实现的
	new_Mi_index := int64(cs.mi_number) * cs.config.SmallMI    //当前MI对应的第一个区块的高度
	data_index := (cs.Height-new_Mi_index)%cs.config.Epoch - 1 //TODO 这需要修改  放到epoch中的索引

	//下面这句话应该是对于最后一个元素的存放位置
	if data_index == -1 {
		data_index = cs.config.Epoch - 1
	}

	//更新bct
	if cs.id_int == ((cs.proposer + int(0)) % cs.Validators.Size()) {
		if cs.config.Mode {
			pdt_value := 0.0
			bct_value := 0.0
			num := 0
			currrent_id, _ := strconv.Atoi(cs.config.ID)
			//当前id是偶数函数奇数
			currrent_flag := true
			if currrent_id%2 != 0 {
				currrent_flag = false
			}
			if currrent_flag {
				for id := 0; id < 4; id++ {
					ID := strconv.Itoa(id * 2)
					if ID != cs.config.ID {
						read_height, read_time, read_time2, read_crash := read_pdt("../config_path/node" + strconv.Itoa(id*2) + "/bct.txt")
						cs.Logger.Info("the bct of per node", "node.height", read_height, "cs.height", cs.Height, "bct_time", read_time2, "node_id", ID)
						if read_height == cs.Height && !read_crash && read_time == float64(0.0) && read_time2 != float64(0) {
							pdt_value = read_time + pdt_value
							bct_value = read_time2 + bct_value
							num = num + 1
						}
					}
				}
			} else {
				for id := 0; id < 4; id++ {
					ID := strconv.Itoa(id*2 + 1)
					if ID != cs.config.ID {
						read_height, read_time, read_time2, read_crash := read_pdt("../config_path/node" + strconv.Itoa(id*2+1) + "/bct.txt")
						cs.Logger.Info("the bct of per node", "node.height", read_height, "cs.height", cs.Height, "bct_time", read_time2, "node_id", ID)
						if read_height == cs.Height && !read_crash && read_time == float64(0.0) && read_time2 != float64(0) {
							pdt_value = read_time + pdt_value
							bct_value = read_time2 + bct_value
							num = num + 1
						}
					}
				}
			}
			if cs.is_crash_node {
				pdt_value = 0.0
				bct_value = 0.0
			} else {
				pdt_value = pdt_value / float64(num)
				bct_value = bct_value / float64(num)
			}

			cs.Logger.Info("the bct of the propose node", "cs.height", cs.Height, "pdt_time", pdt_value, "bct_value", bct_value)

			cs.pt.SetBCT(bct_value)

			cs.Logger.Info("the propose node true bct", "cs.height", cs.Height, "pdt_time", cs.pt.RPT(cs.isProposer(address), cs.config.Mode, strconv.Itoa(int(0))))

		}

	}

	//将统计信息放到列表中,采样的时间,延迟，提交轮次等记录下来。
	//其实在下一个
	if cs.id_int == ((cs.proposer + int(cs.Round)) % cs.Validators.Size()) {
		if cs.config.Mode {
			pdt_value := 0.0
			bct_value := 0.0
			num := 0
			currrent_id, _ := strconv.Atoi(cs.config.ID)
			currrent_flag := true
			if currrent_id%2 != 0 {
				currrent_flag = false
			}
			if currrent_flag {
				for id := 0; id < 4; id++ {
					ID := strconv.Itoa(id * 2)
					if ID != cs.config.ID {
						read_height, read_time, read_time2, read_crash := read_pdt("../config_path/node" + strconv.Itoa(id*2) + "/pdt.txt")
						cs.Logger.Info("the pdt of per node", "node.height", read_height, "cs.height", cs.Height, "pdt_time", read_time, "node_id", ID)
						if read_height == cs.Height && !read_crash && read_time != float64(0) && read_time2 == float64(0) {
							pdt_value = read_time + pdt_value
							bct_value = read_time2 + bct_value
							num = num + 1
						}
					}
				}
			} else {
				for id := 0; id < 4; id++ {
					ID := strconv.Itoa(id*2 + 1)
					if ID != cs.config.ID {
						read_height, read_time, read_time2, read_crash := read_pdt("../config_path/node" + strconv.Itoa(id*2+1) + "/pdt.txt")
						cs.Logger.Info("the pdt of per node", "node.height", read_height, "cs.height", cs.Height, "pdt_time", read_time, "node_id", ID)
						if read_height == cs.Height && !read_crash && read_time != float64(0) && read_time2 == float64(0) {
							pdt_value = read_time + pdt_value
							bct_value = read_time2 + bct_value
							num = num + 1
						}
					}
				}
			}
			if cs.is_crash_node {
				pdt_value = 0.0
				bct_value = 0.0
			} else {
				pdt_value = pdt_value / float64(num)
				bct_value = bct_value / float64(num)
			}
			//pdt_value = pdt_value / float64(num)
			//bct_value = bct_value / float64(num)
			cs.Logger.Info("the pdt of the propose node", "cs.height", cs.Height, "pdt_time", pdt_value, "bct_value", bct_value)

			cs.pt.SetPDT(pdt_value)

			cs.Logger.Info("the propose node true pdt", "cs.height", cs.Height, "pdt_time", cs.pt.RPT(cs.isProposer(address), cs.config.Mode, strconv.Itoa(int(cs.Round))))

		}

		//for _, value := range cs.validatorPDT {
		//	pdt_value = pdt_value + value
		//}
		//pdt_value = pdt_value / float64(len(cs.validatorPDT))
		//cs.Logger.Info("the update pdt_value", "pdt_value", pdt_value, "Len", len(cs.validatorPDT), "validator_pdt", cs.validatorPDT)

	}
	cs.ptl.AddTime(cs.pt.RPT(cs.isProposer(address), cs.config.Mode, strconv.Itoa(int(cs.Round))), int(data_index), cs.proposer)
	cs.ltl.AddTime(cs.blt.Latency(), int(data_index), 0)
	cs.crl.AddRound(float64(cs.Round), int(data_index))

	//TODO proposerListofMI 其实这里记录了两遍
	cs.proposerListofMI = append(cs.proposerListofMI, cs.proposer)                                                          //主节点的信息
	cs.bctListofMI = append(cs.bctListofMI, cs.pt.RPT(cs.isProposer(address), cs.config.Mode, strconv.Itoa(int(cs.Round)))) //bct的值的大小

	//计算当前区块的的BCT和当前区块的初始Timeout
	cs.cost.CostList[len(cs.cost.CostList)-1].BCT = int64(cs.pt.RPT(cs.isProposer(address), cs.config.Mode, strconv.Itoa(int(cs.Round))) * 1000)
	//这个也是用的是该高度开始时的timeout
	cs.cost.CostList[len(cs.cost.CostList)-1].ComputeCost(cs.TimeoutOFHeight)

	//修改区块的TPS，TODO也可以放到后台进行存储
	cs.tps.TxCommitNum = cs.tps.TxCommitNum + int64(len(block.Txs))
	//总体的浪费的时间
	cs.tps.Discretionary = cs.tps.Discretionary + float64(cs.cost.CostList[len(cs.cost.CostList)-1].Discretionary/1000)

	//将当前区块的预测结果放到预测列表中，就是区块开始的timeout，其中TimeoutOFHeight属于变量，将cost的也同时加入队列
	//TODO 将Height 作恶空间也加入到统计数据中，这个其实就是摘取了节点上的几个时间
	cs.Logger.Info("the height of blockchain", height, cs.Height, "cs.isProposer(address))*1000", cs.pt.RPT(cs.isProposer(address), cs.config.Mode, strconv.Itoa(int(cs.Round)))*1000)
	//这个属于大MI
	cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].Height = append(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].Height, cs.Height)

	//这个是不是判断当前节点是不是leader
	//help_is_leader_flag := false
	//help_address := cs.privValidatorPubKey.Address()           //当前的节点的地址
	//help_id_int, _ := cs.Validators.GetByAddress(help_address) //当前节点在 validator_set 中的索引
	//if cs.proposer == int(help_id_int) {
	//	help_is_leader_flag = true
	//}

	cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].IsLeader = append(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].IsLeader, cs.isProposer(address))
	cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].ProposerList = append(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].ProposerList, int64((cs.proposer+int(cs.Round))%cs.Validators.Size()))
	cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].IsByzNode = append(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].IsByzNode, cs.is_byz_node)

	b_c_t2 := cs.pt.GetBCT(test_flag, cs.config.Mode, cs.block_bct_hash, strconv.Itoa(0))
	if bytetostring(cs.ProposalBlockParts.Hash()) == cs.block_bct_hash && cs.Round != int32(0) {
		for i := 0; i < int(cs.Round); i++ {
			timeout_cost2 := float64(cs.config.Propose(int32(i)).Seconds())
			b_c_t2 = b_c_t2 - timeout_cost2
		}
	}
	cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].ActaulTimeoutList = append(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].ActaulTimeoutList, b_c_t2*1000)
	cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].PredictTimeoutList = append(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].PredictTimeoutList, float64(cs.TimeoutOFHeight))
	cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].ByzTimeoutList = append(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].ByzTimeoutList, float64(cs.config.CurrentProposeTime.Milliseconds()))
	cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].Round = append(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].Round, int64(cs.Round))
	cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].CostList = append(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].CostList, &pb.Cost{ViewChangeCost: cs.cost.CostList[len(cs.cost.CostList)-1].ViewChange, ThreatCost: cs.cost.CostList[len(cs.cost.CostList)-1].Threat, Discretionary: cs.cost.CostList[len(cs.cost.CostList)-1].Discretionary, Flag: cs.cost.CostList[len(cs.cost.CostList)-1].Flag})
	cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].LatencyList = append(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].LatencyList, cs.blt.Latency())
	cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].TxNumList = append(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].TxNumList, int64(len(cs.ProposalBlock.Txs)))

	//其实这块需要每个高度都要发送数据
	//当前
	if cs.Height%cs.config.SmallMI == 0 {
		//下面的参数指的是需要给后台发送哪些数据
		code := cs.pcc_service.RunSpc(cs.smallMiofMi)
		cs.ptl.AdjustParameters(int(code))
		cs.smallMiofMi = cs.smallMiofMi + 1
		cs.Logger.Info("SPC change alpha bate code", "code", code)

		cs.mi_number = cs.mi_number + 1
		//下面是改变的逻辑，原来的作恶方式
		//肯定是要使用数据投毒这些东西来替代
		//TODO 增加一个新的方法来计算新的高度的timeout，还要增加一个t_d位
		//cs.pcc_service.RunRLMethod()
		timeout, byz_timeout := cs.computeFirstTimeout(cs.config.Epoch)
		cs.config.CurrentProposeTime = time.Duration(byz_timeout) * time.Millisecond //其实就是真实的bct的意思
		cs.view_change_number_group = 0
		cs.TimeoutOFHeight = cs.config.TimeoutPropose.Milliseconds() //TODO？
		if cs.config.Update {
			//if timeout > 3000 {
			//	timeout = 3000
			//}
			cs.config.TimeoutPropose = time.Duration(timeout) * time.Millisecond
		}

		//查看当前的阶段是否执行完毕
		cs.ptl.Clear(cs.config.Epoch)
		cs.ltl.Clear(cs.config.Epoch)
		cs.crl.Clear(cs.config.Epoch)
		cs.cost.Clear(cs.config.Epoch)

		cs.proposerListofMI = cs.proposerListofMI[:0]
		cs.bctListofMI = cs.bctListofMI[:0]
	}

	if cs.Height%cs.config.MI == 0 {
		//当每个MI结束时,需要计算这个MI对应的tps
		cs.smallMiofMi = 0
		cs.tps.EndTime = time.Now()
		cs.tps.ComputeTps()
		//TODO 计算当前的tps
		cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].Tps = cs.tps.CommitTxNumPerSec
		cs.Logger.Info("the end time of the MI", "EndTime", time.Now())
		cs.Logger.Info("the tps of last mi", "tps", cs.tps.CommitTxNumPerSec)
		cs.Logger.Info("the height list of height ", "height_list", cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].Height)
		//这个是特殊情况的处理
		if len(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].Height) != int(cs.pcc_service.MI) {
			cs.pcc_service.Clear()
		} else {
			if len(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].Height) != int(cs.pcc_service.MI) {
				panic("ps requst height list error" + strconv.Itoa(len(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].Height)) + strconv.Itoa(int(cs.pcc_service.MI)))
			}

			if len(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].ActaulTimeoutList) != int(cs.pcc_service.MI) {
				panic("ps requst sampledata list error")
			}

			if len(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].PredictTimeoutList) != int(cs.pcc_service.MI) {
				panic("ps requst predict_data list error")
			}

			if len(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].ByzTimeoutList) != int(cs.pcc_service.MI) {
				panic("ps requst byzTimeout list error")
			}

			if len(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].Round) != int(cs.pcc_service.MI) {
				panic("ps requst round list error")
			}

			if len(cs.pcc_service.Req.MIData[cs.pcc_service.ExcutedNumber].CostList) != int(cs.pcc_service.MI) {
				panic("ps requst cost list error")
			}
		}

		//已经执行了一个MI
		cs.pcc_service.ExcutedNumber = cs.pcc_service.ExcutedNumber + 1

		if cs.pcc_service.SizeNumber == cs.pcc_service.ExcutedNumber {
			cs.pcc_service.RunPCC() //TODO 其实这块不会调用RunPCC()
		}
		// TODO 下面这行代码是再
		//if cs.config.EpochSizeUpdate {
		//	cs.config.Epoch = int64(cs.pcc_service.SizeList[int(cs.pcc_service.ExcutedNumber)]) //新的MI的epoch_size的大小
		//}
		//先学习再计算下个MI对应的timeout，用学习出来的新K
		if cs.config.KUpdate {
			if cs.config.UpdateLevel == "epoch" {
				peb.Multiple = float64(cs.pcc_service.SizeList[int(cs.pcc_service.ExcutedNumber)])
			}
			if cs.config.UpdateLevel == "block" {
				pbb.Multiple = float64(cs.pcc_service.SizeList[int(cs.pcc_service.ExcutedNumber)])
			}
		}

		// TODO 这个是被换掉的逻辑

		//timeout, byz_timeout := cs.computeFirstTimeout(cs.config.Epoch)
		//
		//cs.config.CurrentProposeTime = time.Duration(byz_timeout) * time.Millisecond //其实就是真实的bct的意思
		//cs.view_change_number_group = 0
		//cs.TimeoutOFHeight = cs.config.TimeoutPropose.Milliseconds()
		//if cs.config.Update {
		//	if timeout > 3000 {
		//		timeout = 3000
		//	}
		//	cs.config.TimeoutPropose = time.Duration(timeout) * time.Millisecond
		//}
		//cs.Logger.Info("PCC_Service: ", "cs.pcc_service.ExcutedNumber", cs.pcc_service.ExcutedNumber)
		//
		////cs.config.Epoch = int64(cs.pcc_service.SizeList[int(cs.pcc_service.ExcutedNumber)]) //新的MI的epoch_size的大小
		////查看当前的阶段是否执行完毕
		//cs.ptl.Clear(cs.config.Epoch)
		//cs.ltl.Clear(cs.config.Epoch)
		//cs.crl.Clear(cs.config.Epoch)
		//cs.cost.Clear(cs.config.Epoch)

		cs.Logger.Info("the tx commited number of the last MI", "commitNum", cs.tps.TxCommitNum)
		cs.tps.Clear()
		//cs.proposerListofMI = cs.proposerListofMI[:0]
		//cs.bctListofMI = cs.bctListofMI[:0]
	}

	// NewHeightStep! ，其实在这已经计算出了下一个Epoch的两个timeout timeout1:诚实的timeout、timeout2作恶的timeout
	// 这个是正常的timeout的更新
	if (cs.Height-new_Mi_index)%cs.config.Epoch == 0 { // TODO 这也许要修改一下
		if cs.Height%cs.config.SmallMI != 0 {
			cs.updateProposeTimeOut()
			cs.updateLatencyTimeOut()
			cs.updateRoundTimeOut()
		}
	}

	//每个区块完成之后都要更新一次下个高度的初始户Timeout,这个要在Epoch和MI更新之后再更新 TODO 其实可以放入cs.UpdateToState中
	cs.TimeoutOFHeight = cs.config.Propose(int32(cs.view_change_number_group)).Milliseconds()
	//TODO 在这加个配置信息

	cs.updateToState(stateCopy)

	//TODO 保证有时直接进入prvote的情况  其实也可以和update round
	cs.updateProposeStartTime(time.Now(), int64(0))

	fail.Fail() // XXX

	// Private validator might have changed it's key pair => refetch pubkey.
	if err := cs.updatePrivValidatorPubKey(); err != nil {
		logger.Error("failed to get private validator pubkey", "err", err)
	}

	// cs.StartTime is already set.
	// Schedule Round0 to start soon.
	cs.scheduleRound0(&cs.RoundState)

	// By here,
	// * cs.Height has been increment to height+1
	// * cs.Step is now cstypes.RoundStepNewHeight
	// * cs.StartTime is set to when we will start round0.
}

type PDTINFO struct {
	Height int64
	Time   float64
	Time2  float64
	crash  int64
}

func write_pdt(pdt_file string, height int64, pdt float64, bct float64, crash bool) {
	// 要写入的内容
	crash_flag := int64(0)
	if crash {
		crash_flag = int64(1)
	}
	data := PDTINFO{height, pdt, bct, crash_flag}

	// 创建或打开文件
	file, err := os.Create(pdt_file)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close() // 确保在函数结束时关闭文件

	_, err = fmt.Fprintf(file, "%d,%f,%f,%d\n", data.Height, data.Time, data.Time2, data.crash)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		return
	}

	fmt.Println("File written successfully!")
}

//func is_ture_proposer() bool {
//
//}

func read_pdt(ptd_file string) (int64, float64, float64, bool) {
	// 打开文件
	file, err := os.Open(ptd_file)
	if err != nil {
		fmt.Println("Error opening file:", err)
	}
	defer file.Close() // 确保在函数结束时关闭文件
	var height int64
	var time float64
	var time2 float64
	var crash_flag int64
	// 读取文件内容
	scanner := bufio.NewScanner(file)
	if scanner.Scan() {

		_, err1 := fmt.Sscanf(scanner.Text(), "%d,%f,%f,%d", &height, &time, &time2, &crash_flag)
		if err1 != nil {
			fmt.Println("Error reading values:", err)
		}
		fmt.Printf("Height: %d, Time: %.f seconds, Time2: %.f seconds crash: %d\n", height, time, time2, crash_flag)
	}

	// 检查扫描过程中是否有错误
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading file:", err)
	}
	crash := false
	if crash_flag == int64(1) {
		crash = true
	}
	return height, time, time2, crash
}

// 计算下一个MI中epoch的初始的timeout
func (cs *State) computeFirstTimeout(size int64) (int64, int64) {
	address := cs.privValidatorPubKey.Address()      //当前的节点的地址
	id_int, _ := cs.Validators.GetByAddress(address) //当前节点在validatorset中的索引
	cs.id_int = int(id_int)
	//下面是判断当前节点是不是拜占庭节点
	for _, node_id := range cs.config.ByzantineNodeList {
		if node_id == cs.id_int {
			cs.is_byz_node = true
			cs.Logger.Info("The node is a byz node", "the node's index in validatorset", id_int)
			break
		}
	}

	var timeout float64
	var byz_timeout float64
	var sample_data_list []float64
	var ProposerList []int
	if cs.config.UpdateLevel == "epoch" {
		//下面过程是正常节点计算timeout的过程
		if len(cs.bctListofMI) != int(cs.config.SmallMI) { //这个应该是处理异常情况的代码 有个height是缺失的
			if len(cs.bctListofMI) < int(cs.config.Epoch) {
				sample_data_list = append(sample_data_list, cs.bctListofMI...)
				ProposerList = append(ProposerList, cs.proposerListofMI...)
			} else {
				sample_data_list = append(sample_data_list, cs.bctListofMI[int(cs.config.SmallMI-size):]...)
				ProposerList = append(ProposerList, cs.proposerListofMI[int(cs.config.SmallMI-size):]...)
			}
		} else {
			if len(cs.bctListofMI) != int(cs.config.SmallMI) {
				panic("the error of the bctList of MI")
			}

			if len(cs.proposerListofMI) != int(cs.config.SmallMI) {
				panic("the error of the proposer of MI")
			}
			sample_data_list = append(sample_data_list, cs.bctListofMI[int(cs.config.SmallMI-size):]...)
			ProposerList = append(ProposerList, cs.proposerListofMI[int(cs.config.SmallMI-size):]...)
		}

		//TODO
		//当串谋的情况
		byz_node_dict := make(map[string]int, len(cs.config.ByzantineNodeList))
		for _, byz_node := range cs.config.ByzantineNodeList {
			byz_node_dict[strconv.Itoa(byz_node)] = 0
		}

		cs.Logger.Info("computeFirstTimeout ", "the sample's length", len(sample_data_list), "the sample list", sample_data_list)
		mean, std := mean_and_variance(sample_data_list)
		cs.Logger.Info("the peb.Multiple", "value", peb.Multiple)
		timeout = mean + peb.Multiple*std
		cs.Logger.Info("computeFirstTimeout ", "honest_node's avg_bct", mean, "std", std, "To", timeout)

		if cs.is_byz_node {
			var byz_sample_data_list []float64
			for k := 0; k < len(sample_data_list); k++ {
				proposer := ProposerList[k] //TODO bug
				if _, ok := byz_node_dict[strconv.Itoa(proposer)]; !ok {
					byz_sample_data_list = append(byz_sample_data_list, sample_data_list[k])
				}
			}
			cs.Logger.Info("computeFirstTimeout ", "the byz sample's length", len(byz_sample_data_list), "the byz sample list", byz_sample_data_list)
			byz_mean, byz_std := mean_and_variance(byz_sample_data_list)
			byz_timeout = byz_mean + peb.Multiple*byz_std
			cs.Logger.Info("computeFirstTimeout ", "byz_mean", byz_mean, "bz_Std", byz_std, "byz_To ", byz_timeout)
		} else {
			byz_mean := mean
			byz_std := std
			byz_timeout = timeout
			cs.Logger.Info("computeFirstTimeout ", "byz_mean", byz_mean, "bz_Std", byz_std, "byz_To ", byz_timeout)
		}
	}
	if cs.config.UpdateLevel == "block" {
		timeout_int64, byz_timeout_int64 := cs.ptl.TimeOut(cs.id_int, cs.is_byz_node, cs.config.ByzantineNodeList)
		timeout = float64(timeout_int64) / 1000.0
		byz_timeout = float64(byz_timeout_int64) / 1000.0
	}
	return int64(timeout * 1000), int64(byz_timeout * 1000)
}

func average(data []float64) float64 {
	sum := 0.0
	for _, value := range data {
		sum += value
	}
	return sum / float64(len(data))
}

func mean_and_variance(data []float64) (float64, float64) {
	mean := average(data)
	sum := 0.0
	for _, value := range data {
		sum += (value - mean) * (value - mean)
	}
	std := math.Sqrt(sum / float64(len(data)))
	return mean, std
}

// 这个函数其实是周期的更新propose_time应该分两种情况
// TODO 这个函数需要解释一下
// step1 首先判断当前的节点是不是拜占庭节点
// step2 接着求解两种timeout
// step3 周期性的保存一下参考信息（bct、cost、latency、round .....）
func (cs *State) updateProposeTimeOut() {

	address := cs.privValidatorPubKey.Address()
	id_int, _ := cs.Validators.GetByAddress(address)
	cs.id_int = int(id_int)
	//下面是判断当前节点是不是拜占庭节点
	for _, node_id := range cs.config.ByzantineNodeList {
		if node_id == cs.id_int {
			cs.is_byz_node = true
			cs.Logger.Info("The node is a byz node", "the node's index in validatorset", id_int)
		}
	}

	//根据拜占庭节点的信息，计算两个timeout，若当前节点为诚实节点，则这两个timeout应该是相同的
	//TODO 这个要修改一下block中的计算作恶时间的epochsize的大小，思路就是限制一下队列的大小
	honest_timeout, _ := cs.ptl.TimeOut(cs.id_int, cs.is_byz_node, cs.config.ByzantineNodeList) //计算下一个epoch的timeout

	//这个操作是节点之间的串谋
	td := 0.0
	if cs.is_byz_node {
		td = cs.block_bct_delay_time
	}

	p_d_t := cs.pt.GetPDT(cs.isProposer(address), cs.config.Mode, bytetostring(cs.ProposalBlockParts.Hash()), "0")
	cs.Logger.Info("the GetPDT's blockhash ", "blockhash", bytetostring(cs.ProposalBlockParts.Hash()), "td", td)

	//诚实节点
	//if cs.Round != int32(0) {
	//	p_d_t = p_d_t + 100.0
	//	//time.Sleep(10 * time.Second)
	//}

	if pingerr := cs.pingcmd.Process.Kill(); pingerr != nil {
		panic("end pingcmd")
	}
	pingresult := cs.out.String()
	fmt.Println("ping result", pingresult)
	re := regexp.MustCompile(`time=([\d.]+) ms`)
	fmt.Println(re)
	matches := re.FindAllStringSubmatch(pingresult, -1)
	fmt.Println(matches)
	if len(matches) == 0 {
		fmt.Println("No matches found. Please check the output format.")
		return
	}
	var times []float64
	for _, match := range matches {
		if len(match) > 1 {
			if t, err := strconv.ParseFloat(match[1], 64); err == nil {
				times = append(times, t)
			}
		}
	}

	// 计算统计值
	min, max, avg, std := calculateStats(times)

	fmt.Printf("Min: %f ms\n", min)
	fmt.Printf("Max: %f ms\n", max)
	fmt.Printf("Avg: %f ms\n", avg)
	fmt.Printf("Std: %f ms\n", std)

	test_height, test_timeout, test_byz_timeout, test_delay := cs.pcc_service.RunRLJ(cs.Height, td, p_d_t*1000.0, cs.is_crash_node, cs.Logger, min, max, avg, std, float64(cs.ReadDeliverTime(cs.Height)))
	cs.Logger.Info("test the service of RJL", "height", test_height, "PDT", p_d_t, "timeout", test_timeout, "honest_timeout", honest_timeout, "byz_timeout", test_byz_timeout, "td", test_delay)
	cs.delay_time = test_delay
	cs.cost.SaveCostPerEpoch()
	cs.cost = pto.NewEpochCost(cs.cost.EpochSize, cs.config.CostFilePath) //#将cost列表置为空

	cs.ptl.SaveSamplePerEpoch(cs.config.TimeoutPropose.Seconds()) //将本epoch中的信息持久化

	cs.config.CurrentProposeTime = time.Duration(test_byz_timeout) * time.Millisecond
	if cs.config.Update { //是否更新timeout
		//if honest_timeout > 3000 {
		//	honest_timeout = 3000
		//}
		//cs.config.TimeoutPropose = time.Duration(honest_timeout) * time.Millisecond
		cs.config.TimeoutPropose = time.Duration(test_timeout) * time.Millisecond

	}
	//TODO
	if cs.config.UpdateLevel != "block" && cs.config.UpdateLevel != "merge" { //更新粒度是不是block
		cs.ptl = peb.NewEpochProposeTimeout(cs.config.Epoch, cs.config.ProposeFilePath)
	}
	cs.view_change_number_group = 0
	cs.TimeoutOFHeight = cs.config.TimeoutPropose.Milliseconds()

}
func (cs *State) updateLatencyTimeOut() {
	cs.ltl.SaveSamplePerEpoch(cs.config.TimeoutPropose.Seconds())
	cs.ltl = makeLatencyTimeList(cs.config.Epoch, cs.config.LatencyFilePath)

}
func (cs *State) updateRoundTimeOut() {
	cs.crl.SaveSamplePerEpoch()
	cs.crl = makeCommitRoundList(cs.config.Epoch, cs.config.RoundFilePath)
}

func (cs *State) pruneBlocks(retainHeight int64) (uint64, error) {
	base := cs.blockStore.Base()
	if retainHeight <= base {
		return 0, nil
	}
	pruned, err := cs.blockStore.PruneBlocks(retainHeight)
	if err != nil {
		return 0, fmt.Errorf("failed to prune block store: %w", err)
	}
	err = cs.blockExec.Store().PruneStates(base, retainHeight)
	if err != nil {
		return 0, fmt.Errorf("failed to prune state database: %w", err)
	}
	return pruned, nil
}

// ping 函数执行 ping 命令并返回输出
func ping(host string, count int, interval time.Duration) (string, error) {
	var cmd *exec.Cmd

	// 根据操作系统选择参数
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", strconv.Itoa(count), host) // Windows 使用 -n
	} else {
		cmd = exec.Command("ping", "-c", strconv.Itoa(count), host) // Linux/macOS 使用 -c
	}

	// 设置 ping 命令的间隔
	if runtime.GOOS != "windows" {
		cmd.Args = append(cmd.Args, "-i", fmt.Sprintf("%.1f", interval.Seconds()))
	}

	var out bytes.Buffer
	cmd.Stdout = &out

	// 启动 ping 命令
	if err := cmd.Start(); err != nil {
		return "", err
	}

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		return "", err
	}

	// 返回输出结果
	return out.String(), nil
}

func calculateStats(times []float64) (min, max, avg, std float64) {
	if len(times) == 0 {
		return 0, 0, 0, 0
	}

	min = times[0]
	max = times[0]
	sum := 0.0

	// 计算 min, max 和 sum
	for _, value := range times {
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
		sum += value
	}

	avg = sum / float64(len(times))

	// 计算标准差
	var variance float64
	for _, value := range times {
		variance += (value - avg) * (value - avg)
	}
	variance /= float64(len(times))
	std = math.Sqrt(variance)

	return min, max, avg, std
}

func (cs *State) recordMetrics(height int64, block *types.Block) {
	cs.metrics.Validators.Set(float64(cs.Validators.Size()))
	cs.metrics.ValidatorsPower.Set(float64(cs.Validators.TotalVotingPower()))

	var (
		missingValidators      int
		missingValidatorsPower int64
	)
	// height=0 -> MissingValidators and MissingValidatorsPower are both 0.
	// Remember that the first LastCommit is intentionally empty, so it's not
	// fair to increment missing validators number.
	if height > cs.state.InitialHeight {
		// Sanity check that commit size matches validator set size - only applies
		// after first block.
		var (
			commitSize = block.LastCommit.Size()
			valSetLen  = len(cs.LastValidators.Validators)
			address    types.Address
		)
		if commitSize != valSetLen {
			panic(fmt.Sprintf("commit size (%d) doesn't match valset length (%d) at height %d\n\n%v\n\n%v",
				commitSize, valSetLen, block.Height, block.LastCommit.Signatures, cs.LastValidators.Validators))
		}

		if cs.privValidator != nil {
			if cs.privValidatorPubKey == nil {
				// Metrics won't be updated, but it's not critical.
				cs.Logger.Error(fmt.Sprintf("recordMetrics: %v", errPubKeyIsNotSet))
			} else {
				address = cs.privValidatorPubKey.Address()
			}
		}

		for i, val := range cs.LastValidators.Validators {
			commitSig := block.LastCommit.Signatures[i]
			if commitSig.Absent() {
				missingValidators++
				missingValidatorsPower += val.VotingPower
			}

			if bytes.Equal(val.Address, address) {
				label := []string{
					"validator_address", val.Address.String(),
				}
				cs.metrics.ValidatorPower.With(label...).Set(float64(val.VotingPower))
				if commitSig.ForBlock() {
					cs.metrics.ValidatorLastSignedHeight.With(label...).Set(float64(height))
				} else {
					cs.metrics.ValidatorMissedBlocks.With(label...).Add(float64(1))
				}
			}

		}
	}
	cs.metrics.MissingValidators.Set(float64(missingValidators))
	cs.metrics.MissingValidatorsPower.Set(float64(missingValidatorsPower))

	// NOTE: byzantine validators power and count is only for consensus evidence i.e. duplicate vote
	var (
		byzantineValidatorsPower = int64(0)
		byzantineValidatorsCount = int64(0)
	)
	for _, ev := range block.Evidence.Evidence {
		if dve, ok := ev.(*types.DuplicateVoteEvidence); ok {
			if _, val := cs.Validators.GetByAddress(dve.VoteA.ValidatorAddress); val != nil {
				byzantineValidatorsCount++
				byzantineValidatorsPower += val.VotingPower
			}
		}
	}
	cs.metrics.ByzantineValidators.Set(float64(byzantineValidatorsCount))
	cs.metrics.ByzantineValidatorsPower.Set(float64(byzantineValidatorsPower))

	if height > 1 {
		lastBlockMeta := cs.blockStore.LoadBlockMeta(height - 1)
		if lastBlockMeta != nil {
			cs.metrics.BlockIntervalSeconds.Observe(
				block.Time.Sub(lastBlockMeta.Header.Time).Seconds(),
			)
		}
	}

	cs.metrics.NumTxs.Set(float64(len(block.Data.Txs)))
	cs.metrics.TotalTxs.Add(float64(len(block.Data.Txs)))
	cs.metrics.BlockSizeBytes.Set(float64(block.Size()))
	cs.metrics.CommittedHeight.Set(float64(block.Height))
}

// -----------------------------------------------------------------------------
// 提案有效的时机 必须是相同的height和相同的round
func (cs *State) defaultSetProposal(proposal *types.Proposal) error {
	// Already have one
	// TODO: possibly catch double proposals
	//
	cs.Logger.Debug("handle proposal")
	if cs.Proposal != nil {
		return nil
	}

	// Does not apply
	if proposal.Height != cs.Height || proposal.Round != cs.Round {
		return nil
	}

	// Verify POLRound, which must be -1 or in range [0, proposal.Round).
	if proposal.POLRound < -1 ||
		(proposal.POLRound >= 0 && proposal.POLRound >= proposal.Round) {
		return ErrInvalidProposalPOLRound
	}

	p := proposal.ToProto()
	// Verify signature
	if !cs.Validators.GetProposer().PubKey.VerifySignature(
		types.ProposalSignBytes(cs.state.ChainID, p), proposal.Signature,
	) {
		return ErrInvalidProposalSignature
	}

	proposal.Signature = p.Signature
	cs.Proposal = proposal
	// We don't update cs.ProposalBlockParts if it is already set.
	// This happens if we're already in cstypes.RoundStepCommit or if there is a valid block in the current round.
	// TODO: We can check if Proposal is for a different block as this is a sign of misbehavior!
	if cs.ProposalBlockParts == nil {
		cs.ProposalBlockParts = types.NewPartSetFromHeader(proposal.BlockID.PartSetHeader)
	}

	cs.Logger.Info("received proposal", "proposal", proposal)
	return nil
}

// NOTE: block is not necessarily valid.
// Asynchronously triggers either enterPrevote (before we proposetimeout of propose) or tryFinalizeCommit,
// once we have the full block.
func (cs *State) addProposalBlockPart(msg *BlockPartMessage, peerID p2p.ID) (added bool, err error) {
	cs.Logger.Debug("handle block part")
	height, round, part := msg.Height, msg.Round, msg.Part

	// Blocks might be reused, so round mismatch is OK
	if cs.Height != height {
		cs.Logger.Debug("received block part from wrong height", "height", height, "round", round)
		return false, nil
	}

	// We're not expecting a block part.
	if cs.ProposalBlockParts == nil {
		// NOTE: this can happen when we've gone to a higher round and
		// then receive parts from the previous round - not necessarily a bad peer.
		cs.Logger.Debug(
			"received a block part when we are not expecting any",
			"height", height,
			"round", round,
			"index", part.Index,
			"peer", peerID,
		)
		return false, nil
	}

	added, err = cs.ProposalBlockParts.AddPart(part)
	if err != nil {
		return added, err
	}
	if cs.ProposalBlockParts.ByteSize() > cs.state.ConsensusParams.Block.MaxBytes {
		return added, fmt.Errorf("total size of proposal block parts exceeds maximum block bytes (%d > %d)",
			cs.ProposalBlockParts.ByteSize(), cs.state.ConsensusParams.Block.MaxBytes,
		)
	}
	if added && cs.ProposalBlockParts.IsComplete() {
		bz, err := io.ReadAll(cs.ProposalBlockParts.GetReader())
		if err != nil {
			return added, err
		}

		pbb := new(tmproto.Block)
		err = proto.Unmarshal(bz, pbb)
		if err != nil {
			return added, err
		}

		block, err := types.BlockFromProto(pbb)
		if err != nil {
			return added, err
		}

		cs.ProposalBlock = block

		// NOTE: it's possible to receive complete proposal blocks for future rounds without having the proposal
		cs.Logger.Info("received complete proposal block", "height", cs.ProposalBlock.Height, "hash", cs.ProposalBlock.Hash(), "block_size", cs.ProposalBlock.Size())

		if err := cs.eventBus.PublishEventCompleteProposal(cs.CompleteProposalEvent()); err != nil {
			cs.Logger.Error("failed publishing event complete proposal", "err", err)
		}
	}
	return added, nil
}

// 处理完整的区块提案，
// 如果已经在本轮轮次拿到了2/3的投票，并且有效的区块轮次<当前的轮次，并且投票和当前的提案相同，则重制有效区块为当前的提案，
// 这个是说明可能轮次中先收到投票
func (cs *State) handleCompleteProposal(blockHeight int64) {
	// Update Valid* if we can.
	prevotes := cs.Votes.Prevotes(cs.Round) //当前轮次对应的prevote已经超过了2/3
	blockID, hasTwoThirds := prevotes.TwoThirdsMajority()
	if hasTwoThirds && !blockID.IsZero() && (cs.ValidRound < cs.Round) {
		if cs.ProposalBlock.HashesTo(blockID.Hash) {
			cs.Logger.Debug(
				"updating valid block to new proposal block",
				"valid_round", cs.Round,
				"valid_block_hash", log.NewLazyBlockHash(cs.ProposalBlock),
			)

			cs.ValidRound = cs.Round
			cs.ValidBlock = cs.ProposalBlock
			cs.ValidBlockParts = cs.ProposalBlockParts
		}
		// TODO: In case there is +2/3 majority in Prevotes set for some
		// block and cs.ProposalBlock contains different block, either
		// proposer is faulty or voting power of faulty processes is more
		// than 1/3. We should trigger in the future accountability
		// procedure at this point.
	}

	if cs.Step <= cstypes.RoundStepPropose && cs.isProposalComplete() {
		// Move onto the next step
		fmt.Println("从正常情况进入")
		cs.enterPrevote(blockHeight, cs.Round)
		if hasTwoThirds { // this is optimisation as this will be triggered when prevote is added
			cs.enterPrecommit(blockHeight, cs.Round)
		}
	} else if cs.Step == cstypes.RoundStepCommit {
		// If we're waiting on the proposal block...
		cs.tryFinalizeCommit(blockHeight)
	}
}

// Attempt to add the vote. if its a duplicate signature, dupeout the validator
func (cs *State) tryAddVote(vote *types.Vote, peerID p2p.ID) (bool, error) {
	added, err := cs.addVote(vote, peerID)
	if err != nil {
		// If the vote height is off, we'll just ignore it,
		// But if it's a conflicting sig, add it to the cs.evpool.
		// If it's otherwise invalid, punish peer.
		//nolint: gocritic
		if voteErr, ok := err.(*types.ErrVoteConflictingVotes); ok {
			if cs.privValidatorPubKey == nil {
				return false, errPubKeyIsNotSet
			}

			if bytes.Equal(vote.ValidatorAddress, cs.privValidatorPubKey.Address()) {
				cs.Logger.Error(
					"found conflicting vote from ourselves; did you unsafe_reset a validator?",
					"height", vote.Height,
					"round", vote.Round,
					"type", vote.Type,
				)

				return added, err
			}

			// report conflicting votes to the evidence pool
			cs.evpool.ReportConflictingVotes(voteErr.VoteA, voteErr.VoteB)
			cs.Logger.Debug(
				"found and sent conflicting votes to the evidence pool",
				"vote_a", voteErr.VoteA,
				"vote_b", voteErr.VoteB,
			)

			return added, err
		} else if errors.Is(err, types.ErrVoteNonDeterministicSignature) {
			cs.Logger.Debug("vote has non-deterministic signature", "err", err)
		} else {
			// Either
			// 1) bad peer OR
			// 2) not a bad peer? this can also err sometimes with "Unexpected step" OR
			// 3) tmkms use with multiple validators connecting to a single tmkms instance
			// 		(https://github.com/tendermint/tendermint/issues/3839).
			cs.Logger.Info("failed attempting to add vote", "err", err)
			return added, ErrAddingVote
		}
	}

	return added, nil
}

// addvote的过程
// 如果是上一个高度的precommit-vote，并且当前的状态在NewHeight step 则加入lastCommit中，此外若已经搜集满票，且可以跳过skipCommit。则进入NewRound
// 此外只要是高度相同的票，就加入进来.
func (cs *State) addVote(vote *types.Vote, peerID p2p.ID) (added bool, err error) {
	cs.Logger.Debug(
		"adding vote",
		"vote_height", vote.Height,
		"vote_type", vote.Type,
		"val_index", vote.ValidatorIndex,
		"cs_height", cs.Height,
	)

	// A precommit for the previous height?
	// These come in while we wait timeoutCommit
	if vote.Height+1 == cs.Height && vote.Type == tmproto.PrecommitType {
		if cs.Step != cstypes.RoundStepNewHeight {
			// Late precommit at prior height is ignored
			cs.Logger.Debug("precommit vote came in after commit proposetimeout and has been ignored", "vote", vote)
			return
		}

		added, err = cs.LastCommit.AddVote(vote)
		if !added {
			return
		}

		//cs.Logger.Debug("added vote to last precommits", "last_commit", cs.LastCommit.StringShort())
		if err := cs.eventBus.PublishEventVote(types.EventDataVote{Vote: vote}); err != nil {
			return added, err
		}

		cs.evsw.FireEvent(types.EventVote, vote)

		// if we can skip timeoutCommit and have all the votes now,
		if cs.config.SkipTimeoutCommit && cs.LastCommit.HasAll() {
			// go straight to new round (skip proposetimeout commit)
			// cs.scheduleTimeout(time.Duration(0), cs.Height, 0, cstypes.RoundStepNewHeight)
			cs.enterNewRound(cs.Height, 0)
		}

		return
	}

	// Height mismatch is ignored.
	// Not necessarily a bad peer, but not favourable behaviour.
	if vote.Height != cs.Height {
		cs.Logger.Debug("vote ignored and not added", "vote_height", vote.Height, "cs_height", cs.Height, "peer", peerID)
		return
	}

	height := cs.Height
	added, err = cs.Votes.AddVote(vote, peerID)
	if !added {
		// Either duplicate, or error upon cs.Votes.AddByIndex()
		return
	}

	if err := cs.eventBus.PublishEventVote(types.EventDataVote{Vote: vote}); err != nil {
		return added, err
	}
	cs.evsw.FireEvent(types.EventVote, vote)

	switch vote.Type {
	case tmproto.PrevoteType:
		prevotes := cs.Votes.Prevotes(vote.Round)
		if vote.Round == cs.Round && prevotes.HasAll() {
			//
			if !cs.prevote_flag {
				if !prevotes.HasTwoThirdsMajority() {
					cs.Logger.Info("All prevote votes for this round have been received", "round", vote)
					cs.cost.CostList[len(cs.cost.CostList)-1].CrashFlag = true
					cs.cost.CostList[len(cs.cost.CostList)-1].GetALLPrevote = append(cs.cost.CostList[len(cs.cost.CostList)-1].GetALLPrevote, time.Now())
					cs.cost.CostList[len(cs.cost.CostList)-1].ALLPrevoteRound = append(cs.cost.CostList[len(cs.cost.CostList)-1].ALLPrevoteRound, cs.Round)
				}
				cs.prevote_flag = true
			}

		}
		//cs.Logger.Info("added vote to prevote", "vote", vote, "prevotes", prevotes.StringShort())

		// If +2/3 prevotes for a block or nil for *any* round:
		if blockID, ok := prevotes.TwoThirdsMajority(); ok {
			// There was a polka!
			// If we're locked but this is a recent polka, unlock.
			// If it matches our ProposalBlock, update the ValidBlock

			// Unlock if `cs.LockedRound < vote.Round <= cs.Round`
			// NOTE: If vote.Round > cs.Round, we'll deal with it when we get to vote.Round
			if (cs.LockedBlock != nil) &&
				(cs.LockedRound < vote.Round) &&
				(vote.Round <= cs.Round) &&
				!cs.LockedBlock.HashesTo(blockID.Hash) {

				cs.Logger.Debug("unlocking because of POL", "locked_round", cs.LockedRound, "pol_round", vote.Round)

				cs.LockedRound = -1
				cs.LockedBlock = nil
				cs.LockedBlockParts = nil

				if err := cs.eventBus.PublishEventUnlock(cs.RoundStateEvent()); err != nil {
					return added, err
				}
			}

			// Update Valid* if we can.
			// NOTE: our proposal block may be nil or not what received a polka..
			if len(blockID.Hash) != 0 && (cs.ValidRound < vote.Round) && (vote.Round == cs.Round) {
				if cs.ProposalBlock.HashesTo(blockID.Hash) {
					cs.Logger.Debug("updating valid block because of POL", "valid_round", cs.ValidRound, "pol_round", vote.Round)
					cs.ValidRound = vote.Round
					cs.ValidBlock = cs.ProposalBlock
					cs.ValidBlockParts = cs.ProposalBlockParts
				} else {
					cs.Logger.Debug(
						"valid block we do not know about; set ProposalBlock=nil",
						"proposal", log.NewLazyBlockHash(cs.ProposalBlock),
						"block_id", blockID.Hash,
					)

					// we're getting the wrong block
					cs.ProposalBlock = nil
				}

				if !cs.ProposalBlockParts.HasHeader(blockID.PartSetHeader) {
					cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartSetHeader)
				}

				cs.evsw.FireEvent(types.EventValidBlock, &cs.RoundState)
				if err := cs.eventBus.PublishEventValidBlock(cs.RoundStateEvent()); err != nil {
					return added, err
				}
			}
		}

		// If +2/3 prevotes for *anything* for future round:
		switch {
		case cs.Round < vote.Round && prevotes.HasTwoThirdsAny():
			// Round-skip if there is any 2/3+ of votes ahead of us
			cs.enterNewRound(height, vote.Round) //进入新的round （当前的机器就是慢）

		case cs.Round == vote.Round && cstypes.RoundStepPrevote <= cs.Step: // current round  这个是一个轮次，并且已经投完票了
			blockID, ok := prevotes.TwoThirdsMajority()
			if ok && (cs.isProposalComplete() || len(blockID.Hash) == 0) {
				cs.enterPrecommit(height, vote.Round)
			} else if prevotes.HasTwoThirdsAny() {
				cs.enterPrevoteWait(height, vote.Round) //我们肯定是属于这种情况
			}

		case cs.Proposal != nil && 0 <= cs.Proposal.POLRound && cs.Proposal.POLRound == vote.Round:
			// If the proposal is now complete, enter prevote of cs.Round. 存在()
			if cs.isProposalComplete() { //wi
				fmt.Println("从情况4进入")
				cs.enterPrevote(height, cs.Round) //这个其实是一个特殊情况，实说提案是完整了，也是当前层次
			}
		} //
		//当收到同一高度的precommit投票时
	//
	case tmproto.PrecommitType:
		precommits := cs.Votes.Precommits(vote.Round)
		if vote.Round == cs.Round && precommits.HasAll() {
			cs.Logger.Info("All precommit votes for this round have been received", "round", vote)
			cs.cost.CostList[len(cs.cost.CostList)-1].GetALLPreCommit = append(cs.cost.CostList[len(cs.cost.CostList)-1].GetALLPreCommit, time.Now())
			cs.cost.CostList[len(cs.cost.CostList)-1].ALLPreCommitRound = append(cs.cost.CostList[len(cs.cost.CostList)-1].ALLPreCommitRound, cs.Round)
		}
		//cs.Logger.Info("added vote to precommit",
		//	"height", vote.Height,
		//	"round", vote.Round,
		//	"validator", vote.ValidatorAddress.String(),
		//	"vote_timestamp", vote.Timestamp,
		//	"data", precommits.LogString())

		blockID, ok := precommits.TwoThirdsMajority()
		if ok {
			// Executed as TwoThirdsMajority could be from a higher round
			cs.enterNewRound(height, vote.Round) //TODO 我理解是低轮次也可以
			cs.enterPrecommit(height, vote.Round)

			if len(blockID.Hash) != 0 {
				cs.enterCommit(height, vote.Round)
				if cs.config.SkipTimeoutCommit && precommits.HasAll() {
					cs.enterNewRound(cs.Height, 0)
				}
			} else {
				fmt.Println("尽管对投票的")
				cs.enterPrecommitWait(height, vote.Round) //这个应该是已经拿到了2/3以上的投票，但是没有区块，投票为空
			}
		} else if cs.Round <= vote.Round && precommits.HasTwoThirdsAny() {
			cs.enterNewRound(height, vote.Round)
			cs.enterPrecommitWait(height, vote.Round)
		}

	default:
		panic(fmt.Sprintf("unexpected vote type %v", vote.Type))
	}

	return added, err
}

// CONTRACT: cs.privValidator is not nil.
func (cs *State) signVote(
	msgType tmproto.SignedMsgType,
	hash []byte,
	header types.PartSetHeader,
) (*types.Vote, error) {
	// Flush the WAL. Otherwise, we may not recompute the same vote to sign,
	// and the privValidator will refuse to sign anything.
	startentiretime := time.Now()
	startwaltime := time.Now()
	if err := cs.wal.FlushAndSync(); err != nil {
		return nil, err
	}
	endwaltime := time.Now()

	if cs.privValidatorPubKey == nil {
		return nil, errPubKeyIsNotSet
	}

	addr := cs.privValidatorPubKey.Address()
	valIdx, _ := cs.Validators.GetByAddress(addr)

	vote := &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   valIdx,
		Height:           cs.Height,
		Round:            cs.Round,
		Timestamp:        cs.voteTime(),
		Type:             msgType,
		BlockID:          types.BlockID{Hash: hash, PartSetHeader: header},
	}

	v := vote.ToProto()
	startsigntime := time.Now()
	err := cs.privValidator.SignVote(cs.state.ChainID, v)
	endsignTime := time.Now()
	vote.Signature = v.Signature
	vote.Timestamp = v.Timestamp
	endentiretime := time.Now()
	cs.Logger.Info("singvote through time", "sign type", msgType.String(), "entire vote time", endentiretime.Sub(startentiretime).Seconds(), "wal flush time", endwaltime.Sub(startwaltime).Seconds(), "only sign time", endsignTime.Sub(startsigntime).Seconds(), "height", cs.Height)
	return vote, err
}

func (cs *State) voteTime() time.Time {
	now := tmtime.Now()
	minVoteTime := now
	// TODO: We should remove next line in case we don't vote for v in case cs.ProposalBlock == nil,
	// even if cs.LockedBlock != nil. See https://github.com/tendermint/tendermint/tree/v0.34.x/spec/.
	timeIota := time.Duration(cs.state.ConsensusParams.Block.TimeIotaMs) * time.Millisecond
	if cs.LockedBlock != nil {
		// See the BFT time spec
		// https://github.com/tendermint/tendermint/blob/v0.34.x/spec/consensus/bft-time.md
		minVoteTime = cs.LockedBlock.Time.Add(timeIota)
	} else if cs.ProposalBlock != nil {
		minVoteTime = cs.ProposalBlock.Time.Add(timeIota)
	}

	if now.After(minVoteTime) {
		return now
	}
	return minVoteTime
}

// sign the vote and publish on internalMsgQueue
func (cs *State) signAddVote(msgType tmproto.SignedMsgType, hash []byte, header types.PartSetHeader) *types.Vote {
	if cs.privValidator == nil { // the node does not have a key
		return nil
	}

	if cs.privValidatorPubKey == nil {
		// Vote won't be signed, but it's not critical.
		cs.Logger.Error(fmt.Sprintf("signAddVote: %v", errPubKeyIsNotSet))
		return nil
	}

	// If the node not in the validator set, do nothing.
	if !cs.Validators.HasAddress(cs.privValidatorPubKey.Address()) {
		return nil
	}

	// TODO: pass pubKey to signVote
	vote, err := cs.signVote(msgType, hash, header)
	if err == nil {
		cs.sendInternalMessage(msgInfo{&VoteMessage{vote}, ""})
		cs.Logger.Debug("signed and pushed vote", "height", cs.Height, "round", cs.Round, "vote", vote)
		return vote
	}

	cs.Logger.Error("failed signing vote", "height", cs.Height, "round", cs.Round, "vote", vote, "err", err)
	return nil
}

// updatePrivValidatorPubKey get's the private validator public key and
// memoizes it. This func returns an error if the private validator is not
// responding or responds with an error.
func (cs *State) updatePrivValidatorPubKey() error {
	if cs.privValidator == nil {
		return nil
	}

	pubKey, err := cs.privValidator.GetPubKey()
	if err != nil {
		return err
	}
	cs.privValidatorPubKey = pubKey
	return nil
}

// look back to check existence of the node's consensus votes before joining consensus
func (cs *State) checkDoubleSigningRisk(height int64) error {
	if cs.privValidator != nil && cs.privValidatorPubKey != nil && cs.config.DoubleSignCheckHeight > 0 && height > 0 {
		valAddr := cs.privValidatorPubKey.Address()
		doubleSignCheckHeight := cs.config.DoubleSignCheckHeight
		if doubleSignCheckHeight > height {
			doubleSignCheckHeight = height
		}

		for i := int64(1); i < doubleSignCheckHeight; i++ {
			lastCommit := cs.blockStore.LoadSeenCommit(height - i)
			if lastCommit != nil {
				for sigIdx, s := range lastCommit.Signatures {
					if s.BlockIDFlag == types.BlockIDFlagCommit && bytes.Equal(s.ValidatorAddress, valAddr) {
						cs.Logger.Info("found signature from the same key", "sig", s, "idx", sigIdx, "height", height-i)
						return ErrSignatureFoundInPastBlocks
					}
				}
			}
		}
	}

	return nil
}

func (cs *State) calculatePrevoteMessageDelayMetrics() {
	if cs.Proposal == nil {
		return
	}

	ps := cs.Votes.Prevotes(cs.Round)
	pl := ps.List()

	sort.Slice(pl, func(i, j int) bool {
		return pl[i].Timestamp.Before(pl[j].Timestamp)
	})

	var votingPowerSeen int64
	for _, v := range pl {
		_, val := cs.Validators.GetByAddress(v.ValidatorAddress)
		votingPowerSeen += val.VotingPower
		if votingPowerSeen >= cs.Validators.TotalVotingPower()*2/3+1 {
			cs.metrics.QuorumPrevoteMessageDelay.Set(v.Timestamp.Sub(cs.Proposal.Timestamp).Seconds())
			break
		}
	}
	if ps.HasAll() {
		cs.metrics.FullPrevoteMessageDelay.Set(pl[len(pl)-1].Timestamp.Sub(cs.Proposal.Timestamp).Seconds())
	}
}

//---------------------------------------------------------

func CompareHRS(h1 int64, r1 int32, s1 cstypes.RoundStepType, h2 int64, r2 int32, s2 cstypes.RoundStepType) int {
	if h1 < h2 {
		return -1
	} else if h1 > h2 {
		return 1
	}
	if r1 < r2 {
		return -1
	} else if r1 > r2 {
		return 1
	}
	if s1 < s2 {
		return -1
	} else if s1 > s2 {
		return 1
	}
	return 0
}

// repairWalFile decodes messages from src (until the decoder errors) and
// writes them to dst.
func repairWalFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	var (
		dec = NewWALDecoder(in)
		enc = NewWALEncoder(out)
	)

	// best-case repair (until first error is encountered)
	for {
		msg, err := dec.Decode()
		if err != nil {
			break
		}

		err = enc.Encode(msg)
		if err != nil {
			return fmt.Errorf("failed to encode msg: %w", err)
		}
	}

	return nil
}
