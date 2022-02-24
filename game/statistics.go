package game

import (
	"math"
	"sort"
)

// Sum ...
func Sum(data []float64) (result float64) {
	for _, v := range data {
		result += v
	}
	return result
}

// Mean ...
func Mean(data []float64) float64 {
	count := float64(len(data))
	if count == 0 {
		return 0
	}
	return Sum(data) / count
}

// Median ...
func Median(data []float64) (median float64) {
	count := float64(len(data))
	if count == 0 {
		return 0.0
	}

	sort.Float64s(data)
	median = (data[int((count-1)*0.5)] + data[int(count*0.5)]) * 0.5
	if int(count)%2 != 0 {
		median = data[int(count*0.5)]
	}

	return
}

// Variance ...
func Variance(data []float64) (variance float64) {
	count := float64(len(data))
	if count == 0 {
		return 0.0
	}
	mean := Sum(data) / count

	for _, number := range data {
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
func StandardDeviation(data []float64) float64 {
	return math.Sqrt(Variance(data))
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

// Skewness ...
func Skewness(data []float64) (skewness float64) {
	sum := Sum(data)
	count := float64(len(data))

	mean := sum / count
	median := Median(data)
	variance := Variance(data)
	if variance > 0 {
		skewness = 3 * (mean - median) / variance
	}

	return
}

// Kurtosis ...
func Kurtosis(data []float64) float64 {
	sum := Sum(data)
	count := float64(len(data))

	if sum == 0.0 || count <= 2 {
		return 0.0
	}

	efficiencyFirst := count * (count + 1) / ((count - 1) * (count - 2) * (count - 3))
	efficiencySecond := 3 * math.Pow(count-1, 2) / ((count - 2) * (count - 3))
	average := Mean(data)

	var variance, varianceSquared float64
	for _, number := range data {
		variance += math.Pow(average-number, 2)
		varianceSquared += math.Pow(average-number, 4)
	}

	return efficiencyFirst*(varianceSquared/math.Pow(variance/sum, 2)) - efficiencySecond
}

// Splice ...
func Splice(data []float64, offset int, length int) []float64 {
	if offset > len(data) {
		return []float64{}
	}
	end := offset + length
	if end < len(data) {
		return data[offset:end]
	}
	return data[offset:]
}

// Outliers ...
func Outliers(collection []float64) int {
	count := float64(len(collection))
	q1 := Median(Splice(collection, 0, int(math.Ceil(count*0.5))))
	q3 := Median(Splice(collection, int(math.Ceil(count*0.5)), int(count)))

	iqr := math.Abs(q1 - q3)
	lowThreshold := q1 - 1.5*iqr
	highThreshold := q3 + 1.5*iqr

	var x, y []float64
	for _, value := range collection {
		if value < lowThreshold {
			x = append(x, value)
		} else if value > highThreshold {
			y = append(y, value)
		}
	}

	return len(x) + len(y)
}
