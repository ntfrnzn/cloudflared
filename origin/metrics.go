package origin

import (
	"fmt"
	"time"

	"github.com/cloudflare/cloudflared/h2mux"

	"github.com/prometheus/client_golang/prometheus"
)

// TunnelMetrics contains pointers to the global prometheus metrics and their common label keys
type TunnelMetrics struct {
	connectionKey string
	locationKey   string
	statusKey     string
	commonKeys    []string

	haConnections prometheus.Gauge
	timerRetries  prometheus.Gauge

	requests        *prometheus.CounterVec
	responses       *prometheus.CounterVec
	serverLocations *prometheus.GaugeVec // XXX location should just be used as a label on other metrics

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

// TunnelMetricsUpdater separates the prometheus metrics and the update process
type TunnelMetricsUpdater interface {
	incrementHaConnections()
	decrementHaConnections()
	incrementRequests(connectionID string)
	decrementConcurrentRequests(connectionID string)
	incrementResponses(connectionID, code string)
	registerServerLocation(connectionID, loc string)
	updateMuxerMetrics(connectionID string, metrics *h2mux.MuxerMetrics)
}

type tunnelMetricsUpdater struct {

	// metrics is a set of pointers to prometheus metrics, configured globally
	metrics *TunnelMetrics

	// commonValues is group of label values that are set for this updater
	commonValues []string
}

// NewTunnelMetricsUpdater creates a metrics updater with common label values
func NewTunnelMetricsUpdater(metrics *TunnelMetrics, commonLabelValues []string) (TunnelMetricsUpdater, error) {

	if len(commonLabelValues) != len(metrics.commonKeys) {
		return nil, fmt.Errorf("failed to create updater, mismatched count of metrics label key (%v) and values (%v)", metrics.commonKeys, commonLabelValues)
	}
	return &tunnelMetricsUpdater{
		metrics:      metrics,
		commonValues: commonLabelValues,
	}, nil
}

// InitializeTunnelMetrics configures the prometheus metrics globally with common label keys
func InitializeTunnelMetrics(commonLabelKeys []string) *TunnelMetrics {

	connectionKey := "connection_id"
	locationKey := "location"
	statusKey := "status"

	// not a labelled vector
	haConnections := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ha_connections",
			Help: "Number of active HA connections",
		},
	)
	prometheus.MustRegister(haConnections)

	// not a labelled vector
	timerRetries := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "timer_retries",
			Help: "Unacknowledged heart beats count",
		})
	prometheus.MustRegister(timerRetries)

	requests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "requests",
			Help: "Count of requests",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(requests)

	responses := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "responses",
			Help: "Count of responses",
		},
		append(commonLabelKeys, connectionKey, statusKey),
	)
	prometheus.MustRegister(responses)

	serverLocations := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "server_locations",
			Help: "Where each tunnel is connected to. 1 means current location, 0 means previous locations.",
		},
		append(commonLabelKeys, connectionKey, locationKey),
	)
	prometheus.MustRegister(serverLocations)

	rtt := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rtt",
			Help: "Round-trip time in millisecond",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(rtt)

	rttMin := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rtt_min",
			Help: "Shortest round-trip time in millisecond",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(rttMin)

	rttMax := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rtt_max",
			Help: "Longest round-trip time in millisecond",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(rttMax)

	receiveWindowAve := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "receive_window_ave",
			Help: "Average receive window size in bytes",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(receiveWindowAve)

	sendWindowAve := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "send_window_ave",
			Help: "Average send window size in bytes",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(sendWindowAve)

	receiveWindowMin := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "receive_window_min",
			Help: "Smallest receive window size in bytes",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(receiveWindowMin)

	receiveWindowMax := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "receive_window_max",
			Help: "Largest receive window size in bytes",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(receiveWindowMax)

	sendWindowMin := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "send_window_min",
			Help: "Smallest send window size in bytes",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(sendWindowMin)

	sendWindowMax := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "send_window_max",
			Help: "Largest send window size in bytes",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(sendWindowMax)

	inBoundRateCurr := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "inbound_bytes_per_sec_curr",
			Help: "Current inbounding bytes per second, 0 if there is no incoming connection",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(inBoundRateCurr)

	inBoundRateMin := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "inbound_bytes_per_sec_min",
			Help: "Minimum non-zero inbounding bytes per second",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(inBoundRateMin)

	inBoundRateMax := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "inbound_bytes_per_sec_max",
			Help: "Maximum inbounding bytes per second",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(inBoundRateMax)

	outBoundRateCurr := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "outbound_bytes_per_sec_curr",
			Help: "Current outbounding bytes per second, 0 if there is no outgoing traffic",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(outBoundRateCurr)

	outBoundRateMin := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "outbound_bytes_per_sec_min",
			Help: "Minimum non-zero outbounding bytes per second",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(outBoundRateMin)

	outBoundRateMax := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "outbound_bytes_per_sec_max",
			Help: "Maximum outbounding bytes per second",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(outBoundRateMax)

	compBytesBefore := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "comp_bytes_before",
			Help: "Bytes sent via cross-stream compression, pre compression",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(compBytesBefore)

	compBytesAfter := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "comp_bytes_after",
			Help: "Bytes sent via cross-stream compression, post compression",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(compBytesAfter)

	compRateAve := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "comp_rate_ave",
			Help: "Average outbound cross-stream compression ratio",
		},
		append(commonLabelKeys, connectionKey),
	)
	prometheus.MustRegister(compRateAve)

	return &TunnelMetrics{

		connectionKey: connectionKey,
		locationKey:   locationKey,
		statusKey:     statusKey,
		commonKeys:    commonLabelKeys,

		haConnections: haConnections,
		timerRetries:  timerRetries,

		requests:        requests,
		responses:       responses,
		serverLocations: serverLocations,

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
		compRateAve:      compRateAve}
}

