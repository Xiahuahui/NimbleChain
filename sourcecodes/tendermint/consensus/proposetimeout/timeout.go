package proposetimeout

import (
	"fmt"
	"time"
)

type Timeout interface {
	//输入参数当前节点是否是作恶节点，所有的byz节点的列表
	TimeOut(node_index int, flag bool, byz []int) (int64, int64)
	//增加时间时间到当前节点上
	AddTime(time float64, index int, proposer int)
	//每过一个epoch时期后保存我自己的采样信息
	SaveSamplePerEpoch(timeout float64)
	Clear(size int64)
	GetSampledata() []float64
	AdjustParameters(code int)
}

// 这个是用来采样propose的时间
type ProposeTime struct {
	Height                  int64
	Propose_start_time      time.Time
	Start_round             int64
	Propose_end_time        time.Time
	Propose_end_time_dict   map[string]time.Time
	Propose_start_time_dict map[string]time.Time
	End_round               int64
	Dur                     float64
	BCTDur                  float64
	PDTDur                  float64
}

func MakeNewProposeTime() ProposeTime {
	pt := ProposeTime{Propose_start_time_dict: make(map[string]time.Time), Propose_end_time_dict: make(map[string]time.Time)}
	return pt
}

func (pt *ProposeTime) AddProposeStartTime(round string, start_time time.Time) {

	if _, exists := pt.Propose_start_time_dict[round]; !exists {
		// 键不存在，插入键值对
		pt.Propose_start_time_dict[round] = start_time

	} else {

		fmt.Println("the round is starting")
	}
}

func (pt *ProposeTime) AddProposeEndTime(block_hash string, end_time time.Time) {

	if _, exists := pt.Propose_end_time_dict[block_hash]; !exists {
		// 键不存在，插入键值对
		pt.Propose_end_time_dict[block_hash] = end_time

	} else {
		pt.Propose_end_time_dict[block_hash] = end_time
		fmt.Println("the same leader generate same block")
	}
}

func (pt *ProposeTime) GetBCT(is_propose bool, mode bool, block_hash string, round string) float64 {
	fmt.Println("the Propose_end_time_dict is: ", pt.Propose_end_time_dict, pt.Propose_end_time, pt.Propose_end_time.Sub(pt.Propose_start_time).Seconds())
	if _, exists := pt.Propose_end_time_dict[block_hash]; exists {
		// 键不存在，插入键值对
		pt.Propose_end_time = pt.Propose_end_time_dict[block_hash]

	} else {
		panic("the block_hash not in the dict")
	}
	pt.Dur = pt.BCTDur
	return pt.RPT(is_propose, mode, round)
}

func (pt *ProposeTime) SetBCT(bct_value float64) {
	pt.BCTDur = bct_value
}

func (pt *ProposeTime) SetPDT(pdt_value float64) {
	pt.PDTDur = pdt_value
}

func (pt *ProposeTime) GetPDT(is_propose bool, mode bool, block_hash string, round string) float64 {
	fmt.Println("the Propose_end_time_dict is: ", pt.Propose_end_time_dict, pt.Propose_end_time, pt.Propose_end_time.Sub(pt.Propose_start_time).Seconds())
	if _, exists := pt.Propose_end_time_dict[block_hash]; exists {
		// 键不存在，插入键值对
		pt.Propose_end_time = pt.Propose_end_time_dict[block_hash]

	} else {
		panic("the block_hash not in the dict")
	}
	pt.Dur = pt.PDTDur
	return pt.RPT(is_propose, mode, round)
}

func (pt *ProposeTime) RPT(is_propose bool, mode bool, round string) float64 {
	pt.Propose_start_time = pt.Propose_start_time_dict[round]
	fmt.Println("the round", round, "    start_time is", pt.Propose_start_time, "start_time_dict is", pt.Propose_start_time_dict)
	if mode {
		if !is_propose {
			pt.Dur = pt.Propose_end_time.Sub(pt.Propose_start_time).Seconds()
			return pt.Dur
		}
		return pt.Dur
	}
	pt.Dur = pt.Propose_end_time.Sub(pt.Propose_start_time).Seconds()
	return pt.Dur
}
