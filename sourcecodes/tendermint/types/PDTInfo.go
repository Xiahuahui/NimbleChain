package types

import tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

type PDTInfo struct {
	height    int64
	pdt_value float64
	bdt_value float64
}

func MakePDTInfo(height int64, pdt_value float64, bct_value float64) *PDTInfo {
	return &PDTInfo{height: height, pdt_value: pdt_value, bdt_value: bct_value}
}

func (pdt_info *PDTInfo) GetValue() (int64, float64, float64) {
	return pdt_info.height, pdt_info.pdt_value, pdt_info.bdt_value
}

func (pdt_info *PDTInfo) ToProto() *tmproto.PDTInfo {
	return &tmproto.PDTInfo{
		Height:  pdt_info.height,
		PdtInfo: pdt_info.pdt_value,
		BctInfo: pdt_info.bdt_value,
	}
}

func PDTInfoFromProto(ppdt *tmproto.PDTInfo) (*PDTInfo, error) {
	return &PDTInfo{height: ppdt.Height, pdt_value: ppdt.PdtInfo, bdt_value: ppdt.BctInfo}, nil

}