func (t *tunnelMetricsUpdater) incrementHaConnections() {
	t.metrics.haConnections.Inc()
}

func (t *tunnelMetricsUpdater) decrementHaConnections() {
	t.metrics.haConnections.Dec()
}

func convertRTTMilliSec(t time.Duration) float64 {
	return float64(t / time.Millisecond)
}
func (t *tunnelMetricsUpdater) updateMuxerMetrics(connectionID string, muxMetrics *h2mux.MuxerMetrics) {
	values := append(t.commonValues, connectionID)

	t.metrics.rtt.WithLabelValues(values...).Set(convertRTTMilliSec(muxMetrics.RTT))
	t.metrics.rttMin.WithLabelValues(values...).Set(convertRTTMilliSec(muxMetrics.RTTMin))
	t.metrics.rttMax.WithLabelValues(values...).Set(convertRTTMilliSec(muxMetrics.RTTMax))
	t.metrics.receiveWindowAve.WithLabelValues(values...).Set(muxMetrics.ReceiveWindowAve)
	t.metrics.sendWindowAve.WithLabelValues(values...).Set(muxMetrics.SendWindowAve)
	t.metrics.receiveWindowMin.WithLabelValues(values...).Set(float64(muxMetrics.ReceiveWindowMin))
	t.metrics.receiveWindowMax.WithLabelValues(values...).Set(float64(muxMetrics.ReceiveWindowMax))
	t.metrics.sendWindowMin.WithLabelValues(values...).Set(float64(muxMetrics.SendWindowMin))
	t.metrics.sendWindowMax.WithLabelValues(values...).Set(float64(muxMetrics.SendWindowMax))
	t.metrics.inBoundRateCurr.WithLabelValues(values...).Set(float64(muxMetrics.InBoundRateCurr))
	t.metrics.inBoundRateMin.WithLabelValues(values...).Set(float64(muxMetrics.InBoundRateMin))
	t.metrics.inBoundRateMax.WithLabelValues(values...).Set(float64(muxMetrics.InBoundRateMax))
	t.metrics.outBoundRateCurr.WithLabelValues(values...).Set(float64(muxMetrics.OutBoundRateCurr))
	t.metrics.outBoundRateMin.WithLabelValues(values...).Set(float64(muxMetrics.OutBoundRateMin))
	t.metrics.outBoundRateMax.WithLabelValues(values...).Set(float64(muxMetrics.OutBoundRateMax))
	t.metrics.compBytesBefore.WithLabelValues(values...).Set(float64(muxMetrics.CompBytesBefore.Value()))
	t.metrics.compBytesAfter.WithLabelValues(values...).Set(float64(muxMetrics.CompBytesAfter.Value()))
	t.metrics.compRateAve.WithLabelValues(values...).Set(float64(muxMetrics.CompRateAve()))
}

func (t *tunnelMetricsUpdater) incrementRequests(connectionID string) {
	values := append(t.commonValues, connectionID)
	t.metrics.requests.WithLabelValues(values...).Inc()
}

func (t *tunnelMetricsUpdater) decrementConcurrentRequests(connectionID string) {
	// TODO
}

func (t *tunnelMetricsUpdater) incrementResponses(connectionID, code string) {
	values := append(t.commonValues, connectionID, code)

	t.metrics.responses.WithLabelValues(values...).Inc()
}

// registerServerLocation should be renamed to countServerConnectionEvents or something like that
func (t *tunnelMetricsUpdater) registerServerLocation(connectionID, loc string) {

	values := append(t.commonValues, connectionID, loc)
	// TODO
	t.metrics.serverLocations.WithLabelValues(values...).Inc()
}
