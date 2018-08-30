package origin

import (
	"fmt"
	"time"

	"github.com/cloudflare/cloudflared/h2mux"

	"github.com/prometheus/client_golang/prometheus"
)

type TunnelMetrics struct {
	haConnections prometheus.Gauge
	timerRetries  prometheus.Gauge

	requests        *prometheus.CounterVec
	responses       *prometheus.CounterVec
	serverLocations *prometheus.GaugeVec

	connectionKey string
	locationKey   string
	statusKey     string
	commonKeys    []string

	muxerMetrics *muxerMetrics
}

type tunnelMetricsUpdater struct {
	metrics      *TunnelMetrics
	commonValues []string
	muxerUpdater *muxerMetricsUpdater
}

type TunnelMetricsUpdater interface {
	incrementHaConnections()
	decrementHaConnections()
	updateMuxerMetrics(connectionID string, metrics *h2mux.MuxerMetrics)
	incrementRequests(connectionID string)
	decrementConcurrentRequests(connectionID string)
	incrementResponses(connectionID, code string)
	registerServerLocation(connectionID, loc string)
}

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

type muxerMetricsUpdater struct {
	metrics *muxerMetrics
}

func newMuxerMetrics() *muxerMetrics {
	rtt := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rtt",
			Help: "Round-trip time in millisecond",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(rtt)

	rttMin := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rtt_min",
			Help: "Shortest round-trip time in millisecond",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(rttMin)

	rttMax := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rtt_max",
			Help: "Longest round-trip time in millisecond",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(rttMax)

	receiveWindowAve := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "receive_window_ave",
			Help: "Average receive window size in bytes",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(receiveWindowAve)

	sendWindowAve := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "send_window_ave",
			Help: "Average send window size in bytes",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(sendWindowAve)

	receiveWindowMin := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "receive_window_min",
			Help: "Smallest receive window size in bytes",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(receiveWindowMin)

	receiveWindowMax := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "receive_window_max",
			Help: "Largest receive window size in bytes",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(receiveWindowMax)

	sendWindowMin := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "send_window_min",
			Help: "Smallest send window size in bytes",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(sendWindowMin)

	sendWindowMax := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "send_window_max",
			Help: "Largest send window size in bytes",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(sendWindowMax)

	inBoundRateCurr := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "inbound_bytes_per_sec_curr",
			Help: "Current inbounding bytes per second, 0 if there is no incoming connection",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(inBoundRateCurr)

	inBoundRateMin := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "inbound_bytes_per_sec_min",
			Help: "Minimum non-zero inbounding bytes per second",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(inBoundRateMin)

	inBoundRateMax := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "inbound_bytes_per_sec_max",
			Help: "Maximum inbounding bytes per second",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(inBoundRateMax)

	outBoundRateCurr := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "outbound_bytes_per_sec_curr",
			Help: "Current outbounding bytes per second, 0 if there is no outgoing traffic",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(outBoundRateCurr)

	outBoundRateMin := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "outbound_bytes_per_sec_min",
			Help: "Minimum non-zero outbounding bytes per second",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(outBoundRateMin)

	outBoundRateMax := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "outbound_bytes_per_sec_max",
			Help: "Maximum outbounding bytes per second",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(outBoundRateMax)

	compBytesBefore := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "comp_bytes_before",
			Help: "Bytes sent via cross-stream compression, pre compression",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(compBytesBefore)

	compBytesAfter := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "comp_bytes_after",
			Help: "Bytes sent via cross-stream compression, post compression",
		},
		[]string{"connection_id"},
	)
	prometheus.MustRegister(compBytesAfter)

	compRateAve := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "comp_rate_ave",
			Help: "Average outbound cross-stream compression ratio",
		},
		[]string{"connection_id"},
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

