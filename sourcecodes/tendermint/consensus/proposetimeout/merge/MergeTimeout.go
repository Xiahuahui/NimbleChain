package merge

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"strconv"
)

type Queue interface {
	Enqueue(item interface{})             //进入到队列
	Dequeue() interface{}                 //元素出队
	Clear()                               //将队列中的元素清空
	Len() int                             //队列的长度
	GetItemByIndex(index int) interface{} //根据元素获取索引
}

type ItemQueue struct {
	items []interface{}
}

func (q *ItemQueue) Enqueue(item interface{}) {
	q.items = append(q.items, item)
}

func (q *ItemQueue) Dequeue() interface{} {
	if len(q.items) == 0 {
		return nil
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item
}

func (q *ItemQueue) Clear() {
	q.items = []interface{}{}
}

func (q *ItemQueue) Len() int {
	return len(q.items)
}

func (q *ItemQueue) GetItemByIndex(index int) interface{} {
	return q.items[index]
}

// 下面这些参数给出breakdown就好。
var multiple float64 = 4 //这个是加上几倍的标准差
var head = 0             //这个是一个flag用来控制不同的head，只需要打印一次头
var alpha = 0.125
var beta = 0.25

// 以block的方式更新timeout
type MergeTimeout struct {
	group_id        int64   //前的group的id（和epoch的含义类似）
	RCPT            float64 //收到整个提案的时间间隔的估计置
	RCPTVAR         float64 //收到整个提案的时间的变化值（偏差）
	SampleRCPT      float64 //收到整个提案的时间间隔的采样值(真实的测量值)  //其实不是一个真是的测量值应该是group的一个统计量而已
	alpha           float64 //在计算新的RCPT时所用的低通滤波的权重
	beta            float64 //在计算新的偏差时所用到的低通滤波的权重
	timeout         float64 //新的高度timeout的估计值
	data_size       int     //统计的数据的大小
	byz_avg_time    float64 //拜占庭节点计算的avg时间
	byz_std_time    float64 //拜占庭节点计算的std时间
	byz_timeout     float64 //拜占庭节点的timeout
	CPTList         Queue   //保存一些采样数据
	CPTProposerList Queue   //保存提案的proposer列表
	SaveFile        string  //需要保存数据的文件
}

// 初始化第一个timeout
func NewMergeTimeout(group_id int64, timeout float64, save_file string) *MergeTimeout {
	mto := MergeTimeout{
		group_id:        group_id,
		RCPT:            0.0,
		RCPTVAR:         0.0,
		SampleRCPT:      0.0,
		alpha:           alpha,
		beta:            beta,
		timeout:         timeout,
		byz_timeout:     timeout, //TODO 是否可以直接这样赋值？
		CPTProposerList: &ItemQueue{},
		CPTList:         &ItemQueue{},
		SaveFile:        save_file,
	}
	file, err := os.OpenFile(mto.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	if head == 0 {
		// 创建CSV writer
		writer := csv.NewWriter(file)
		defer writer.Flush()
		var strRecord []string

		strRecord = append(strRecord, "group_id")
		strRecord = append(strRecord, "当前RCPT的值")
		strRecord = append(strRecord, "接收到整个提案的时间")
		strRecord = append(strRecord, "当前的RCPTVAR的值")
		strRecord = append(strRecord, "当前设置的timeout")
		strRecord = append(strRecord, "timeout_number")

		// 写入数据
		err = writer.Write(strRecord)
		if err != nil {
			panic(err)
		}
		head = head + 1
	}
	return &mto
}

func (mt *MergeTimeout) GetSampleRpt(node_index int) interface{} {
	var sample_time_list []float64
	for k := 0; k < mt.CPTList.Len(); k++ {
		if time_proposer, flag := mt.CPTProposerList.GetItemByIndex(k).(int); flag {
			if time_proposer == node_index {
				continue
			}
		}
		if propose_time, ok := mt.CPTList.GetItemByIndex(k).(float64); !ok {
			fmt.Println("proppose_time is not an float64")
		} else {
			sample_time_list = append(sample_time_list, propose_time)
		}

	}
	return average(sample_time_list)
}

// 这个函数应该是每个epoch换一次
func (mt *MergeTimeout) TimeOut(node_index int, is_byz bool, byz []int) (int64, int64) {
	//这个是给作恶节点建立个字典
	//TODO 这差一个统计采样数据的操作
	if sampleRpt, flag := mt.GetSampleRpt(node_index).(float64); flag {
		mt.SampleRCPT = sampleRpt
	} else {
		panic("get smaple rpt time happen error")
	}
	//当提案的列表为0时,则不更新了，直接返回timeout的值
	if mt.SampleRCPT == 0.0 {
		return int64(1000 * mt.timeout), int64(1000 * mt.byz_timeout)
	}
	byz_node_dict := make(map[string]int, len(byz))
	for _, byz_node := range byz {
		byz_node_dict[strconv.Itoa(byz_node)] = 0
	}
	mt.RCPT = (1-mt.alpha)*mt.RCPT + mt.alpha*mt.SampleRCPT //低通滤波计算下一个RCPT
	timeVar := mt.SampleRCPT - mt.RCPT
	mt.RCPTVAR = (1-mt.beta)*mt.RCPTVAR + mt.beta*(math.Abs(timeVar))
	timeout := mt.RCPT + multiple*mt.RCPTVAR
	mt.timeout = timeout
	fmt.Println("诚实节点的timeout:", mt.timeout)
	if is_byz {
		//下面这段代码是将所有没有作恶的数据拿出来也就是诚实节点的提案时间拿出来
		var byz_sample_data_list []float64
		for k := 0; k < mt.CPTList.Len(); k++ {
			proposer_index := mt.CPTProposerList.GetItemByIndex(k)
			proposer, ok := proposer_index.(int)
			if !ok {
				panic("propposer is not an int")
			}
			if _, ok1 := byz_node_dict[strconv.Itoa(proposer)]; !ok1 {
				sample_data, ok2 := mt.CPTList.GetItemByIndex(k).(float64)
				if !ok2 {
					panic("proppose_time not an float64")
				}
				byz_sample_data_list = append(byz_sample_data_list, sample_data)
			}
		}
		fmt.Println("拜占庭采样的长度", len(byz_sample_data_list), "拜占庭参与timeout估计的采样", byz_sample_data_list)
		byz_mean, byz_std := mean_and_variance(byz_sample_data_list)
		byz_timeout := byz_mean + multiple*byz_std
		mt.byz_avg_time = byz_mean
		mt.byz_std_time = byz_std
		mt.byz_timeout = byz_timeout
		if mt.byz_timeout == 0.0 {
			mt.byz_timeout = 0.5
		}
		fmt.Println("拜占庭情况下当前的均值为: ", mt.byz_avg_time, "标准差为: ", mt.byz_std_time, "propose的timeout: ", mt.byz_timeout)
	} else {
		mt.byz_avg_time = 0.0
		mt.byz_std_time = 0.0
		mt.byz_timeout = timeout
	}
	fmt.Println("返回值1", int64(1000*mt.timeout), "返回值2", int64(1000*mt.byz_timeout))
	return int64(1000 * mt.timeout), int64(1000 * mt.byz_timeout)
}

func (mt *MergeTimeout) AddTime(time float64, index int, proposer int) {
	mt.CPTList.Enqueue(time)
	mt.CPTProposerList.Enqueue(proposer)
}

func (mt *MergeTimeout) Clear(epoch_size int64) {

}

func (mt *MergeTimeout) AdjustParameters(code int) {

}

func (mt *MergeTimeout) GetSampledata() []float64 {
	//TODO应该怎样实现
	return []float64{}
}

// 这个需要记录采样的RCPT，和当前高度的timeout，以及更新的RTO有没有超时(两个数据是同一高度的时间)
func (mt *MergeTimeout) SaveSamplePerEpoch(timeout float64) {
	file, err := os.OpenFile(mt.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 创建CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()
	var strRecord []string
	strRecord = append(strRecord, strconv.FormatFloat(float64(mt.group_id), 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(mt.RCPT, 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(mt.SampleRCPT, 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(mt.RCPTVAR, 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(timeout, 'f', -1, 64))
	timout_number := 0
	var smaple_time_list []string
	for k := 0; k < mt.CPTList.Len(); k++ {
		if propose_time, ok := mt.CPTList.GetItemByIndex(k).(float64); !ok {
			fmt.Println("proppose_time is not an float64")
		} else {
			smaple_time_list = append(smaple_time_list, strconv.FormatFloat(propose_time, 'f', -1, 64))
			if propose_time > timeout {
				timout_number = timout_number + 1
			}
		}

	}
	strRecord = append(strRecord, strconv.Itoa(timout_number))
	strRecord = append(strRecord, smaple_time_list...)
	strRecord = append(strRecord, strconv.Itoa(mt.CPTList.Len()))
	err = writer.Write(strRecord)
	if err != nil {
		panic(err)
	}
	mt.group_id = mt.group_id + 1
	mt.CPTList.Clear()
	mt.CPTProposerList.Clear()
}

func average(data []float64) float64 {
	if len(data) == 0 {
		return 0.0
	}
	sum := 0.0
	for _, value := range data {
		sum += value
	}
	return sum / float64(len(data))
}

func mean_and_variance(data []float64) (float64, float64) {
	if len(data) == 0 {
		return 0.0, 0.0
	}
	mean := average(data)
	sum := 0.0
	for _, value := range data {
		sum += (value - mean) * (value - mean)
	}
	std := math.Sqrt(sum / float64(len(data)))
	return mean, std
}
