package epoch_base

import (
	"encoding/csv"
	"fmt"
	"github.com/tendermint/tendermint/consensus/proposetimeout"
	"math"
	"os"
	"strconv"
)

// 以epoch方式来更新timeout
var Multiple float64 = 3 //这个是加上几倍的标准差
var head = 0

type EpochProposeTimeout struct {
	CompleteBlockTime []float64 // 采样的提案时间
	ProposerList      []int     //提案节点的列表
	EpochSize         int64     // epoch的大小
	avgTime           float64   // 诚实节点的avgTime
	stdTime           float64   // 诚实节点的stdTime
	timeout           float64   // 诚实节点的timeout
	byz_avg_time      float64   // 拜占庭节点的avgTime
	byz_std_time      float64   // 拜占庭节点的stdTime
	byz_timeout       float64   // 拜占庭节点的timeout
	SaveFile          string    // 保存log文件的路径
}

var _ proposetimeout.Timeout = &EpochProposeTimeout{}

// 制造新的EpochProposeTimeout 并且初始化csv文件
func NewEpochProposeTimeout(epoch_size int64, save_file string) *EpochProposeTimeout {
	ept := EpochProposeTimeout{
		CompleteBlockTime: make([]float64, epoch_size),
		ProposerList:      make([]int, epoch_size),
		EpochSize:         epoch_size,
		SaveFile:          save_file,
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
		}
		strRecord = append(strRecord, "均值")
		strRecord = append(strRecord, "标准差")
		strRecord = append(strRecord, "下一epoch的timeout")
		strRecord = append(strRecord, "byz均值")
		strRecord = append(strRecord, "byz标准差")
		strRecord = append(strRecord, "byz下一epoch的timeout")
		strRecord = append(strRecord, "超时的个数")
		strRecord = append(strRecord, "超时比例")
		// 写入数据
		err = writer.Write(strRecord)
		if err != nil {
			panic(err)
		}
		head = head + 1
	}
	return &ept
}

// Timeout 每个epoch计算一次下一个epoch的timeout
// Input  ：isbyz当前节点是否是拜占庭节点，拜占庭的节点的列表
// Output ：两种情况下的timeout
// TODO 怎样能够将自己做leader的时间去掉  这个先不考虑
func (ept *EpochProposeTimeout) TimeOut(node_index int, isbyz bool, byz []int) (int64, int64) {
	//当串谋的情况
	byz_node_dict := make(map[string]int, len(byz))
	for _, byz_node := range byz {
		byz_node_dict[strconv.Itoa(byz_node)] = 0
	}
	var sample_data_list []float64
	for _, sample_data := range ept.CompleteBlockTime {
		sample_data_list = append(sample_data_list, sample_data)
	}
	fmt.Println("采样的长度", len(sample_data_list), "参与timeout估计的采样", sample_data_list)
	mean, std := mean_and_variance(sample_data_list)
	timeout := mean + Multiple*std
	ept.avgTime = mean
	ept.stdTime = std
	ept.timeout = timeout
	fmt.Println("当前的均值为: ", ept.avgTime, "标准差为: ", ept.stdTime, "proposetimeout: ", ept.timeout)
	if isbyz {
		var byz_sample_data_list []float64
		for k := 0; k < int(ept.EpochSize); k++ {
			proposer := ept.ProposerList[k]
			if _, ok := byz_node_dict[strconv.Itoa(proposer)]; !ok {
				byz_sample_data_list = append(byz_sample_data_list, ept.CompleteBlockTime[k])
			}
		}
		fmt.Println("拜占庭采样的长度", len(byz_sample_data_list), "拜占庭参与timeout估计的采样", byz_sample_data_list)
		byz_mean, byz_std := mean_and_variance(byz_sample_data_list)
		byz_timeout := byz_mean + Multiple*byz_std
		ept.byz_avg_time = byz_mean
		ept.byz_std_time = byz_std
		ept.byz_timeout = byz_timeout
		fmt.Println("拜占庭情况下当前的均值为: ", ept.byz_avg_time, "标准差为: ", ept.byz_std_time, "proposetimeout: ", ept.byz_timeout)
	} else {
		ept.byz_avg_time = mean
		ept.byz_std_time = std
		ept.byz_timeout = timeout
	}

	return int64(ept.timeout * 1000), int64(ept.byz_timeout * 1000)
}
func (ept *EpochProposeTimeout) SaveSamplePerEpoch(timeout float64) {
	// 打开文件，如果文件不存在则创建，以追加模式写入
	fmt.Println("当前需要比较的timeout", timeout)
	timeout_number := 0 //记录的是超时的个数
	file, err := os.OpenFile(ept.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 创建CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()
	var strRecord []string
	for _, value := range ept.CompleteBlockTime {
		if value > timeout {
			timeout_number = timeout_number + 1
		}
		strRecord = append(strRecord, strconv.FormatFloat(value, 'f', -1, 64))
	}
	//mean, std := mean_and_variance(plt.CompleteBlockTime)
	fmt.Println("当前的均值为: ", ept.avgTime, "方差为: ", ept.stdTime, "proposetimeout: ", ept.timeout)
	fmt.Println("拜占庭情况下当前的均值为: ", ept.byz_avg_time, "标准差为: ", ept.byz_std_time, "proposetimeout: ", ept.byz_timeout)
	strRecord = append(strRecord, strconv.FormatFloat(ept.avgTime, 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(ept.stdTime, 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(ept.timeout, 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(ept.byz_avg_time, 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(ept.byz_std_time, 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(ept.byz_timeout, 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(float64(timeout_number), 'f', -1, 64))
	retio := float64(timeout_number) / float64(ept.EpochSize)
	strRecord = append(strRecord, strconv.FormatFloat(retio, 'f', -1, 64))

	// 写入数据
	err = writer.Write(strRecord)
	if err != nil {
		panic(err)
	}

}

// 这个要和spc理论要匹配上
func (ept *EpochProposeTimeout) AdjustParameters(code int) {
	step := int64(1)
	if code == 0 {
		ept.EpochSize = ept.EpochSize + step
	} else if code == 1 {
		ept.EpochSize = ept.EpochSize - step
	} else if code == 10 {
		fmt.Println("test")
	} else {
		panic("The value of the code is unknown.")
	}
}

func (ept *EpochProposeTimeout) AddTime(time float64, index int, proposer int) {
	ept.CompleteBlockTime[index] = time
	ept.ProposerList[index] = proposer
	fmt.Println("当前提案时间的列表", ept.CompleteBlockTime)
}

func (ept *EpochProposeTimeout) Clear(epoch_size int64) {
	file, err := os.OpenFile(ept.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 创建CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()
	var strRecord []string
	for _, value := range ept.CompleteBlockTime {
		if value != 0.0 {
			strRecord = append(strRecord, strconv.FormatFloat(value, 'f', -1, 64))
		}

	}

	// 写入数据
	err = writer.Write(strRecord)
	if err != nil {
		panic(err)
	}

	ept.CompleteBlockTime = make([]float64, epoch_size)
	ept.ProposerList = make([]int, epoch_size)
	ept.EpochSize = epoch_size
}

func (ept *EpochProposeTimeout) GetSampledata() []float64 {
	var timelist []float64
	for _, time_data := range ept.CompleteBlockTime {
		if time_data != 0.0 {
			timelist = append(timelist, time_data)
		}
	}
	return timelist
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
