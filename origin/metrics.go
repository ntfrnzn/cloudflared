package origin

import (
	"errors"
	"sync"
	"time"

	"github.com/cloudflare/cloudflared/h2mux"

	"github.com/prometheus/client_golang/prometheus"
)

type muxerMetrics struct {
	rtt              *prometheus.GaugeVec
	rttMin           *prometheus.GaugeVec
	rttMax           *prometheus.GaugeVec
	receiveWindowAve *prometheus.GaugeVec
	sendWindowAve    *prometheus.GaugeVec
	receiveWindowMin *prometheus.GaugeVec
	receiveWindowMax *prometheus.GaugeVec
	sendWindowMin    *prometheus.GaugeVec
	sendWindowMax    *prometheus.GaugeVec
	inBoundRateCurr  *prometheus.GaugeVec
	inBoundRateMin   *prometheus.GaugeVec
	inBoundRateMax   *prometheus.GaugeVec
	outBoundRateCurr *prometheus.GaugeVec
	outBoundRateMin  *prometheus.GaugeVec
	outBoundRateMax  *prometheus.GaugeVec
	compBytesBefore  *prometheus.GaugeVec
	compBytesAfter   *prometheus.GaugeVec
	compRateAve      *prometheus.GaugeVec
}

type TunnelMetrics struct {
	haConnections     prometheus.Gauge
	totalRequests     prometheus.Counter
	requestsPerTunnel *prometheus.CounterVec
	// concurrentRequestsLock is a mutex for concurrentRequests and maxConcurrentRequests
	concurrentRequestsLock      sync.Mutex
	concurrentRequestsPerTunnel *prometheus.GaugeVec
	// concurrentRequests records count of concurrent requests for each tunnel
	concurrentRequests             map[string]uint64
	maxConcurrentRequestsPerTunnel *prometheus.GaugeVec
	// concurrentRequests records max count of concurrent requests for each tunnel
	maxConcurrentRequests map[string]uint64
	timerRetries          prometheus.Gauge
	responseByCode        *prometheus.CounterVec
	responseCodePerTunnel *prometheus.CounterVec
	serverLocations       *prometheus.GaugeVec
	// locationLock is a mutex for oldServerLocations
	locationLock sync.Mutex
	// oldServerLocations stores the last server the tunnel was connected to
	oldServerLocations map[string]string

	// labelConfig stores the set of extended key/value labels
	labelConfig metricsLabelConfig

	muxerMetrics *muxerMetrics
}

type metricsLabelConfig struct {
	connectionKey  string
	locationKey    string
	statusKey      string
	extendedKeys   []string
	extendedValues []string
}

func defaultMetricsLabelConfig() metricsLabelConfig {
	return metricsLabelConfig{
		connectionKey:  "connection_id",
		locationKey:    "location",
		statusKey:      "status_code",
		extendedKeys:   []string{},
		extendedValues: []string{},
	}
}

func (config *metricsLabelConfig) setLabelValues(values []string) error {
	if len(values) != len(config.extendedKeys) {
		return errors.New("new set of label values doesn't match number of keys")
	}
	config.extendedValues = values
	return nil
}

func (config *metricsLabelConfig) getConnectionKeys() []string {
	return append([]string{config.connectionKey}, config.extendedKeys...)
}
func (config *metricsLabelConfig) getStatusKeys() []string {
	return append([]string{config.connectionKey, config.statusKey}, config.extendedKeys...)
}
func (config *metricsLabelConfig) getLocationKeys() []string {
	return append([]string{config.connectionKey, config.locationKey}, config.extendedKeys...)
}

func (config *metricsLabelConfig) getConnectionValues(connectionID string) []string {
	return append([]string{connectionID}, config.extendedValues...)
}
func (config *metricsLabelConfig) getStatusValues(connectionID, status string) []string {
	return append([]string{connectionID, status}, config.extendedValues...)
}
func (config *metricsLabelConfig) getLocationValues(connectionID, location string) []string {
	return append([]string{connectionID, location}, config.extendedValues...)
}

