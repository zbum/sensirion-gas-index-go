package gasindex_test

import (
	"fmt"

	gasindex "github.com/zbum/sensirion-gas-index-go"
)

func ExampleVOC() {
	voc := gasindex.NewVOC()
	for range 60 {
		index := voc.Process(30000)
		_ = index // publish or display the index once per second
	}
	fmt.Println(voc.InBlackout())
	// Output: false
}