func (m *muxerMetrics) update(connectionID string, metrics *h2mux.MuxerMetrics) {
	m.rtt.WithLabelValues(connectionID).Set(convertRTTMilliSec(metrics.RTT))
	m.rttMin.WithLabelValues(connectionID).Set(convertRTTMilliSec(metrics.RTTMin))
	m.rttMax.WithLabelValues(connectionID).Set(convertRTTMilliSec(metrics.RTTMax))
	m.receiveWindowAve.WithLabelValues(connectionID).Set(metrics.ReceiveWindowAve)
	m.sendWindowAve.WithLabelValues(connectionID).Set(metrics.SendWindowAve)
	m.receiveWindowMin.WithLabelValues(connectionID).Set(float64(metrics.ReceiveWindowMin))
	m.receiveWindowMax.WithLabelValues(connectionID).Set(float64(metrics.ReceiveWindowMax))
	m.sendWindowMin.WithLabelValues(connectionID).Set(float64(metrics.SendWindowMin))
	m.sendWindowMax.WithLabelValues(connectionID).Set(float64(metrics.SendWindowMax))
	m.inBoundRateCurr.WithLabelValues(connectionID).Set(float64(metrics.InBoundRateCurr))
	m.inBoundRateMin.WithLabelValues(connectionID).Set(float64(metrics.InBoundRateMin))
	m.inBoundRateMax.WithLabelValues(connectionID).Set(float64(metrics.InBoundRateMax))
	m.outBoundRateCurr.WithLabelValues(connectionID).Set(float64(metrics.OutBoundRateCurr))
	m.outBoundRateMin.WithLabelValues(connectionID).Set(float64(metrics.OutBoundRateMin))
	m.outBoundRateMax.WithLabelValues(connectionID).Set(float64(metrics.OutBoundRateMax))
	m.compBytesBefore.WithLabelValues(connectionID).Set(float64(metrics.CompBytesBefore.Value()))
	m.compBytesAfter.WithLabelValues(connectionID).Set(float64(metrics.CompBytesAfter.Value()))
	m.compRateAve.WithLabelValues(connectionID).Set(float64(metrics.CompRateAve()))
}

func convertRTTMilliSec(t time.Duration) float64 {
	return float64(t / time.Millisecond)
}

func NewTunnelMetricsUpdater(metrics *TunnelMetrics, commonLabelValues []string) (TunnelMetricsUpdater, error) {

	if len(commonLabelValues) != len(metrics.commonKeys) {
		return nil, fmt.Errorf("Mismatched count of metrics label key (%v) and values (%v)", metrics.commonKeys, commonLabelValues)
	}

	return &tunnelMetricsUpdater{
		metrics:      metrics,
		commonValues: commonLabelValues,
	}, nil
}

// Metrics that can be collected without asking the edge
func InitializeTunnelMetrics(commonLabelKeys []string) *TunnelMetrics {

	connectionKey := "connection_id"
	locationKey := "location"
	statusKey := "status"

	haConnections := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ha_connections",
			Help: "Number of active ha connections",
		},
	)
	prometheus.MustRegister(haConnections)

	requests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "requests_per_tunnel",
			Help: "Amount of requests proxied through each tunnel",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(requests)

	responses := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "response_code_per_tunnel",
			Help: "Count of responses by HTTP status code fore each tunnel",
		},
		append(commonLabelKeys, connectionKey, statusKey),
	)
	prometheus.MustRegister(responses)

	timerRetries := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "timer_retries",
			Help: "Unacknowledged heart beats count",
		})
	prometheus.MustRegister(timerRetries)

	serverLocations := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "server_locations",
			Help: "Where each tunnel is connected to. 1 means current location, 0 means previous locations.",
		},
		append(commonLabelKeys, connectionKey, locationKey),
	)
	prometheus.MustRegister(serverLocations)

	return &TunnelMetrics{
		haConnections: haConnections,
		timerRetries:  timerRetries,

		requests:        requests,
		responses:       responses,
		serverLocations: serverLocations,

		connectionKey: connectionKey,
		locationKey:   locationKey,
		statusKey:     statusKey,
		commonKeys:    commonLabelKeys,

		muxerMetrics: newMuxerMetrics(),
	}
}

func (t *tunnelMetricsUpdater) incrementHaConnections() {
	t.metrics.haConnections.Inc()
}

func (t *tunnelMetricsUpdater) decrementHaConnections() {
	t.metrics.haConnections.Dec()
}

func (t *tunnelMetricsUpdater) updateMuxerMetrics(connectionID string, metrics *h2mux.MuxerMetrics) {
	// t.muxerUpdater.muxerMetrics.update(connectionID, metrics)
}

func (t *tunnelMetricsUpdater) incrementRequests(connectionID string) {
	values := append(t.commonValues, connectionID)
	t.metrics.requests.WithLabelValues(values...).Inc()
}

func (t *tunnelMetricsUpdater) decrementConcurrentRequests(connectionID string) {
}

func (t *tunnelMetricsUpdater) incrementResponses(connectionID, code string) {
	values := append(t.commonValues, connectionID, code)

	t.metrics.responses.WithLabelValues(values...).Inc()
}

// registerServerLocation should be renamed to countServerConnectionEvents or something like that
func (t *tunnelMetricsUpdater) registerServerLocation(connectionID, loc string) {

	values := append(t.commonValues, connectionID, loc)

	t.metrics.serverLocations.WithLabelValues(values...).Inc()
}
