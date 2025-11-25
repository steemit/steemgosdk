package steemgosdk

import (
	"testing"
)

func TestGetBlocks(t *testing.T) {
	client := GetClient("https://api.steemit.com")
	api := client.GetAPI()
	// dgp, err := api.GetDynamicGlobalProperties()
	// t.Errorf("test: %+v, err: %+v", dgp, err)
	// block, err := api.GetBlock(20221123)
	// for _, tr := range block.Transactions {
	// 	for _, op := range tr.Operations {
	// 		fmt.Printf("op: %+v\n", op.Type() == "vote")
	// 	}
	// }
	// t.Errorf("test: %+v, err: %+v", block, err)
	blocks, err := api.GetBlocks(10000000, 10000100)
	if len(blocks) != 100 {
		t.Errorf("GetBlocks unexpected length, err: %+v", err)
	}
}
