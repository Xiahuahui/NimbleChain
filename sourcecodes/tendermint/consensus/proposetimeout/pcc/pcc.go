package pcc

import (
	"context"
	"fmt"
	tm_log "github.com/tendermint/tendermint/libs/log"
	pb "github.com/tendermint/tendermint/proto/tendermint/consensus"
	"google.golang.org/grpc"
	"log"
)

type PCCTimeout struct {
	MI            int64               //每个MI的大小
	SmallMI       int64               //每个小的MI的大小, 用来更新alpha和beta
	SizeNumber    int64               //当前学习需要的MI的个数
	ExcutedNumber int64               //已经执行了几个MI
	TimeoutValue  int64               //
	DelayTime     float64             //延误时间
	SizeList      []int64             //每个MI具体的block大小
	GrpcService   pb.PccServiceClient //提供调用远程调用Grpc的句柄
	Req           pb.RunRequest       //在执行run的请求  应给是初始化下一次的请求
}

const (
	address = "localhost:50051"
)

// 初始化pcc
func InitPCC(mi int64, smi int64, epoch_size int64, timeout_value int64, is_byz bool, is_crash bool, cold_start_phase int64, version string, delay_time float64, method string, para_a float64) PCCTimeout {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	//defer conn.Close()
	if mi > int64(10000000) {
		p := PCCTimeout{MI: mi, SmallMI: smi, ExcutedNumber: 0, SizeNumber: 1, SizeList: []int64{epoch_size}, Req: pb.RunRequest{MIData: make([]*pb.MIData, 1)}}
		p.Req.MIData = []*pb.MIData{}
		var mi_data pb.MIData
		p.Req.MIData = append(p.Req.MIData, &mi_data)
		return p
	}
	c := pb.NewPccServiceClient(conn)
	ps := PCCTimeout{GrpcService: c, SizeList: []int64{}, ExcutedNumber: int64(0)}
	fmt.Println("the cold_start_phase len's is: ", cold_start_phase)
	res, _ := ps.GrpcService.InitPCC(context.Background(), &pb.InitRequest{MI: mi, SMI: smi, EpochSize: epoch_size, InitTimeout: timeout_value, IsByz: is_byz, IsCrash: is_crash, ColdStartPhase: cold_start_phase, Version: version, DelayTime: delay_time, Method: method, ParaA: para_a})
	ps.MI = res.MI
	ps.SmallMI = smi
	ps.SizeNumber = res.MiOfNumber
	ps.TimeoutValue = res.InitTimeout
	ps.DelayTime = res.DelayTime
	for _, size := range res.EpochSizeList {
		ps.SizeList = append(ps.SizeList, size)
		var mi_data pb.MIData
		ps.Req.MIData = append(ps.Req.MIData, &mi_data)
	}
	return ps
}

// 调用后台的pcc
func (pcct *PCCTimeout) RunPCC() {
	if pcct.MI > int64(10000000) {
		return
	}
	res, err := pcct.GrpcService.RunPcc(context.Background(), &pcct.Req)
	if err != nil {
		log.Fatalf("Error running PCC: %v", err)
	}
	pcct.ExcutedNumber = 0
	pcct.SizeNumber = res.MiOfNumber
	pcct.MI = res.MI
	pcct.TimeoutValue = res.InitTimeout
	pcct.SizeList = pcct.SizeList[:0]
	pcct.Req.MIData = []*pb.MIData{}
	for _, size := range res.EpochSizeList {
		pcct.SizeList = append(pcct.SizeList, size)
		var mi_data pb.MIData
		pcct.Req.MIData = append(pcct.Req.MIData, &mi_data)
	}
}

