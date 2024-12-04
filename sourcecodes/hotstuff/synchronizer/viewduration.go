package synchronizer

import (
	"encoding/csv"
	"fmt"
	"github.com/relab/hotstuff"
	"os"
	"strconv"
	"time"
)

type pdt_bct_time struct {
	start_id   hotstuff.ID
	end_id     hotstuff.ID
	start_time time.Time
	end_time   time.Time
}

var record_time pdt_bct_time
var start_time = make(map[hotstuff.ID][]time.Time)

// ViewDuration determines the duration of a view.
// The view synchronizer uses this interface to set its timeouts.
type ViewDuration interface {
	// Duration returns the duration that the next view should last.
	Duration() time.Duration
	// ViewStarted is called by the synchronizer when starting a new view.
	ViewStarted()
	// ViewSucceeded is called by the synchronizer when a view ended successfully.
	ViewSucceeded(view hotstuff.View,proposer hotstuff.ID)
	// ViewTimeout is called by the synchronizer when a view timed out.
	ViewTimeout(view hotstuff.View)
	//log bct and pcd and timeout
	ViewLog(view hotstuff.View)
	//return the timeout
	GetTimeout() float64
}

// NewViewDuration returns a ViewDuration that approximates the view duration based on durations of previous views.
// sampleSize determines the number of previous views that should be considered.
// startTimeout determines the view duration of the first views.
// When a timeout occurs, the next view duration will be multiplied by the multiplier.
func NewViewDuration(id uint32, sampleSize uint64, startTimeout, maxTimeout, multiplier float64) ViewDuration {
	var delay_list = []uint32{}
	var is_byz =false


	for _, i := range delay_list {
		if id == i{
			is_byz = true
			break
		}
	}
	return &viewDuration{
		limit:        sampleSize,
		mean:         startTimeout,
		timeout:      startTimeout,
		max:          maxTimeout,
		mul:          multiplier,
		LastTimeout:  startTimeout,
		a:            0.875,
		b:            0.75,
		k:            4,
		count:        0,
		ID:           id,
		m2:           20,
		height:		  1,
		is_byz: is_byz,
		is_crash:false,
		PDTStartTime: time.Now(),
		service:      InitPCC(int64(id), 10000000, 1000000, 1, 3000, is_byz, false, 30, "v3", 0, "mal-para", -3.0),
	}
}

// viewDuration uses statistics from previous views to guess a good value for the view duration.
// It only takes a limited amount of measurements into account.
type viewDuration struct {
	mul          float64   // on failed views, multiply the current mean by this number (should be > 1)
	limit        uint64    // how many measurements should be included in mean
	count        uint64    // total number of measurements
	startTime    time.Time // the start time for the current measurement
	mean         float64   // the mean view duration
	m2           float64   // sum of squares of differences from the mean
	max          float64   // upper bound on view timeout
	a            float64   // alpha
	b            float64   // beta
	k            float64   // 偏差
	BCT          float64
	is_byz bool
	is_crash bool
	height 		int64
	timeout      float64
	LastTimeout  float64
	service      PCCTimeout
	PDT          float64
	LastBCT float64
	LastPDT float64
	ID           uint32
	PDTStartTime time.Time
	prevM2 float64
}

var cost float64
// //NimbleChain
// func (v *viewDuration) ViewSucceeded(view hotstuff.View,proposer hotstuff.ID) {
// 	if v.startTime.IsZero() {
// 		return
// 	}
// 	duration := float64(time.Since(v.startTime)) / float64(time.Millisecond)
// 	if duration < 100 {return}
// 	v.count++
// 	v.LastTimeout = v.timeout
// 	v.LastBCT = v.BCT
// 	v.LastPDT = v.PDT
// 	v.BCT = float64(time.Since(v.startTime)) / float64(time.Millisecond)
// 	v.PDT = float64(time.Since(v.PDTStartTime)) / float64(time.Millisecond)
// 	t := time.Now()
// 	if v.count<30{
// 		BCT:=(v.LastBCT + v.BCT)/2
// 		PDT:=(v.LastPDT + v.PDT)/2
// 		var isLeader=false
// 		if uint32(proposer)==v.ID{
// 			isLeader=true
// 		}
// 		_, _, v.timeout, _ = v.service.RunRLJ(v.height, 0, BCT, PDT,v.is_crash,isLeader,v.is_byz, 0,0, 0, 0, 0,int64(proposer))
// 		v.height++
// 		v.ViewLog(view)
// 	}else if v.count%2 == 0{
// 	//}else {
// 			go func(v *viewDuration,proposer hotstuff.ID){
// 				BCT:=(v.LastBCT + v.BCT)/2
// 				PDT:=(v.LastPDT + v.PDT)/2
// 				var isLeader=false
// 				if uint32(proposer)==v.ID{
// 					isLeader=true
// 				}
// 				_, _, v.timeout, _ = v.service.RunRLJ(v.height, 0, BCT, PDT,v.is_crash,isLeader,v.is_byz, 0,0, 0, 0, 0,int64(proposer))
// 				v.height++
// 				v.ViewLog(view)
// 		}(v,proposer)
// 	}else{
// 		v.ViewLog(view)
// 		v.timeout = v.LastTimeout
// 	}
// 	cost = float64(time.Since(t))/float64(time.Millisecond)
// 	v.PDTStartTime = time.Now()
// }