func newMetricsLabelConfig(extendedLabels map[string]string) metricsLabelConfig {
	config := defaultMetricsLabelConfig()
	for key, value := range extendedLabels {
		config.extendedKeys = append(config.extendedKeys, key)
		config.extendedValues = append(config.extendedValues, value)
	}
	return config
}

func newMuxerMetrics(config *metricsLabelConfig) *muxerMetrics {
	rtt := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rtt",
			Help: "Round-trip time in millisecond",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(rtt)

	rttMin := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rtt_min",
			Help: "Shortest round-trip time in millisecond",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(rttMin)

	rttMax := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rtt_max",
			Help: "Longest round-trip time in millisecond",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(rttMax)

	receiveWindowAve := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "receive_window_ave",
			Help: "Average receive window size in bytes",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(receiveWindowAve)

	sendWindowAve := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "send_window_ave",
			Help: "Average send window size in bytes",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(sendWindowAve)

	receiveWindowMin := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "receive_window_min",
			Help: "Smallest receive window size in bytes",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(receiveWindowMin)

	receiveWindowMax := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "receive_window_max",
			Help: "Largest receive window size in bytes",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(receiveWindowMax)

	sendWindowMin := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "send_window_min",
			Help: "Smallest send window size in bytes",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(sendWindowMin)

	sendWindowMax := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "send_window_max",
			Help: "Largest send window size in bytes",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(sendWindowMax)

	inBoundRateCurr := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "inbound_bytes_per_sec_curr",
			Help: "Current inbounding bytes per second, 0 if there is no incoming connection",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(inBoundRateCurr)

	inBoundRateMin := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "inbound_bytes_per_sec_min",
			Help: "Minimum non-zero inbounding bytes per second",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(inBoundRateMin)

	inBoundRateMax := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "inbound_bytes_per_sec_max",
			Help: "Maximum inbounding bytes per second",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(inBoundRateMax)

	outBoundRateCurr := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "outbound_bytes_per_sec_curr",
			Help: "Current outbounding bytes per second, 0 if there is no outgoing traffic",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(outBoundRateCurr)

	outBoundRateMin := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "outbound_bytes_per_sec_min",
			Help: "Minimum non-zero outbounding bytes per second",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(outBoundRateMin)

	outBoundRateMax := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "outbound_bytes_per_sec_max",
			Help: "Maximum outbounding bytes per second",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(outBoundRateMax)

	compBytesBefore := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "comp_bytes_before",
			Help: "Bytes sent via cross-stream compression, pre compression",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(compBytesBefore)

	compBytesAfter := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "comp_bytes_after",
			Help: "Bytes sent via cross-stream compression, post compression",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(compBytesAfter)

	compRateAve := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "comp_rate_ave",
			Help: "Average outbound cross-stream compression ratio",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(compRateAve)

	return &muxerMetrics{
		rtt:              rtt,
		rttMin:           rttMin,
		rttMax:           rttMax,
		receiveWindowAve: receiveWindowAve,
		sendWindowAve:    sendWindowAve,
		receiveWindowMin: receiveWindowMin,
		receiveWindowMax: receiveWindowMax,
		sendWindowMin:    sendWindowMin,
		sendWindowMax:    sendWindowMax,
		inBoundRateCurr:  inBoundRateCurr,
		inBoundRateMin:   inBoundRateMin,
		inBoundRateMax:   inBoundRateMax,
		outBoundRateCurr: outBoundRateCurr,
		outBoundRateMin:  outBoundRateMin,
		outBoundRateMax:  outBoundRateMax,
		compBytesBefore:  compBytesBefore,
		compBytesAfter:   compBytesAfter,
		compRateAve:      compRateAve,
	}
}

