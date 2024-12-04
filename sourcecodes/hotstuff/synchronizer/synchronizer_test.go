package synchronizer_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/relab/hotstuff"

	"github.com/golang/mock/gomock"
	"github.com/relab/hotstuff/internal/mocks"
	"github.com/relab/hotstuff/internal/testutil"
	"github.com/relab/hotstuff/modules"
	"github.com/relab/hotstuff/synchronizer"
)

func TestAdvanceViewQC(t *testing.T) {
	const n = 4
	ctrl := gomock.NewController(t)
	builders := testutil.CreateBuilders(t, ctrl, n)
	s := synchronizer.New(testutil.FixedTimeout(1000))
	hs := mocks.NewMockConsensus(ctrl)
	builders[0].Add(s, hs)

	hl := builders.Build()
	signers := hl.Signers()

	block := hotstuff.NewBlock(
		hotstuff.GetGenesis().Hash(),
		hotstuff.NewQuorumCert(nil, 0, hotstuff.GetGenesis().Hash()),
		"foo",
		1,
		2,
	)

	var blockChain modules.BlockChain
	hl[0].Get(&blockChain)

	blockChain.Store(block)
	qc := testutil.CreateQC(t, block, signers)
	// synchronizer should tell hotstuff to propose
	hs.EXPECT().Propose(gomock.AssignableToTypeOf(hotstuff.NewSyncInfo()))

	s.AdvanceView(hotstuff.NewSyncInfo().WithQC(qc), false)

	if s.View() != 2 {
		t.Errorf("wrong view: expected: %v, got: %v", 2, s.View())
	}
}

func TestAdvanceViewTC(t *testing.T) {
	const n = 4
	ctrl := gomock.NewController(t)
	builders := testutil.CreateBuilders(t, ctrl, n)
	s := synchronizer.New(testutil.FixedTimeout(100))
	hs := mocks.NewMockConsensus(ctrl)
	builders[0].Add(s, hs)

	hl := builders.Build()
	signers := hl.Signers()

	tc := testutil.CreateTC(t, 1, signers)

	// synchronizer should tell hotstuff to propose
	hs.EXPECT().Propose(gomock.AssignableToTypeOf(hotstuff.NewSyncInfo()))

	s.AdvanceView(hotstuff.NewSyncInfo().WithTC(tc), false)

	if s.View() != 2 {
		t.Errorf("wrong view: expected: %v, got: %v", 2, s.View())
	}
}

var head = 0

func TestWriteFile(t *testing.T) {
	// CSV 文件路径
	filePath := "data.csv"
	ClearCSV(filePath)
	// 示例数据
	data := []string{"viewId1", "bct1", "pdt1", "timeout1"}

	// 写入数据
	if head == 0 {
		if err := WriteCSV(filePath, data); err != nil {
			head = head + 1
			fmt.Println("错误:", err)
		} else {
			fmt.Println("数据已成功写入 CSV 文件")
		}
	}
	num := uint32(123)
	str := strconv.FormatUint(uint64(num), 10)
	var str_list []string
	str_list = append(str_list, str)
	WriteCSV(filePath, str_list)

}
func ClearCSV(filePath string) error {
	// 打开文件，清空内容
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("无法打开文件: %v", err)
	}
	defer file.Close()

	// 文件内容已被清空
	return nil
}

// WriteCSV 写入一行数据到 CSV 文件
func WriteCSV(filePath string, data []string) error {
	// 打开文件，如果不存在则创建
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开文件: %v", err)
	}
	defer file.Close()

	// 创建 CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入数据
	if err := writer.Write(data); err != nil {
		return fmt.Errorf("写入数据失败: %v", err)
	}

	return nil
}
