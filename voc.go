// Package gasindex implements Sensirion's gas index algorithms in pure Go.
//
// The VOC implementation is a float32 port of Sensirion's BSD-3-Clause
// gas-index-algorithm. Call Process once per second with an unshifted SGP40
// raw signal. A processor is stateful and must not be shared concurrently.
package gasindex

import "math"

const (
	samplingInterval              float32 = 1
	initialBlackout               float32 = 45
	indexGain                     float32 = 230
	srawStdInitial                float32 = 50
	srawStdBonusVOC               float32 = 220
	tauMeanHoursVOC               float32 = 12
	tauVarianceHoursVOC           float32 = 12
	tauInitialMeanVOC             float32 = 20
	initDurationMeanVOC           float32 = 3600 * 0.75
	initTransitionMean            float32 = 0.01
	tauInitialVariance            float32 = 2500
	initDurationVarianceVOC       float32 = 3600 * 1.45
	initTransitionVariance        float32 = 0.01
	gatingThresholdVOC            float32 = 340
	gatingThresholdInitial        float32 = 510
	gatingThresholdTransition     float32 = 0.09
	gatingMaxDurationMinutesVOC   float32 = 60 * 3
	gatingMaxRatio                float32 = 0.3
	sigmoidL                      float32 = 500
	sigmoidKVOC                   float32 = -0.0065
	sigmoidX0VOC                  float32 = 213
	vocIndexOffsetDefault         float32 = 100
	lpTauFast                     float32 = 20
	lpTauSlow                     float32 = 500
	lpAlpha                       float32 = -0.2
	mveGammaScaling               float32 = 64
	mveAdditionalGammaMeanScaling float32 = 8
	mveFix16Max                   float32 = 32767
)

// VOC converts SGP40 raw signals into a VOC Index in the range 0..500.
// The first 46 calls return 0 while the algorithm completes its blackout.
type VOC struct {
	uptime, sraw, index float32

	initialized                              bool
	mean, srawOffset, std                    float32
	gamma, gammaVariance                     float32
	gammaInitialMean, gammaInitialVariance   float32
	gammaMean, gammaVarianceSteady           float32
	uptimeGamma, uptimeGating, gatingMinutes float32

	moxStd, moxMean  float32
	lpA1, lpA2       float32
	lpInitialized    bool
	lpX1, lpX2, lpX3 float32
}

// NewVOC returns a VOC processor configured with Sensirion's defaults.
func NewVOC() *VOC {
	v := new(VOC)
	v.Reset()
	return v
}

// Reset discards learned state and restores the power-on condition.
func (v *VOC) Reset() {
	*v = VOC{
		std:    srawStdInitial,
		moxStd: srawStdInitial,
		lpA1:   samplingInterval / (lpTauFast + samplingInterval),
		lpA2:   samplingInterval / (lpTauSlow + samplingInterval),
		gammaMean: (mveAdditionalGammaMeanScaling * mveGammaScaling * samplingInterval / 3600) /
			(tauMeanHoursVOC + samplingInterval/3600),
		gammaVarianceSteady: (mveGammaScaling * samplingInterval / 3600) /
			(tauVarianceHoursVOC + samplingInterval/3600),
		gammaInitialMean: (mveAdditionalGammaMeanScaling * mveGammaScaling * samplingInterval) /
			(tauInitialMeanVOC + samplingInterval),
		gammaInitialVariance: (mveGammaScaling * samplingInterval) /
			(tauInitialVariance + samplingInterval),
	}
}

// Process consumes one unshifted SGP40 raw signal. It must be called once per
// second. Invalid values reuse the last accepted raw signal, as in the C API.
func (v *VOC) Process(raw int32) int32 {
	if v.uptime <= initialBlackout {
		v.uptime += samplingInterval
		return 0
	}
	if raw > 0 && raw < 65000 {
		raw = min(max(raw, 20001), 52767)
		v.sraw = float32(raw - 20000)
	}

	v.index = v.sigmoid(v.mox(v.sraw))
	v.index = v.lowpass(v.index)
	if !(v.index >= 0.5) { // also repairs NaN
		v.index = 0.5
	}
	if v.sraw > 0 {
		v.updateEstimator(v.sraw, v.index)
		v.moxStd = v.std
		v.moxMean = v.mean + v.srawOffset
	}
	return int32(v.index + 0.5)
}

// Index returns the latest unrounded VOC Index.
func (v *VOC) Index() float32 { return v.index }

// State returns the learned raw-signal mean and standard deviation. These
// values are useful for diagnostics and future persistence support.
func (v *VOC) State() (mean, std float32) {
	return v.mean + v.srawOffset, v.std
}

