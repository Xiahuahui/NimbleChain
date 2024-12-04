package block_base

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
	cap   int
	items []interface{}
}

func (q *ItemQueue) Enqueue(item interface{}) {
	if len(q.items) == q.cap {
		q.items = q.items[1:]
	}
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

var Multiple float64 = 4 //这个是加上几倍的标准差
var head = 0             //这个是一个flag用来控制不同的head
var Alpha = 0.125
var Beta = 0.25

// 以block的方式更新timeout
type BlockProposeTimeout struct {
	height           int64   //当前区块的高度
	RCPT             float64 //收到整个提案的时间间隔的估计置
	RCPTVAR          float64 //收到整个提案的时间的变化值（偏差）
	SampleRCPT       float64 //收到整个提案的时间间隔的采样值(真实的测量值)
	alpha            float64 //在计算新的RCPT时所用的低通滤波的权重
	beta             float64 //在计算新的偏差时所用到的低通滤波的权重
	timeout          float64 //新的高度timeout的估计值
	Data_Size        int     //统计的数据的大小
	proposer         int     //统计的
	byz_avg_time     float64 //拜占庭节点计算的avg时间
	byz_std_time     float64 //拜占庭节点计算的std时间
	byz_timeout      float64 //拜占庭节点的timeout
	flag             bool    //查看当前区块高度中 是否超时
	CPTList          Queue   //保存一些采样数据
	CPTProposerList  Queue   //保存提案的proposer列表
	SaveFile         string  //需要保存数据的文件
	courrent_timeout float64
}

// 初始化第一个timeout
func NewBlockProposeTimeout(height int64, data_size int, timeout float64, save_file string) *BlockProposeTimeout {
	pto := BlockProposeTimeout{
		height:           height,
		Data_Size:        data_size,
		RCPT:             0.0,
		RCPTVAR:          0.0,
		SampleRCPT:       0.0,
		alpha:            Alpha,
		beta:             Beta,
		timeout:          timeout,
		byz_timeout:      timeout,
		courrent_timeout: timeout,
		CPTProposerList:  &ItemQueue{cap: data_size},
		CPTList:          &ItemQueue{cap: data_size},
		SaveFile:         save_file,
	}
	file, err := os.OpenFile(pto.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	if head == 0 {
		// 创建CSV writer
		writer := csv.NewWriter(file)
		defer writer.Flush()
		var strRecord []string

		strRecord = append(strRecord, "当前区块高度")
		strRecord = append(strRecord, "当前RCPT的值")
		strRecord = append(strRecord, "接收到整个提案的时间")
		strRecord = append(strRecord, "当前的RCPTVAR的值")
		strRecord = append(strRecord, "当前设置的timeout")
		strRecord = append(strRecord, "是否超时")

		// 写入数据
		err = writer.Write(strRecord)
		if err != nil {
			panic(err)
		}
		head = head + 1
	}
	return &pto
}

// 在block_based的情况下，怎样计算Timeout和byzTimeout
// 当leader节点是当前节点时，不用更新现在的timeout
func (bpt *BlockProposeTimeout) TimeOut(node_index int, is_byz bool, byz []int) (int64, int64) {
	bpt.courrent_timeout = bpt.timeout
	if bpt.SampleRCPT != 0.0 { //这个是什么意思
		if node_index == bpt.proposer { // TODO 这个可能是删除自己做leader的sample，还需要在确认
			fmt.Println("当前的数据是自己当leader节点的数据  ", "对应的高度", bpt.height)
			return int64(1000 * bpt.timeout), int64(1000 * bpt.byz_timeout)
		}
		byz_node_dict := make(map[string]int, len(byz))
		for _, byz_node := range byz {
			byz_node_dict[strconv.Itoa(byz_node)] = 0
		}
		bpt.RCPT = (1-bpt.alpha)*bpt.RCPT + bpt.alpha*bpt.SampleRCPT //低通滤波计算下一个RCPT
		timeVar := bpt.SampleRCPT - bpt.RCPT
		bpt.RCPTVAR = (1-bpt.beta)*bpt.RCPTVAR + bpt.beta*(math.Abs(timeVar))
		timeout := bpt.RCPT + Multiple*bpt.RCPTVAR
		bpt.timeout = timeout
		fmt.Println("SampleRCPT", bpt.SampleRCPT, "RCPT:", bpt.RCPT, "Var", bpt.RCPTVAR, "alpha", bpt.alpha, "bate", bpt.beta, "诚实节点的timeout:", bpt.timeout)
		// TODO 作恶时 作恶时间怎样统计
		if is_byz {
			var byz_sample_data_list []float64
			//这个是遍历实际使用的区块的时间
			for k := 0; k < bpt.CPTList.Len(); k++ {
				proposer_index := bpt.CPTProposerList.GetItemByIndex(k)
				proposer, ok := proposer_index.(int)
				if !ok {
					fmt.Println("propposer is not an int")
				}
				if _, ok1 := byz_node_dict[strconv.Itoa(proposer)]; !ok1 {
					sample_data, ok2 := bpt.CPTList.GetItemByIndex(k).(float64)
					if !ok2 {
						fmt.Println("propposer is not an int")
					}
					if proposer != node_index { // TODO 将自己是leader时的数据过滤掉
						byz_sample_data_list = append(byz_sample_data_list, sample_data)
					}
				}
			}
			fmt.Println("拜占庭采样的长度", len(byz_sample_data_list), "拜占庭参与timeout估计的采样", byz_sample_data_list)
			byz_mean, byz_std := mean_and_variance(byz_sample_data_list)
			byz_timeout := byz_mean + 3.0*byz_std
			bpt.byz_avg_time = byz_mean
			bpt.byz_std_time = byz_std
			bpt.byz_timeout = byz_timeout
			if bpt.byz_timeout == 0.0 {
				bpt.byz_timeout = 0.5
			}
			fmt.Println("拜占庭情况下当前的均值为: ", bpt.byz_avg_time, "标准差为: ", bpt.byz_std_time, "propose的timeout: ", bpt.byz_timeout)
		} else {
			bpt.byz_avg_time = 0.0
			bpt.byz_std_time = 0.0
			bpt.byz_timeout = timeout
		}
	}
	fmt.Println("返回值1", int64(1000*bpt.timeout), "返回值2", int64(1000*bpt.byz_timeout))
	return int64(1000 * bpt.timeout), int64(1000 * bpt.byz_timeout)
}

func (bpt *BlockProposeTimeout) AddTime(time float64, index int, proposer int) {
	bpt.SampleRCPT = time
	bpt.CPTList.Enqueue(time)
	bpt.CPTProposerList.Enqueue(proposer)
	bpt.proposer = proposer
	fmt.Println("当前队列的长度是: ", bpt.CPTList.Len())
	if bpt.SampleRCPT > bpt.timeout {
		bpt.flag = true
	} else {
		bpt.flag = false
	}
}

// TODO 这个怎样clear比较好
func (bpt *BlockProposeTimeout) Clear(epoch_size int64) {
	file, err := os.OpenFile(bpt.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 创建CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()
	var strRecord []string
	strRecord = append(strRecord, strconv.FormatFloat(float64(bpt.height), 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(bpt.RCPT, 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(bpt.SampleRCPT, 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(bpt.RCPTVAR, 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(bpt.courrent_timeout, 'f', -1, 64))
	if bpt.flag {
		strRecord = append(strRecord, strconv.Itoa(1))
	} else {
		strRecord = append(strRecord, strconv.Itoa(0))
	}
	// 写入数据
	err = writer.Write(strRecord)
	if err != nil {
		panic(err)
	}
	bpt.height = bpt.height + 1
}

func (bpt *BlockProposeTimeout) AdjustParameters(code int) {
	step := 0.1
	if code == 0 {
		fmt.Println("code_code:", code)
		bpt.beta = bpt.beta + step
		if bpt.beta >= 1.0 {
			bpt.beta = 0.99
		}
	} else if code == 1 {
		fmt.Println("code_code:", code)
		bpt.beta = bpt.beta + step
		if bpt.beta <= 0.0 {
			bpt.beta = 0.01
		}
	} else if code == 2 {
		fmt.Println("test")
	} else {
		panic("The value of the code is unknown.")
	}

	//else if code == 1 {
	//	bpt.alpha = bpt.alpha + step
	//} else if code == 2 {
	//	bpt.beta = bpt.beta - step
	//} else if code == 3 {
	//	bpt.alpha = bpt.alpha - step
	//} else if code == 10 {
	//	fmt.Println("test")
	//} else {
	//	panic("The value of the code is unknown.")
	//}
}

func (bpt *BlockProposeTimeout) GetSampledata() []float64 {
	//TODO应该怎样实现
	return []float64{}
}

// 这个需要记录采样的RCPT，和当前高度的timeout，以及更新的RTO有没有超时(两个数据是同一高度的时间)
func (bpt *BlockProposeTimeout) SaveSamplePerEpoch(timeout float64) {
	file, err := os.OpenFile(bpt.SaveFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 创建CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()
	var strRecord []string
	strRecord = append(strRecord, strconv.FormatFloat(float64(bpt.height), 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(bpt.RCPT, 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(bpt.SampleRCPT, 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(bpt.RCPTVAR, 'f', -1, 64))
	strRecord = append(strRecord, strconv.FormatFloat(timeout, 'f', -1, 64))
	if bpt.flag {
		strRecord = append(strRecord, strconv.Itoa(1))
	} else {
		strRecord = append(strRecord, strconv.Itoa(0))
	}
	// 写入数据
	err = writer.Write(strRecord)
	if err != nil {
		panic(err)
	}
	bpt.height = bpt.height + 1
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
