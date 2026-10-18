//go:build cgo

// Package referencec exposes Sensirion's C implementation for differential
// testing. It is internal so applications cannot accidentally depend on CGO.
package referencec

/*
#cgo CFLAGS: -std=c99
#cgo LDFLAGS: -lm
#include "sensirion_gas_index_algorithm.h"

static float reference_mean(const GasIndexAlgorithmParams *params) {
	return params->m_Mean_Variance_Estimator___Mean +
	       params->m_Mean_Variance_Estimator___Sraw_Offset;
}

static float reference_std(const GasIndexAlgorithmParams *params) {
	return params->m_Mean_Variance_Estimator___Std;
}
*/
import "C"

// VOC wraps one instance of Sensirion's official floating-point C algorithm.
type VOC struct {
	params C.GasIndexAlgorithmParams
}

// NewVOC returns a reference VOC processor configured with the C defaults.
func NewVOC() *VOC {
	v := new(VOC)
	C.GasIndexAlgorithm_init(&v.params, C.GasIndexAlgorithm_ALGORITHM_TYPE_VOC)
	return v
}

// Process consumes one raw SGP40 signal and returns the rounded VOC Index.
func (v *VOC) Process(raw int32) int32 {
	var index C.int32_t
	C.GasIndexAlgorithm_process(&v.params, C.int32_t(raw), &index)
	return int32(index)
}

// State returns the C implementation's learned mean and standard deviation.
func (v *VOC) State() (mean, std float32) {
	return float32(C.reference_mean(&v.params)), float32(C.reference_std(&v.params))
}
