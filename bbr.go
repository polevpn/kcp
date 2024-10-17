package kcp

import (
	"fmt"
)

const (
	BBR_START_UP  = 1
	BBR_DRAIN     = 2
	BBR_PROBE_BW  = 3
	BBR_PROBE_RTT = 4
)

const (
	MAX_RTT_SAMPLE_COUNT = 20
	MAX_BW_SAMPLE_COUNT  = 20
	BW_DELTA_RATIO       = 0.01
	MAX_RTT_PROBE_TIME   = 10000
	MAX_START_RESET_TIME = 10000

	CONGESTION_RTT_RATIO = 1.3
	CWND_START_UP_RATIO  = 2
	CWND_DRAIN_RATIO     = 0.75
	CWND_BW_PROBE_RATIO  = 1.05
	CWND_RTT_PROBE_RATIO = 0.5
)

type BBRStateMachine struct {
	state          int
	start          uint64
	delivered      float64
	minRtt         float64
	maxBw          float64
	sampleRtt      float64
	estimateRtt    float64
	sampleBw       float64
	estimateBw     float64
	rttSampleCount uint32
	bwSampleCount  uint32
	cwnd           uint32
	paceRate       uint32
	rttProbe       bool
	mss            uint32
}

func newBBRStateMachine(mss uint32) *BBRStateMachine {

	return &BBRStateMachine{
		state:    BBR_START_UP,
		mss:      mss,
		sampleBw: 0,
	}
}

func (bsm *BBRStateMachine) setMss(mss uint32) {
	bsm.mss = mss
}

func (bsm *BBRStateMachine) input(ts uint32, inFlight uint32) {

	switch bsm.state {
	case BBR_START_UP:
		bsm.startup(ts)
	case BBR_DRAIN:
		bsm.drain(ts, inFlight)
	case BBR_PROBE_BW:
		bsm.probeBW(ts, inFlight)
	case BBR_PROBE_RTT:
		bsm.probeRTT(ts)
	}
}

func (bsm *BBRStateMachine) output() (uint32, uint32) {
	return bsm.paceRate, bsm.cwnd
}

func (bsm *BBRStateMachine) startup(ts uint32) {

	curTime := currentMricos()

	if bsm.start == 0 {
		bsm.start = uint64(ts * 1000)
	}

	bsm.delivered += float64(bsm.mss)
	rtt := float64(curTime-uint64(ts*1000)) / float64(1000)

	interval := float64((uint64(curTime) - uint64(bsm.start))) / float64(1000)

	bw := bsm.delivered / interval

	if bsm.minRtt == 0.0 {
		bsm.minRtt = rtt
	}

	if rtt < bsm.minRtt {
		bsm.minRtt = rtt
	}

	if curTime-bsm.start > MAX_START_RESET_TIME*1000 {
		bsm.start = 0
		bsm.delivered = 0
		bsm.sampleBw = 0
		bsm.sampleRtt = 0
	}

	if (bsm.rttSampleCount % MAX_RTT_SAMPLE_COUNT) == 0 {
		if bsm.sampleRtt > 0 {
			bsm.estimateRtt = bsm.sampleRtt
		}
		bsm.sampleRtt = rtt
	} else {
		bsm.estimateRtt = 0
		bsm.sampleRtt = (bsm.sampleRtt + rtt) / 2
	}

	bsm.rttSampleCount++

	if (bsm.bwSampleCount % MAX_BW_SAMPLE_COUNT) == 0 {
		if bsm.sampleBw > 0 {
			bsm.estimateBw = bsm.sampleBw
		}
		bsm.sampleBw = bw
	} else {
		bsm.estimateBw = 0
		bsm.sampleBw = (bsm.sampleBw + bw) / 2
	}

	bsm.bwSampleCount++

	if bsm.maxBw < bw {
		bsm.maxBw = bw
	}

	bsm.cwnd = bsm.cwnd * 2
	bsm.paceRate = uint32((1024 * 1024 * 1024) / 8)
	if bsm.cwnd < 4 {
		bsm.cwnd = 4
	}

	fmt.Println(bsm.delivered, interval, bsm.delivered/interval, bsm.estimateRtt, bsm.minRtt, bsm.estimateBw, bsm.maxBw)

	if bsm.minRtt*CONGESTION_RTT_RATIO < bsm.estimateRtt && bsm.maxBw > bsm.estimateBw*(1-BW_DELTA_RATIO) && bsm.maxBw < bsm.estimateBw*(1+BW_DELTA_RATIO) {
		fmt.Println("to draining")
		bsm.state = BBR_DRAIN
		bsm.delivered = 0
		bsm.start = 0
		bsm.sampleBw = 0
		bsm.sampleRtt = 0
	}

}