func (m *muxerMetrics) update(connectionLabelValues []string, metrics *h2mux.MuxerMetrics) {
	m.rtt.WithLabelValues(connectionLabelValues...).Set(convertRTTMilliSec(metrics.RTT))
	m.rttMin.WithLabelValues(connectionLabelValues...).Set(convertRTTMilliSec(metrics.RTTMin))
	m.rttMax.WithLabelValues(connectionLabelValues...).Set(convertRTTMilliSec(metrics.RTTMax))
	m.receiveWindowAve.WithLabelValues(connectionLabelValues...).Set(metrics.ReceiveWindowAve)
	m.sendWindowAve.WithLabelValues(connectionLabelValues...).Set(metrics.SendWindowAve)
	m.receiveWindowMin.WithLabelValues(connectionLabelValues...).Set(float64(metrics.ReceiveWindowMin))
	m.receiveWindowMax.WithLabelValues(connectionLabelValues...).Set(float64(metrics.ReceiveWindowMax))
	m.sendWindowMin.WithLabelValues(connectionLabelValues...).Set(float64(metrics.SendWindowMin))
	m.sendWindowMax.WithLabelValues(connectionLabelValues...).Set(float64(metrics.SendWindowMax))
	m.inBoundRateCurr.WithLabelValues(connectionLabelValues...).Set(float64(metrics.InBoundRateCurr))
	m.inBoundRateMin.WithLabelValues(connectionLabelValues...).Set(float64(metrics.InBoundRateMin))
	m.inBoundRateMax.WithLabelValues(connectionLabelValues...).Set(float64(metrics.InBoundRateMax))
	m.outBoundRateCurr.WithLabelValues(connectionLabelValues...).Set(float64(metrics.OutBoundRateCurr))
	m.outBoundRateMin.WithLabelValues(connectionLabelValues...).Set(float64(metrics.OutBoundRateMin))
	m.outBoundRateMax.WithLabelValues(connectionLabelValues...).Set(float64(metrics.OutBoundRateMax))
	m.compBytesBefore.WithLabelValues(connectionLabelValues...).Set(float64(metrics.CompBytesBefore.Value()))
	m.compBytesAfter.WithLabelValues(connectionLabelValues...).Set(float64(metrics.CompBytesAfter.Value()))
	m.compRateAve.WithLabelValues(connectionLabelValues...).Set(float64(metrics.CompRateAve()))
}

func convertRTTMilliSec(t time.Duration) float64 {
	return float64(t / time.Millisecond)
}

// Metrics that can be collected without asking the edge
func NewTunnelMetrics() *TunnelMetrics {
	config := defaultMetricsLabelConfig()
	return NewTunnelMetricsFromConfig(config)
}

func NewTunnelMetricsFromConfig(config metricsLabelConfig) *TunnelMetrics {
	haConnections := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ha_connections",
			Help: "Number of active ha connections",
		})
	prometheus.MustRegister(haConnections)

	totalRequests := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "total_requests",
			Help: "Amount of requests proxied through all the tunnels",
		})
	prometheus.MustRegister(totalRequests)

	requestsPerTunnel := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "requests_per_tunnel",
			Help: "Amount of requests proxied through each tunnel",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(requestsPerTunnel)

	concurrentRequestsPerTunnel := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "concurrent_requests_per_tunnel",
			Help: "Concurrent requests proxied through each tunnel",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(concurrentRequestsPerTunnel)

	maxConcurrentRequestsPerTunnel := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "max_concurrent_requests_per_tunnel",
			Help: "Largest number of concurrent requests proxied through each tunnel so far",
		},
		config.getConnectionKeys(),
	)
	prometheus.MustRegister(maxConcurrentRequestsPerTunnel)

	timerRetries := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "timer_retries",
			Help: "Unacknowledged heart beats count",
		})
	prometheus.MustRegister(timerRetries)

	responseByCode := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "response_by_code",
			Help: "Count of responses by HTTP status code",
		},
		config.getStatusKeys(),
	)
	prometheus.MustRegister(responseByCode)

	responseCodePerTunnel := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "response_code_per_tunnel",
			Help: "Count of responses by HTTP status code fore each tunnel",
		},
		config.getStatusKeys(),
	)
	prometheus.MustRegister(responseCodePerTunnel)

	serverLocations := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "server_locations",
			Help: "Where each tunnel is connected to. 1 means current location, 0 means previous locations.",
		},
		config.getLocationKeys(),
	)
	prometheus.MustRegister(serverLocations)

	return &TunnelMetrics{
		haConnections:                  haConnections,
		totalRequests:                  totalRequests,
		requestsPerTunnel:              requestsPerTunnel,
		concurrentRequestsPerTunnel:    concurrentRequestsPerTunnel,
		concurrentRequests:             make(map[string]uint64),
		maxConcurrentRequestsPerTunnel: maxConcurrentRequestsPerTunnel,
		maxConcurrentRequests:          make(map[string]uint64),
		timerRetries:                   timerRetries,
		responseByCode:                 responseByCode,
		responseCodePerTunnel:          responseCodePerTunnel,
		serverLocations:                serverLocations,
		oldServerLocations:             make(map[string]string),
		muxerMetrics:                   newMuxerMetrics(&config),
		labelConfig:                    config,
	}
}

