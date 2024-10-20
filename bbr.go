package kcp

const (
	BBR_START_UP  = 1
	BBR_DRAIN     = 2
	BBR_PROBE_BW  = 3
	BBR_PROBE_RTT = 4
)

const (
	MAX_RTT_SAMPLE_COUNT    = 3
	MAX_BW_SAMPLE_COUNT     = 3
	BW_DELTA_RATIO          = 0.01
	RTT_DELTA_RATIO         = 0.1
	MAX_RTT_PROBE_TIME      = 5000
	MAX_START_UP_RESET_TIME = 1000
	MAX_PROBE_BW_RESET_TIME = 1000
	MAX_PROBE_BW_TIME       = 60000

	CONGESTION_RTT_RATIO = 1.5
	CWND_START_UP_RATIO  = 2
	CWND_DRAIN_RATIO     = 0.5
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
	bwStartTime    uint64
	rttStartTime   uint64

	mss uint32
}

func newBBRStateMachine(mss uint32) *BBRStateMachine {

	return &BBRStateMachine{
		state:     BBR_START_UP,
		mss:       mss,
		sampleBw:  0,
		sampleRtt: 0,
	}
}

func (bsm *BBRStateMachine) setMss(mss uint32) {
	bsm.mss = mss
}

func (bsm *BBRStateMachine) input(ts uint32, infight uint32) {

	switch bsm.state {
	case BBR_START_UP:
		bsm.startup(ts)
	case BBR_DRAIN:
		bsm.drain(ts, infight)
	case BBR_PROBE_BW:
		bsm.probeBW(ts)
	case BBR_PROBE_RTT:
		bsm.probeRTT(ts, infight)
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

	if bsm.minRtt == 0 {
		bsm.minRtt = rtt
	}

	if rtt < bsm.minRtt {
		bsm.minRtt = rtt
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

	bsm.cwnd = uint32(float64(bsm.cwnd) * float64(CWND_START_UP_RATIO))
	bsm.paceRate = uint32((1024 * 1024 * 1024) / 8)
	if bsm.cwnd < 4 {
		bsm.cwnd = 4
	}

	//fmt.Println(bsm.delivered, interval, bsm.delivered/interval, bsm.estimateRtt, bsm.minRtt, bsm.estimateBw, bsm.maxBw)

	if bsm.minRtt*CONGESTION_RTT_RATIO < bsm.estimateRtt && bsm.maxBw > bsm.estimateBw*(1-BW_DELTA_RATIO) && bsm.maxBw < bsm.estimateBw*(1+BW_DELTA_RATIO) {
		//fmt.Println("to draining")
		//fmt.Println("minRtt=", bsm.minRtt, "maxBw=", bsm.maxBw)
		bsm.state = BBR_DRAIN
		bsm.delivered = 0
		bsm.start = 0
		bsm.sampleBw = 0
		bsm.sampleRtt = 0
	}

	if curTime-bsm.start > MAX_START_UP_RESET_TIME*1000 {
		bsm.start = 0
		bsm.delivered = 0
		bsm.sampleBw = 0
		bsm.sampleRtt = 0
	}
}

func (bsm *BBRStateMachine) drain(ts uint32, infight uint32) {

	curTime := currentMricos()

	rtt := float64(curTime-uint64(ts*1000)) / float64(1000)

	if rtt < bsm.minRtt {
		bsm.minRtt = rtt
	}

	bsm.cwnd = uint32(float64(bsm.maxBw)*CWND_DRAIN_RATIO*float64(bsm.minRtt)) / bsm.mss
	bsm.paceRate = uint32(float64(bsm.maxBw) * CWND_DRAIN_RATIO)

	if float64(infight*bsm.mss) < bsm.maxBw {
		//fmt.Println("to probe_bw")
		//fmt.Println("minRtt=", bsm.minRtt, "maxBw=", bsm.maxBw)

		bsm.state = BBR_PROBE_BW
		bsm.bwStartTime = curTime
		bsm.start = 0
		bsm.delivered = 0
		bsm.sampleBw = 0
		bsm.sampleRtt = 0
	}

}

func (bsm *BBRStateMachine) probeBW(ts uint32) {

	curTime := currentMricos()

	if bsm.start == 0 {
		bsm.start = uint64(ts * 1000)
	}

	bsm.delivered += float64(bsm.mss)

	interval := float64((uint64(curTime) - uint64(bsm.start))) / float64(1000)

	bw := bsm.delivered / interval

	if bw > bsm.maxBw {
		bsm.maxBw = bw
	}

	bsm.bwSampleCount++

	probeRatio := []float64{1.5, 1.5, 1.5, 1.5, 1.5, 1.5, 1.5, 1.5, 1, 1, 1, 1, 1, 1, 1, 1}
	index := bsm.bwSampleCount % uint32(len(probeRatio))
	ratio := probeRatio[index]

	bsm.cwnd = uint32(float64(bsm.maxBw)*ratio*float64(bsm.minRtt)) / bsm.mss
	bsm.paceRate = uint32(float64(bsm.maxBw) * ratio)

	if curTime-bsm.bwStartTime > MAX_PROBE_BW_TIME*1000 {
		bsm.state = BBR_PROBE_RTT
		//fmt.Println("to probe_rtt")
		//fmt.Println("minRtt=", bsm.minRtt, "maxBw=", bsm.maxBw)
		bsm.start = 0
		bsm.delivered = 0
		bsm.rttProbe = false
		bsm.sampleBw = 0
		bsm.sampleRtt = 0
		bsm.rttStartTime = 0

	}

	if curTime-bsm.start > MAX_PROBE_BW_RESET_TIME*1000 {
		bsm.start = 0
		bsm.delivered = 0
		bsm.sampleBw = 0
		bsm.sampleRtt = 0
	}

}

func (bsm *BBRStateMachine) probeRTT(ts uint32, inflight uint32) {

	curTime := currentMricos()

	rtt := float64(curTime-uint64(ts*1000)) / float64(1000)

	if rtt <= bsm.minRtt*(1+RTT_DELTA_RATIO) {
		bsm.rttProbe = true
		//fmt.Println("minRtt=", bsm.minRtt, "maxBw=", bsm.maxBw)

	}

	if rtt < bsm.minRtt {
		bsm.minRtt = rtt
		//fmt.Println("minRtt=", bsm.minRtt, "maxBw=", bsm.maxBw)
	}

	if float64(inflight*bsm.mss) <= bsm.maxBw {
		bsm.rttStartTime = curTime
	}

	bsm.cwnd = uint32(float64(bsm.maxBw)*CWND_RTT_PROBE_RATIO*float64(bsm.minRtt)) / bsm.mss

	bsm.paceRate = uint32(float64(bsm.maxBw) * CWND_RTT_PROBE_RATIO)

	if bsm.rttProbe {
		//fmt.Println("to probe_bw")
		//fmt.Println("minRtt=", bsm.minRtt, "maxBw=", bsm.maxBw)
		bsm.state = BBR_PROBE_BW
		bsm.start = 0
		bsm.delivered = 0
		bsm.sampleBw = 0
		bsm.sampleRtt = 0
		bsm.bwStartTime = curTime
	}

	if bsm.rttStartTime > 0 && curTime-bsm.rttStartTime > MAX_RTT_PROBE_TIME*1000 && !bsm.rttProbe {
		bsm.state = BBR_START_UP
		//fmt.Println("to start_up")
		//fmt.Println("minRtt=", bsm.minRtt, "maxBw=", bsm.maxBw)
		bsm.start = 0
		bsm.delivered = 0
		bsm.minRtt = 0
		bsm.sampleBw = 0
		bsm.sampleRtt = 0
		bsm.maxBw = 0
	}
}
