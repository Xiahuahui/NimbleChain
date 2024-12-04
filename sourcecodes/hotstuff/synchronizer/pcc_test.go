package synchronizer

import "testing"

func TestGrpc(t *testing.T) {
	mi := int64(10000000)
	smi := int64(1000000)
	epoch_size := int64(1)
	timeout_value := int64(3000)
	is_byz := false
	is_crash := false
	cold_start_phase := int64(30)
	version := "v3"            //v1,v2,v3
	delay_time := float64(0.0) //0,1000
	method := "Fixed"          //Fixed，Jac，mal-para
	para_a := -3.0             // -1,-2,-3

	service := InitPCC(0, mi, smi, epoch_size, timeout_value, is_byz, is_crash, cold_start_phase, version, delay_time, method, para_a)

	for i := 0; i < 100; i++ {
		height := int64(i)
		delay_time = 0
		PDT := float64(500.0)
		is_crash = false
		service.RunRLJ(height, delay_time, PDT, is_crash, 0.0, 0.0, 0.0, 0.0, 300.0)
	}

}