// TunnelMetricsUpdater can update the set of tunnel metrics
type TunnelMetricsUpdater interface {
	incrementHaConnections()
	decrementHaConnections()
	updateMuxerMetrics(connectionID string, metrics *h2mux.MuxerMetrics)
	incrementRequests(connectionID string)
	decrementConcurrentRequests(connectionID string)
	incrementResponses(connectionID, code string)
	registerServerLocation(connectionID, loc string)
}

func (t *TunnelMetrics) setLabelValues(labelValues []string) error {
	return t.labelConfig.setLabelValues(labelValues)
}

// XXX must add labels
func (t *TunnelMetrics) incrementHaConnections() {
	t.haConnections.Inc()
}

func (t *TunnelMetrics) decrementHaConnections() {
	t.haConnections.Dec()
}

func (t *TunnelMetrics) updateMuxerMetrics(connectionID string, metrics *h2mux.MuxerMetrics) {
	t.muxerMetrics.update(t.labelConfig.getConnectionValues(connectionID), metrics)
}

func (t *TunnelMetrics) incrementRequests(connectionID string) {
	t.concurrentRequestsLock.Lock()
	var concurrentRequests uint64
	var ok bool
	if concurrentRequests, ok = t.concurrentRequests[connectionID]; ok {
		t.concurrentRequests[connectionID]++
		concurrentRequests++
	} else {
		t.concurrentRequests[connectionID] = 1
		concurrentRequests = 1
	}
	if maxConcurrentRequests, ok := t.maxConcurrentRequests[connectionID]; (ok && maxConcurrentRequests < concurrentRequests) || !ok {
		t.maxConcurrentRequests[connectionID] = concurrentRequests
		t.maxConcurrentRequestsPerTunnel.WithLabelValues(t.labelConfig.getConnectionValues(connectionID)...).Set(float64(concurrentRequests))
	}
	t.concurrentRequestsLock.Unlock()

	t.totalRequests.Inc()
	t.requestsPerTunnel.WithLabelValues(t.labelConfig.getConnectionValues(connectionID)...).Inc()
	t.concurrentRequestsPerTunnel.WithLabelValues(t.labelConfig.getConnectionValues(connectionID)...).Inc()
}

func (t *TunnelMetrics) decrementConcurrentRequests(connectionID string) {
	t.concurrentRequestsLock.Lock()
	if _, ok := t.concurrentRequests[connectionID]; ok {
		t.concurrentRequests[connectionID] -= 1
	}
	t.concurrentRequestsLock.Unlock()

	t.concurrentRequestsPerTunnel.WithLabelValues(t.labelConfig.getConnectionValues(connectionID)...).Dec()
}

func (t *TunnelMetrics) incrementResponses(connectionID, code string) {
	t.responseByCode.WithLabelValues(t.labelConfig.getStatusValues(connectionID, code)...).Inc()
	t.responseCodePerTunnel.WithLabelValues(t.labelConfig.getStatusValues(connectionID, code)...).Inc()

}

func (t *TunnelMetrics) registerServerLocation(connectionID, loc string) {
	t.locationLock.Lock()
	defer t.locationLock.Unlock()
	if oldLoc, ok := t.oldServerLocations[connectionID]; ok && oldLoc == loc {
		return
	} else if ok {
		t.serverLocations.WithLabelValues(t.labelConfig.getLocationValues(connectionID, oldLoc)...).Dec()
	}
	t.serverLocations.WithLabelValues(t.labelConfig.getLocationValues(connectionID, loc)...).Inc()
	t.oldServerLocations[connectionID] = loc
}
