package kcp

const (
	BBR_START_UP  = 1
	BBR_DRAIN     = 2
	BBR_PROBE_BW  = 3
	BBR_PROBE_RTT = 4
)

const (
	MAX_RTT_SAMPLE_COUNT = 20
	MAX_BW_SAMPLE_COUNT  = 20
	BW_DELTA_RATIO       = 0.05
	MAX_RTT_PROBE_TIME   = 3000

	CONGESTION_RTT_RATIO = 1.3
	CWND_START_UP_RATIO  = 1.5
	CWND_DRAIN_RATIO     = 0.75
	CWND_BW_PROBE_RATIO  = 1.05
	CWND_RTT_PROBE_RATIO = 0.5
)

type BBRStateMachine struct {
	state          int
	start          uint32
	delivered      uint64
	minRtt         uint32
	maxBw          uint64
	sampleRtt      uint32
	sampleBw       uint64
	rttSampleCount uint64
	bwSampleCount  uint32
	cwnd           uint32
	paceRate       uint32
}

func (bsm *BBRStateMachine) getPaceRateAndCWnd(seg segment, inFlight uint64) (uint32, uint32) {

	switch bsm.state {
	case BBR_START_UP:
		return bsm.startup(seg)
	case BBR_DRAIN:
		return bsm.drain(seg, inFlight)
	case BBR_PROBE_BW:
		return bsm.probeBW(seg, inFlight)
	case BBR_PROBE_RTT:
		return bsm.probeRTT(seg)
	}

	return 0, 0
}

func (bsm *BBRStateMachine) startup(seg segment) (uint32, uint32) {

	curTime := currentMs()
	if bsm.start == 0 {
		bsm.start = curTime
	}

	bsm.delivered += uint64(len(seg.data))
	rtt := curTime - seg.ts

	if rtt < bsm.minRtt {
		bsm.minRtt = rtt
	}

	if (bsm.rttSampleCount % MAX_RTT_SAMPLE_COUNT) == 0 {
		bsm.sampleRtt = rtt
	} else {
		bsm.sampleRtt = (bsm.sampleRtt + rtt) / 2
	}

	bsm.rttSampleCount++

	bw := bsm.delivered / (uint64(curTime) - uint64(bsm.start))

	if (bsm.bwSampleCount % MAX_BW_SAMPLE_COUNT) == 0 {
		bsm.sampleBw = bw
	}

	bsm.bwSampleCount++

	if uint32(float64(bsm.minRtt)*CONGESTION_RTT_RATIO) < bsm.sampleRtt && bw > uint64(float64(bsm.sampleBw)*(1-BW_DELTA_RATIO)) && bw < uint64(float64(bsm.sampleBw)*(1+BW_DELTA_RATIO)) {
		bsm.maxBw = bw
		bsm.state = BBR_DRAIN
	}

	if bw == 0 {
		bw = 1
	}

	bsm.cwnd = uint32(float64(bw) * CWND_START_UP_RATIO * float64(bsm.minRtt))

	return 0, bsm.cwnd
}

func (bsm *BBRStateMachine) drain(seg segment, inFlight uint64) (uint32, uint32) {

	curTime := currentMs()

	if inFlight < uint64(bsm.maxBw) {

		bsm.state = BBR_PROBE_BW
	}

	rtt := curTime - seg.ts

	if rtt < bsm.minRtt {
		bsm.minRtt = rtt
	}

	bsm.cwnd = uint32(float64(bsm.maxBw) * CWND_DRAIN_RATIO * float64(bsm.minRtt))

	return 0, bsm.cwnd

}

func (bsm *BBRStateMachine) probeBW(seg segment, inFlight uint64) (uint32, uint32) {

	curTime := currentMs()

	rtt := curTime - seg.ts

	if (bsm.rttSampleCount % MAX_RTT_SAMPLE_COUNT) == 0 {
		bsm.sampleRtt = rtt
	} else {
		bsm.sampleRtt = (bsm.sampleRtt + rtt) / 2
	}

	bsm.rttSampleCount++

	if uint32(float64(bsm.minRtt)*CONGESTION_RTT_RATIO) < bsm.sampleRtt && inFlight > uint64(bsm.maxBw) {
		bsm.state = BBR_PROBE_RTT
		bsm.minRtt = bsm.sampleRtt
		bsm.start = curTime
	}

	bw := bsm.delivered / (uint64(curTime) - uint64(bsm.start))

	if bw > bsm.maxBw {
		bsm.maxBw = bw
	}

	bsm.cwnd = uint32(float64(bsm.maxBw) * CWND_BW_PROBE_RATIO * float64(bsm.minRtt))

	return 0, bsm.cwnd
}

func (bsm *BBRStateMachine) probeRTT(seg segment) (uint32, uint32) {

	curTime := currentMs()

	rtt := curTime - seg.ts

	if curTime-bsm.start > MAX_RTT_PROBE_TIME {
		bsm.state = BBR_PROBE_BW
	}

	if rtt < bsm.minRtt {
		bsm.minRtt = rtt
	}

	bsm.cwnd = uint32(float64(bsm.maxBw) * CWND_RTT_PROBE_RATIO * float64(bsm.minRtt))

	return 0, bsm.cwnd
}
