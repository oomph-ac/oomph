package omath

import "math"

// Sum ...
func Sum(nums []float64) (result float64) {
	for _, v := range nums {
		result += v
	}
	return result
}

// Mean ...
func Mean(nums []float64) float64 {
	count := float64(len(nums))
	if count == 0 {
		return 0
	}
	var sum float64
	for _, v := range nums {
		sum += v
	}
	return sum / count
}

// Variance ...
func Variance(nums []float64) (variance float64) {
	count := float64(len(nums))
	if count == 0 {
		return 0.0
	}
	mean := Sum(nums) / count

	for _, number := range nums {
		variance += math.Pow(number-mean, 2)
	}
	return variance / count
}

// Covariance ...
func Covariance(x, y []float64) float64 {
	xMean, yMean := Mean(x), Mean(y)
	l := len(x)

	var sum float64
	for i := 0; i < l; i++ {
		sum += (x[i] - xMean) * (y[i] - yMean)
	}
	return sum / float64(l)
}

// StandardDeviation ...
func StandardDeviation(nums []float64) float64 {
	return math.Sqrt(Variance(nums))
}

// CorrelationCoefficient ...
func CorrelationCoefficient(x, y []float64) float64 {
	xStdDev := StandardDeviation(x)
	yStdDev := StandardDeviation(y)
	covariance := Covariance(x, y)
	if xStdDev*yStdDev == 0 {
		return 0
	}
	return covariance / (xStdDev * yStdDev)
}
