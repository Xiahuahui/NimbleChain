package consensus

//
//import (
//	"fmt"
//	"math"
//	"testing"
//)
//
//func TestProposeTimeList_Timeout(t *testing.T) {
//	plt := makeProposeTimeList(5, "test.csv")
//	plt.AddTime(1, 0)
//	plt.AddTime(2, 1)
//	plt.AddTime(0, 2)
//	fmt.Println("proposetimeout", plt.TimeOut())
//	plt.SaveSamplePerEpoch(6.9)
//	ltl := makeLatencyTimeList(5, "latency.csv")
//	ltl.AddTime(1, 0)
//	ltl.SaveSamplePerEpoch(66)
//
//}
//
//func TestProposeTimeOut_TimeOut(t *testing.T) {
//	Timeout := 6.0
//	pto := MakeInitializationPTO(1, Timeout, "test.csv") //初始化
//	RTTime := 0.0
//	RTTVar := 0.0
//
//	fmt.Println("proposetimeout", pto.TimeOut())
//	for i := 1; i < 10; i++ {
//		sampleRTTime := float64(i) / 10
//		pto.AddTime(sampleRTTime, 0)
//		pto.TimeOut()
//		//fmt.Println("proposetimeout", pto.TimeOut())
//		RTTime = (1-alpha)*RTTime + (alpha * sampleRTTime)
//		//fmt.Println("alpha*time1", RTTime)
//		RTTVar = (1-beta)*RTTVar + beta*(math.Abs(sampleRTTime-RTTime))
//		//fmt.Println("beta*time2", VarTime)
//		pto.SaveSamplePerEpoch(Timeout)
//		Timeout = RTTime + 4*RTTVar
//		fmt.Println("rtt: ", RTTime, "smapleRTT", sampleRTTime, "rttvar:", RTTVar, "proposetimeout", Timeout)
//
//	}
//}