// InBlackout reports whether Process is still suppressing startup output.
func (v *VOC) InBlackout() bool { return v.uptime <= initialBlackout }

func (v *VOC) updateEstimator(sraw, index float32) {
	if !v.initialized {
		v.initialized = true
		v.srawOffset = sraw
		return
	}
	if v.mean >= 100 || v.mean <= -100 {
		v.srawOffset += v.mean
		v.mean = 0
	}
	sraw -= v.srawOffset
	v.calculateGamma(index)
	delta := (sraw - v.mean) / mveGammaScaling
	c := v.std + abs(delta)
	scaling := float32(1)
	if c > 1440 {
		scaling = (c / 1440) * (c / 1440)
	}
	v.std = sqrt(scaling*(mveGammaScaling-v.gammaVariance)) *
		sqrt(v.std*(v.std/(mveGammaScaling*scaling))+
			(v.gammaVariance*delta/scaling)*delta)
	v.mean += v.gamma * delta / mveAdditionalGammaMeanScaling
}

func (v *VOC) calculateGamma(index float32) {
	limit := mveFix16Max - samplingInterval
	if v.uptimeGamma < limit {
		v.uptimeGamma += samplingInterval
	}
	if v.uptimeGating < limit {
		v.uptimeGating += samplingInterval
	}

	sigmoidGammaMean := unitSigmoid(v.uptimeGamma, initTransitionMean, initDurationMeanVOC)
	gammaMean := v.gammaMean + (v.gammaInitialMean-v.gammaMean)*sigmoidGammaMean
	thresholdMean := gatingThresholdVOC + (gatingThresholdInitial-gatingThresholdVOC)*
		unitSigmoid(v.uptimeGating, initTransitionMean, initDurationMeanVOC)
	sigmoidGatingMean := unitSigmoid(index, gatingThresholdTransition, thresholdMean)
	v.gamma = sigmoidGatingMean * gammaMean

	sigmoidGammaVariance := unitSigmoid(v.uptimeGamma, initTransitionVariance, initDurationVarianceVOC)
	gammaVariance := v.gammaVarianceSteady +
		(v.gammaInitialVariance-v.gammaVarianceSteady)*(sigmoidGammaVariance-sigmoidGammaMean)
	thresholdVariance := gatingThresholdVOC + (gatingThresholdInitial-gatingThresholdVOC)*
		unitSigmoid(v.uptimeGating, initTransitionVariance, initDurationVarianceVOC)
	v.gammaVariance = unitSigmoid(index, gatingThresholdTransition, thresholdVariance) * gammaVariance

	v.gatingMinutes += samplingInterval / 60 *
		((1-sigmoidGatingMean)*(1+gatingMaxRatio) - gatingMaxRatio)
	v.gatingMinutes = max(v.gatingMinutes, 0)
	if v.gatingMinutes > gatingMaxDurationMinutesVOC {
		v.uptimeGating = 0
	}
}

func (v *VOC) mox(sraw float32) float32 {
	return (sraw - v.moxMean) / -(v.moxStd + srawStdBonusVOC) * indexGain
}

func (v *VOC) sigmoid(sample float32) float32 {
	x := sigmoidKVOC * (sample - sigmoidX0VOC)
	if x < -50 {
		return sigmoidL
	}
	if x > 50 {
		return 0
	}
	if sample >= 0 {
		shift := (sigmoidL - 5*vocIndexOffsetDefault) / 4
		return (sigmoidL+shift)/(1+exp(x)) - shift
	}
	return sigmoidL / (1 + exp(x))
}

func (v *VOC) lowpass(sample float32) float32 {
	if !v.lpInitialized {
		v.lpX1, v.lpX2, v.lpX3 = sample, sample, sample
		v.lpInitialized = true
	}
	v.lpX1 += v.lpA1 * (sample - v.lpX1)
	v.lpX2 += v.lpA2 * (sample - v.lpX2)
	tau := (lpTauSlow-lpTauFast)*exp(lpAlpha*abs(v.lpX1-v.lpX2)) + lpTauFast
	a3 := samplingInterval / (samplingInterval + tau)
	v.lpX3 += a3 * (sample - v.lpX3)
	return v.lpX3
}

func unitSigmoid(sample, k, x0 float32) float32 {
	x := k * (sample - x0)
	if x < -50 {
		return 1
	}
	if x > 50 {
		return 0
	}
	return 1 / (1 + exp(x))
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func exp(v float32) float32  { return float32(math.Exp(float64(v))) }
func sqrt(v float32) float32 { return float32(math.Sqrt(float64(v))) }