// GetSMIDataFromMIData( )下面的函数是将MI中的摘取一部分去做smallMI的处理，其实数据都应该一样的
// Input
// Output
func GetSMIDataFromMIData(midata *pb.MIData, smi_index int64, smi int64) *pb.MIData {
	//fmt.Println("the smi_index is: ", len(midata.Height), smi_index*smi, (smi_index+1)*smi-1)
	smidata := pb.MIData{}
	smidata.Height = append(smidata.Height, midata.Height[smi_index*smi:(smi_index+1)*smi]...)
	smidata.IsLeader = append(smidata.IsLeader, midata.IsLeader[smi_index*smi:(smi_index+1)*smi]...)
	smidata.ProposerList = append(smidata.ProposerList, midata.ProposerList[smi_index*smi:(smi_index+1)*smi]...)
	smidata.IsByzNode = append(smidata.IsByzNode, midata.IsByzNode[smi_index*smi:(smi_index+1)*smi]...)
	smidata.ActaulTimeoutList = append(smidata.ActaulTimeoutList, midata.ActaulTimeoutList[smi_index*smi:(smi_index+1)*smi]...)
	fmt.Println("smidata.ActaulTimeoutList", smidata.ActaulTimeoutList)
	smidata.PredictTimeoutList = append(smidata.PredictTimeoutList, midata.PredictTimeoutList[smi_index*smi:(smi_index+1)*smi]...)
	smidata.ByzTimeoutList = append(smidata.ByzTimeoutList, midata.ByzTimeoutList[smi_index*smi:(smi_index+1)*smi]...)
	smidata.Round = append(smidata.Round, midata.Round[smi_index*smi:(smi_index+1)*smi]...)
	smidata.CostList = append(smidata.CostList, midata.CostList[smi_index*smi:(smi_index+1)*smi]...)
	smidata.LatencyList = append(smidata.LatencyList, midata.LatencyList[smi_index*smi:(smi_index+1)*smi]...)
	smidata.TxNumList = append(smidata.TxNumList, midata.TxNumList[smi_index*smi:(smi_index+1)*smi]...)
	//TODO 下面两个参数要重新调整一下
	//int64 timeout_value = 9;
	actaul_latency := 0.0
	txnum := 0
	//fmt.Println("the smi length is: ", len(smidata.LatencyList), len(smidata.CostList))
	for i := int64(0); i < smi; i++ {
		actaul_latency = actaul_latency + smidata.LatencyList[int(i)] - float64(smidata.CostList[int(i)].Discretionary)/1000
		txnum = txnum + int(smidata.TxNumList[int(i)])
	}
	smidata.Tps = float64(txnum) / float64(actaul_latency)
	return &smidata
}

// 拿到当前高度所用到的数据
func GetAuxiliarydataofHeight(midata *pb.MIData, height int64, delay_time float64, deliver_time float64) *pb.HeightData {
	heightdata := pb.HeightData{}
	height_index := len(midata.Height) - 1
	heightdata.Height = height
	if height != midata.Height[height_index] {
		panic("the height error")
	}
	heightdata.IsLeader = midata.IsLeader[height_index]
	heightdata.Proposer = midata.ProposerList[height_index]
	heightdata.IsByzNode = midata.IsByzNode[height_index]
	heightdata.ActaulTimeout = midata.ActaulTimeoutList[height_index]
	heightdata.PredictTimeout = midata.PredictTimeoutList[height_index]
	heightdata.ByzTimeout = midata.ByzTimeoutList[height_index]
	heightdata.Round = midata.Round[height_index]
	heightdata.Cost = midata.CostList[height_index]
	heightdata.Latency = midata.LatencyList[height_index]
	heightdata.TxNum = midata.TxNumList[height_index]
	actaul_latency := heightdata.Latency - float64(heightdata.Cost.Discretionary)/1000
	heightdata.Tps = float64(heightdata.TxNum) / float64(actaul_latency)
	heightdata.DelayTime = delay_time
	heightdata.DeliverTime = deliver_time

	return &heightdata
}

// TODO 当前默认MI是SmallMI的整数倍。
func (pcct *PCCTimeout) RunSpc(smi_index int64) int64 {
	spc_req := pb.RunSpcRequest{}
	//将当前smallMi中的数据发送给后台，这个需要executedNumber
	spc_req.MIData = GetSMIDataFromMIData(pcct.Req.MIData[pcct.ExcutedNumber], smi_index, pcct.SmallMI)

	//fmt.Println("spc_req: ", *spc_req.MIData)
	res, err := pcct.GrpcService.RunSpc(context.Background(), &spc_req)
	if err != nil {
		log.Fatalf("Error running SPC: %v", err)
	}
	return res.Code
}

func (pcct *PCCTimeout) Clear() {
	pcct.Req.MIData[pcct.ExcutedNumber] = &pb.MIData{}
	pcct.ExcutedNumber = pcct.ExcutedNumber - 1
}

// TODO 做一些清除工作
func (pcct *PCCTimeout) RunRLJ(height int64, delay_time float64, PDT float64, is_crash bool, logger tm_log.Logger, min float64, max float64, avg float64, std float64, deliver_time float64) (int64, float64, float64, float64) {
	rlj_req := pb.RunRLJRequest{}
	rlj_req.HeightData = GetAuxiliarydataofHeight(pcct.Req.MIData[pcct.ExcutedNumber], height, delay_time, deliver_time)
	rlj_req.HeightData.Pdt = PDT
	rlj_req.HeightData.IsCrashNode = is_crash
	rlj_req.HeightData.Min = min
	rlj_req.HeightData.Max = max
	rlj_req.HeightData.Avg = avg
	rlj_req.HeightData.Std = std
	logger.Info("deliver_time", deliver_time)
	logger.Info("before call frpc")
	res, err := pcct.GrpcService.RunRLJ(context.Background(), &rlj_req)
	logger.Info("after call grpc")
	if err != nil {
		log.Fatalf("Error running SPC: %v", err)
	}

	timeout := res.Timeout
	byz_timeout := res.ByzTimeout
	td := res.DelayTime
	return res.Code, timeout, byz_timeout, td
}
