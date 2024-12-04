package merge

import (
	"testing"
)

func TestItemQueue_GetSampleRpt(t *testing.T) {
	q := ItemQueue{}
	q.Enqueue(1.0)
	q.Enqueue(2.0)
	q.Enqueue(3.0)
	//fmt.Println(q.GetSampleRpt()) //这块其实就是统计一个平均值

}
