package utils

import (
	"math"
)

func cleanData(data []interface{}) []float64 {
	nums := []float64{}
	for _, d := range data {
		num, ok := d.(float64)
		if ok {
			nums = append(nums, num)
			continue
		}

		it, ok := d.(int)
		if ok {
			nums = append(nums, float64(it))
			continue
		}

	}
	return nums
}

// Mean 可以取平均
func Mean(data []interface{}) (float64, int) {
	nums := cleanData(data)
	if len(nums) == 0 {
		return -1, 0
	}

	sum := 0.0
	for _, n := range nums {
		sum += n
	}
	return sum / float64(len(nums)), len(nums)
}

// SD 可以算出標準差
func SD(data []interface{}) (float64, int) {
	nums := cleanData(data)
	if len(nums) == 0 {
		return -1, 0
	}
	mean, _ := Mean(data)
	vars := 0.0
	for _, x := range nums {
		vars += math.Pow(x-mean, 2)
	}
	vars /= float64(len(nums))
	vars = math.Sqrt(vars)
	return vars, len(nums)
}

// ToFloat cast number to float64
func ToFloat(num interface{}) float64 {
	switch i := num.(type) {
	case float64:
		return i
	case float32:
		return float64(i)
	case int:
		return float64(i)
	default:
		return math.NaN()
	}
}