func (bsm *BBRStateMachine) drain(ts uint32, inFlight uint32) {

	curTime := currentMricos()

	rtt := float64(curTime-uint64(ts*1000)) / float64(1000)

	if rtt < bsm.minRtt {
		bsm.minRtt = rtt
	}

	bsm.cwnd = uint32(float64(bsm.maxBw)*CWND_DRAIN_RATIO*float64(bsm.minRtt)) / bsm.mss
	bsm.paceRate = uint32(float64(bsm.maxBw) * CWND_DRAIN_RATIO)

	if float64(inFlight*bsm.mss) < bsm.maxBw {
		fmt.Println("to probe_bw")
		bsm.state = BBR_PROBE_BW
		bsm.start = 0
		bsm.delivered = 0
		bsm.sampleBw = 0
		bsm.sampleRtt = 0
	}

}

func (bsm *BBRStateMachine) probeBW(ts uint32, inFlight uint32) {

	curTime := currentMricos()

	if bsm.start == 0 {
		bsm.start = uint64(ts * 1000)
	}

	bsm.delivered += float64(bsm.mss)

	rtt := float64(curTime-uint64(ts*1000)) / float64(1000)

	if (bsm.rttSampleCount % MAX_RTT_SAMPLE_COUNT) == 0 {
		if bsm.sampleRtt > 0 {
			bsm.estimateRtt = bsm.sampleRtt
		}
		bsm.sampleRtt = rtt
	} else {
		bsm.estimateRtt = 0
		bsm.sampleRtt = (bsm.sampleRtt + rtt) / 2
	}

	bsm.rttSampleCount++

	interval := float64((uint64(curTime) - uint64(bsm.start))) / float64(1000)

	bw := bsm.delivered / interval

	if bw > bsm.maxBw {
		bsm.maxBw = bw
	}

	probeRatio := []float64{1.25, 0.75, 1, 1, 1, 1, 1, 1}
	index := bsm.rttSampleCount % uint32(len(probeRatio))
	ratio := probeRatio[index]

	bsm.cwnd = uint32(float64(bsm.maxBw)*ratio*float64(bsm.minRtt)) / bsm.mss
	bsm.paceRate = uint32(float64(bsm.maxBw) * ratio)

	if bsm.minRtt*CONGESTION_RTT_RATIO < bsm.estimateRtt && float64(inFlight*bsm.mss) > bsm.maxBw {
		bsm.state = BBR_PROBE_RTT
		fmt.Println("to probe_rtt")
		bsm.start = 0
		bsm.delivered = 0
		bsm.rttProbe = false
		bsm.sampleBw = 0
		bsm.sampleRtt = 0

	}

}

func (bsm *BBRStateMachine) probeRTT(ts uint32) {

	curTime := currentMricos()

	bsm.delivered += float64(bsm.mss)

	rtt := float64(curTime-uint64(ts*1000)) / float64(1000)

	if curTime-bsm.start > MAX_RTT_PROBE_TIME*1000 && bsm.rttProbe {
		fmt.Println("to probe_bw")
		bsm.state = BBR_PROBE_BW
		bsm.start = curTime
		bsm.delivered = 0
		bsm.sampleBw = 0
		bsm.sampleRtt = 0
	} else if curTime-bsm.start > MAX_RTT_PROBE_TIME*1000 && !bsm.rttProbe {
		bsm.state = BBR_START_UP
		fmt.Println("to start_up")
		bsm.start = curTime
		bsm.delivered = 0
		bsm.minRtt = 0
		bsm.sampleBw = 0
		bsm.sampleRtt = 0
	}

	if rtt <= bsm.minRtt {
		bsm.minRtt = rtt
		bsm.rttProbe = true
	}

	bsm.cwnd = uint32(float64(bsm.maxBw)*CWND_RTT_PROBE_RATIO*float64(bsm.minRtt)) / bsm.mss

	bsm.paceRate = uint32(float64(bsm.maxBw) * CWND_RTT_PROBE_RATIO)

}
