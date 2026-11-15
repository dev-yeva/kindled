package lib

import (
	"crypto/rand"
	"math"
	"strconv"
)

type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

var (
	dataUnits = []string{"b", "kb", "mb", "gb"}
)

func ConvertBytes[n Numeric](bytesNum n) string {
	res := float64(bytesNum)
	i := 0
	for ; i < len(dataUnits); i++ {
		if res < 1024 {
			break
		}
		res = res / 1024
	}
	res = math.Round(res*100) / 100
	formattedNum := strconv.FormatFloat(res, 'f', -1, 64)
	return formattedNum + dataUnits[i]
}

func GenerateLocalPeerID() []byte {
	localPeerID := make([]byte, 20)
	rand.Read(localPeerID)
	return localPeerID
}
