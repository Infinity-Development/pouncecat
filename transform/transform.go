package transform

import (
	"strconv"
	"strings"
	"time"
)

func ToList(v any) []any {
	var res = []any{}
	casted, ok := v.(string)

	if !ok {
		panic("not a list")
	}

	resCasted := strings.Split(strings.ReplaceAll(casted, " ", ""), ",")

	// Ugly but needed for go-staticcheck
	for _, v := range resCasted {
		res = append(res, v)
	}

	return res
}

func ToTimestamp(res any) any {
	if resCast, ok := res.(int64); ok {
		res = time.UnixMilli(resCast)
	} else if resCast, ok := res.(float64); ok {
		res = time.UnixMilli(int64(resCast))
	} else if resCast, ok := res.(string); ok {
		// Cast string to int64
		resD, err := strconv.ParseInt(resCast, 10, 64)
		if err != nil {
			// Could be a datetime string
			resDV, err := time.Parse(time.RFC3339, resCast)
			if err != nil {
				// Last ditch effort, try checking if its NOW or something
				if strings.Contains(resCast, "NOW") {
					res = time.Now()
				} else {
					panic(err)
				}
			} else {
				res = resDV
			}
		} else {
			res = time.UnixMilli(resD)
		}
	}

	return res
}
