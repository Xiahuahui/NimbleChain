package consensus

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"
)

// 这个是区块的延迟统计
type BlockLatency struct {
	height     int64
	start_time time.Time
	end_time   time.Time
	dur        float64
}

func (bl *BlockLatency) Latency() float64 {

	bl.dur = bl.end_time.Sub(bl.start_time).Seconds()
	return bl.dur
}

// 统计的区块延迟的列表
type LatencyTimeList struct {
	latency_time_list []float64
	EpochSize         int64
	SaveFile          string
}

func makeLatencyTimeList(epoch_size int64, save_file string) LatencyTimeList {
	ltl := LatencyTimeList{
		latency_time_list: make([]float64, epoch_size),
		EpochSize:         epoch_size,
		SaveFile:          save_file,
	}
	file, err := os.OpenFile(ltl.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	return ltl
}

func (ltl *LatencyTimeList) SaveSamplePerEpoch(timeout float64) {
	// 打开文件，如果文件不存在则创建，以追加模式写入
	file, err := os.OpenFile(ltl.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 创建CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()
	var strRecord []string
	for _, value := range ltl.latency_time_list {
		strRecord = append(strRecord, strconv.FormatFloat(value, 'f', -1, 64))
	}

	// 写入数据
	err = writer.Write(strRecord)
	if err != nil {
		panic(err)
	}

}

func (ltl *LatencyTimeList) Clear(epoch_size int64) {
	file, err := os.OpenFile(ltl.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 创建CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()
	var strRecord []string
	for _, value := range ltl.latency_time_list {
		if value != 0.0 {
			strRecord = append(strRecord, strconv.FormatFloat(value, 'f', -1, 64))
		}
	}

	// 写入数据
	err = writer.Write(strRecord)
	if err != nil {
		panic(err)
	}
	ltl.latency_time_list = make([]float64, epoch_size)
	ltl.EpochSize = epoch_size
}

func (ltl *LatencyTimeList) AddTime(time float64, index int, proposer int) {
	ltl.latency_time_list[index] = float64(time)
	fmt.Println("当前的延迟时间列表", ltl.latency_time_list)
}

type CommitRoundList struct {
	RoundList []float64
	EpochSize int64
	SaveFile  string
}

func makeCommitRoundList(epoch_size int64, save_file string) CommitRoundList {
	ltl := CommitRoundList{
		RoundList: make([]float64, epoch_size),
		EpochSize: epoch_size,
		SaveFile:  save_file,
	}
	file, err := os.OpenFile(ltl.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	return ltl
}
func (crl *CommitRoundList) AddRound(Round float64, index int) {
	crl.RoundList[index] = Round
	fmt.Println("当前提交轮次的列表", crl.RoundList)
}
func (crl *CommitRoundList) SaveSamplePerEpoch() {
	// 打开文件，如果文件不存在则创建，以追加模式写入
	file, err := os.OpenFile(crl.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 创建CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()
	var strRecord []string
	for _, value := range crl.RoundList {
		strRecord = append(strRecord, strconv.FormatFloat(value, 'f', -1, 64))
	}

	// 写入数据
	err = writer.Write(strRecord)
	if err != nil {
		panic(err)
	}

}

func (crl *CommitRoundList) Clear(epoch_size int64) {
	// 打开文件，如果文件不存在则创建，以追加模式写入
	file, err := os.OpenFile(crl.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 创建CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()
	var strRecord []string
	for _, value := range crl.RoundList {
		strRecord = append(strRecord, strconv.FormatFloat(value, 'f', -1, 64))
	}

	// 写入数据
	err = writer.Write(strRecord)
	if err != nil {
		panic(err)
	}

	crl.RoundList = make([]float64, epoch_size)
	crl.EpochSize = epoch_size

}