// //jacobson
func (v *viewDuration) ViewSucceeded(view hotstuff.View,proposer hotstuff.ID) {
	if v.startTime.IsZero() {
		return
	}
	duration := float64(time.Since(v.startTime)) / float64(time.Millisecond)
	if duration < 30 {return}
	v.LastTimeout = v.timeout
	v.BCT = duration
	v.PDT = float64(time.Since(v.PDTStartTime)) / float64(time.Millisecond)
	v.PDTStartTime = time.Now()
	v.mean = v.a*v.mean + (1-v.a)*duration
	if v.mean > duration {
		v.m2 = v.b*v.m2 + (1-v.b)*(v.mean-duration)
	} else {
		v.m2 = v.b*v.m2 + (1-v.b)*(duration-v.mean)
	}
	v.count++
	v.LastTimeout = v.timeout
	if v.count < 30000{
		v.timeout = 350
	}else{
		v.timeout = v.mean + v.k * v.m2
	}
	v.ViewLog(view)
}

//EMA
// func (v *viewDuration) ViewSucceeded(view hotstuff.View,proposer hotstuff.ID){
// 	if v.startTime.IsZero() {
// 		return
// 	}
// 	duration := float64(time.Since(v.startTime)) / float64(time.Millisecond)
// 	if duration < 30 {return}
// 	v.BCT = duration
// 	var a = 0.9
// 	v.PDT = float64(time.Since(v.PDTStartTime)) / float64(time.Millisecond)
// 	v.PDTStartTime = time.Now()
// 	v.timeout = a * v.LastTimeout + (1-a) * duration
// 	v.LastTimeout = v.timeout
// 	v.ViewLog(view) 
// }


// ViewTimeout should be called when a view timeout occurred. It will multiply the current mean by 'mul'.
func (v *viewDuration) ViewTimeout(view hotstuff.View) {
	v.timeout *= v.mul
	filePath := strconv.FormatUint(uint64(v.ID), 10)+".csv"
	var strList []string
	strList = append(strList, "View")
	strList = append(strList, strconv.FormatUint(uint64(view), 10))
	strList = append(strList, "BCT")
	strList = append(strList, strconv.FormatFloat(v.BCT, 'f', -1, 64))
	strList = append(strList, "PDT")
	strList = append(strList, strconv.FormatFloat(v.PDT, 'f', -1, 64))
	strList = append(strList, "LastTimeout")
	strList = append(strList, strconv.FormatFloat(v.LastTimeout, 'f', -1, 64))
	strList = append(strList, "Timeout")
	strList = append(strList, strconv.FormatFloat(v.timeout, 'f', -1, 64))
	strList = append(strList, "Timeout-PDT")
	strList = append(strList, strconv.FormatFloat(v.LastTimeout - v.PDT, 'f', -1, 64))
	strList = append(strList, "timeout happen")
	err := WriteCSV(filePath, strList)
	if err != nil {
		return
	}
}

// ViewStarted records the start time of a view.
func (v *viewDuration) ViewStarted() { 
	duration := float64(time.Since(v.startTime)) / float64(time.Millisecond)
	if duration < 30{return}
	v.startTime = time.Now()
}

func (v *viewDuration) Duration() time.Duration {
	return time.Duration((v.timeout) * float64(time.Millisecond))
}

func (v *viewDuration) ViewLog(view hotstuff.View) {
	if v.startTime.IsZero() {
		return
	}
	filePath := strconv.FormatUint(uint64(v.ID), 10)+".csv"
	var strList []string
	strList = append(strList, "View")
	strList = append(strList, strconv.FormatUint(uint64(view), 10))
	strList = append(strList, "BCT")
	strList = append(strList, strconv.FormatFloat(v.BCT, 'f', -1, 64))
	strList = append(strList, "PDT")
	strList = append(strList, strconv.FormatFloat(v.PDT, 'f', -1, 64))
	strList = append(strList, "LastTimeout")
	strList = append(strList, strconv.FormatFloat(v.LastTimeout, 'f', -1, 64))
	strList = append(strList, "Timeout")
	strList = append(strList, strconv.FormatFloat(v.timeout, 'f', -1, 64))
	strList = append(strList, "Timeout-PDT")
	strList = append(strList, strconv.FormatFloat(v.LastTimeout - v.PDT, 'f', -1, 64))
	strList = append(strList, "no timeout happen")
	err := WriteCSV(filePath, strList)
	if err != nil {
		return
	}
}
//writing files
func WriteCSV(filePath string, data []string) error {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开文件: %v", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write(data); err != nil {
		return fmt.Errorf("写入数据失败: %v", err)
	}

	return nil
}
func (v *viewDuration) SetPDT(PDT float64) {
	v.PDT = PDT
}
func (v *viewDuration) GetTimeout() float64{
	return v.timeout
}