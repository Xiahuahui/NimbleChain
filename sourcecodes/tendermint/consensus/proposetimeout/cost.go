package proposetimeout

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"
)

var head = 0

type Cost struct {
	Height               int64
	Round0StartTime      time.Time
	RoundCommitStartTime time.Time
	AfterRound           int32
	ViewChange           int64
	Threat               int64
	BCT                  int64 //收到区块的时间
	GetALLPrevote        []time.Time
	ALLPrevoteRound      []int32 //上边的时间对应的轮次
	GetALLPreCommit      []time.Time
	ALLPreCommitRound    []int32     //上边时间对应的轮次
	EnterPrecommitTime   []time.Time //进入precommit阶段的时间
	PreCommitRound       []int32
	EnterNewRoundTime    []time.Time //进入到下一个轮次的时间
	NewRoundRound        []int32     //下一个轮次是多少
	CrashFlag            bool
	Discretionary        int64 //当前高度的浪费的时间
	Flag                 bool
}

//ComputeCost 这个函数用来计算当前区块的Cost，计算Cost的步骤如下：
//当没有发生view-change时，即只有威胁空间的情况；那就是timeout-vc
//对于没有发生view-change的情况：
//				1当前的timeout比bct的的时间设置的长，那么cost为正，
//				2当前timeout比bct的时间设置的较短，但是该节点的占比小于1/3对共识的过程造不成影响 TODO（这种情况的cost的计算问题）
//
//当发生view-change时：即只有vc代价：
//               1当timeout>bct时，其实vc的情况并不是该节点造成的，这种情况的cost的计算问题
//               2.当timeout<bct时，其实这有两种： 僵持情况，（作恶节点在1/3-2/3之间的一个情况）vc的情况

func (cost *Cost) ComputeCost(timeout int64) {
	if cost.AfterRound == 0 {
		cost.ViewChange = 0
		fmt.Println("timeout:", timeout, "BCT", cost.BCT)
		cost.Threat = timeout - cost.BCT
	} else {
		if len(cost.GetALLPrevote) > 0 {
			cost.Flag = true
			fmt.Println("prevote time ", cost.GetALLPrevote, "preround: ", cost.ALLPrevoteRound, "precommit time: ", cost.EnterPrecommitTime, "precomit round", cost.PreCommitRound)
			for i := 0; i < len(cost.GetALLPrevote); i++ {
				if cost.ALLPrevoteRound[i] != cost.PreCommitRound[i] {
					panic("error from prevote cost compute")
				}
				cost_one_round := (cost.EnterPrecommitTime[i].Sub(cost.GetALLPrevote[i])).Milliseconds()
				cost.Discretionary = cost.Discretionary + cost_one_round
			}
			cost.ViewChange = cost.RoundCommitStartTime.Sub(cost.Round0StartTime).Milliseconds() - cost.Discretionary
			cost.Threat = int64(0)
		} else {
			//这块还是有bug
			cost.ViewChange = cost.RoundCommitStartTime.Sub(cost.Round0StartTime).Milliseconds()
			cost.Threat = int64(0)
		}
	}
}

// 这些cost的epoch的作用是什么
type EpochCost struct {
	EpochSize int64
	CostList  []*Cost
	SaveFile  string
}

// 制造新的EpochProposeTimeout 并且初始化csv文件
func NewEpochCost(epoch_size int64, save_file string) *EpochCost {
	ept := EpochCost{
		EpochSize: epoch_size,
		SaveFile:  save_file,
	}
	file, err := os.OpenFile(ept.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	if head == 0 {
		// 创建CSV writer
		writer := csv.NewWriter(file)
		defer writer.Flush()
		var strRecord []string
		for k := 0; k < int(epoch_size); k++ {
			strRecord = append(strRecord, strconv.Itoa(k))
			strRecord = append(strRecord, "cost")
			strRecord = append(strRecord, "view-change-cost")
			strRecord = append(strRecord, "threat-cost")
			strRecord = append(strRecord, "crash")
			strRecord = append(strRecord, "Discretionary")
		}
		// 写入数据
		err = writer.Write(strRecord)
		if err != nil {
			panic(err)
		}
		head = head + 1
	}
	return &ept
}

func (epoch_cost *EpochCost) Clear(epoch_size int64) {
	file, err := os.OpenFile(epoch_cost.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 创建CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()
	var strRecord []string
	for index, cost := range epoch_cost.CostList {
		strRecord = append(strRecord, strconv.Itoa(index))
		strRecord = append(strRecord, strconv.Itoa(int(cost.ViewChange+cost.Threat)))
		strRecord = append(strRecord, strconv.Itoa(int(cost.ViewChange)))
		strRecord = append(strRecord, strconv.Itoa(int(cost.Threat)))
		strRecord = append(strRecord, strconv.FormatBool(cost.Flag))
		strRecord = append(strRecord, strconv.Itoa(int(cost.Discretionary)))
	}
	// 写入数据
	err = writer.Write(strRecord)
	if err != nil {
		panic(err)
	}
	epoch_cost.EpochSize = epoch_size
	epoch_cost.CostList = epoch_cost.CostList[:0]
}

// 按照epoch的方式保存Cost的开销。
func (epoch_cost *EpochCost) SaveCostPerEpoch() {

	file, err := os.OpenFile(epoch_cost.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 创建CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()
	var strRecord []string
	for index, cost := range epoch_cost.CostList {
		strRecord = append(strRecord, strconv.Itoa(index))
		strRecord = append(strRecord, strconv.Itoa(int(cost.ViewChange+cost.Threat)))
		strRecord = append(strRecord, strconv.Itoa(int(cost.ViewChange)))
		strRecord = append(strRecord, strconv.Itoa(int(cost.Threat)))
		strRecord = append(strRecord, strconv.FormatBool(cost.Flag))
		strRecord = append(strRecord, strconv.Itoa(int(cost.Discretionary)))
	}
	// 写入数据
	err = writer.Write(strRecord)
	if err != nil {
		panic(err)
	}
}
