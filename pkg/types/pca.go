package types

import (
	"fmt"

	"gonum.org/v1/gonum/mat"
)

type PCA struct {
	svd *mat.SVD
}

func (pca *PCA) FitTransform(x []SeriesExtend, lookback, feature int) ([]SeriesExtend, error) {
	if err := pca.Fit(x, lookback); err != nil {
		return nil, err
	}
	return pca.Transform(x, lookback, feature), nil
}

func (pca *PCA) Fit(x []SeriesExtend, lookback int) error {
	vec := make([]float64, lookback*len(x))
	for i, xx := range x {
		mean := xx.Mean(lookback)
		for j := 0; j < lookback; j++ {
			vec[i+j*i] = xx.Last(j) - mean
		}
	}
	pca.svd = &mat.SVD{}
	diffMatrix := mat.NewDense(lookback, len(x), vec)
	if ok := pca.svd.Factorize(diffMatrix, mat.SVDThin); !ok {
		return fmt.Errorf("Unable to factorize")
	}
	return nil
}

func (pca *PCA) Transform(x []SeriesExtend, lookback int, features int) (result []SeriesExtend) {
	result = make([]SeriesExtend, features)
	vTemp := new(mat.Dense)
	pca.svd.VTo(vTemp)
	var ret mat.Dense
	vec := make([]float64, lookback*len(x))
	for i, xx := range x {
		for j := 0; j < lookback; j++ {
			vec[i+j*i] = xx.Last(j)
		}
	}
	newX := mat.NewDense(lookback, len(x), vec)
	ret.Mul(newX, vTemp)
	newMatrix := mat.NewDense(lookback, features, nil)
	newMatrix.Copy(&ret)
	for i := 0; i < features; i++ {
		queue := NewQueue(lookback)
		for j := 0; j < lookback; j++ {
			queue.Update(newMatrix.At(lookback-j-1, i))
		}
		result[i] = queue
	}
	return result
}
