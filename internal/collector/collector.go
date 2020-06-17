package collector

import (
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

// KeepalivedCollector implements prometheus.Collector interface and stores required info to collect data
type KeepalivedCollector struct {
	sync.Mutex
	runningSignal     bool
	failedStatsSignal bool
	useJSON           bool
	pidPath           string
	scriptPath        string
	SIGDATA           int
	SIGJSON           int
	SIGSTATS          int
	metrics           map[string]*prometheus.Desc
}

// VRRPStats represents Keepalived stats about VRRP
type VRRPStats struct {
	AdvertRcvd        int `json:"advert_rcvd"`
	AdvertSent        int `json:"advert_sent"`
	BecomeMaster      int `json:"become_master"`
	ReleaseMaster     int `json:"release_master"`
	PacketLenErr      int `json:"packet_len_err"`
	AdvertIntervalErr int `json:"advert_interval_err"`
	IPTTLErr          int `json:"ip_ttl_err"`
	InvalidTypeRcvd   int `json:"invalid_type_rcvd"`
	AddrListErr       int `json:"addr_list_err"`
	InvalidAuthType   int `json:"invalid_authtype"`
	AuthTypeMismatch  int `json:"authtype_mismatch"`
	AuthFailure       int `json:"auth_failure"`
	PRIZeroRcvd       int `json:"pri_zero_rcvd"`
	PRIZeroSent       int `json:"pri_zero_sent"`
}

// VRRPData represents Keepalived data about VRRP
type VRRPData struct {
	IName     string   `json:"iname"`
	State     int      `json:"state"`
	WantState int      `json:"wantstate"`
	Intf      string   `json:"ifp_ifname"`
	GArpDelay int      `json:"garp_delay"`
	VRID      int      `json:"vrid"`
	VIPs      []string `json:"vips"`
}

// VRRPScript represents Keepalived script about VRRP
type VRRPScript struct {
	Name   string
	Status string
	State  string
}

// VRRP ties together VRRPData and VRRPStats
type VRRP struct {
	Data  VRRPData  `json:"data"`
	Stats VRRPStats `json:"stats"`
}

// KeepalivedStats ties together VRRP and VRRPScript
type KeepalivedStats struct {
	VRRPs   []VRRP
	Scripts []VRRPScript
}

// NewKeepalivedCollector is creating new instance of KeepalivedCollector
func NewKeepalivedCollector(useJSON bool, pidPath, scriptPath string) *KeepalivedCollector {
	kc := &KeepalivedCollector{
		useJSON:           useJSON,
		pidPath:           pidPath,
		scriptPath:        scriptPath,
		runningSignal:     false,
		failedStatsSignal: false,
	}

	commonLabels := []string{"iname", "intf", "vrid", "state"}
	kc.metrics = map[string]*prometheus.Desc{
		"keepalived_up":                                   prometheus.NewDesc("keepalived_up", "Status", nil, nil),
		"keepalived_vrrp_state":                           prometheus.NewDesc("keepalived_vrrp_state", "State of vrrp", []string{"iname", "intf", "vrid", "ip_address"}, nil),
		"keepalived_exporter_check_script_status":         prometheus.NewDesc("keepalived_exporter_check_script_status", "Check Script status for each VIP", []string{"iname", "intf", "vrid", "ip_address"}, nil),
		"keepalived_gratuitous_arp_delay_total":           prometheus.NewDesc("keepalived_gratuitous_arp_delay_total", "Gratuitous ARP delay", commonLabels, nil),
		"keepalived_advertisements_received_total":        prometheus.NewDesc("keepalived_advertisements_received_total", "Advertisements received", commonLabels, nil),
		"keepalived_advertisements_sent_total":            prometheus.NewDesc("keepalived_advertisements_sent_total", "Advertisements sent", commonLabels, nil),
		"keepalived_become_master_total":                  prometheus.NewDesc("keepalived_become_master_total", "Became master", commonLabels, nil),
		"keepalived_release_master_total":                 prometheus.NewDesc("keepalived_release_master_total", "Released master", commonLabels, nil),
		"keepalived_packet_length_errors_total":           prometheus.NewDesc("keepalived_packet_length_errors_total", "Packet length errors", commonLabels, nil),
		"keepalived_advertisements_interval_errors_total": prometheus.NewDesc("keepalived_advertisements_interval_errors_total", "Advertisement interval errors", commonLabels, nil),
		"keepalived_ip_ttl_errors_total":                  prometheus.NewDesc("keepalived_ip_ttl_errors_total", "TTL errors", commonLabels, nil),
		"keepalived_invalid_type_received_total":          prometheus.NewDesc("keepalived_invalid_type_received_total", "Invalid type errors", commonLabels, nil),
		"keepalived_address_list_errors_total":            prometheus.NewDesc("keepalived_address_list_errors_total", "Address list errors", commonLabels, nil),
		"keepalived_authentication_invalid_total":         prometheus.NewDesc("keepalived_authentication_invalid_total", "Authentication invalid", commonLabels, nil),
		"keepalived_authentication_mismatch_total":        prometheus.NewDesc("keepalived_authentication_mismatch_total", "Authentication mismatch", commonLabels, nil),
		"keepalived_authentication_failure_total":         prometheus.NewDesc("keepalived_authentication_failure_total", "Authentication failure", commonLabels, nil),
		"keepalived_priority_zero_received_total":         prometheus.NewDesc("keepalived_priority_zero_received_total", "Priority zero received", commonLabels, nil),
		"keepalived_priority_zero_sent_total":             prometheus.NewDesc("keepalived_priority_zero_sent_total", "Priority zero sent", commonLabels, nil),
		"keepalived_script_status":                        prometheus.NewDesc("keepalived_script_status", "Tracker Script Status", []string{"name"}, nil),
		"keepalived_script_state":                         prometheus.NewDesc("keepalived_script_state", "Tracker Script State", []string{"name"}, nil),
	}

	if kc.useJSON {
		kc.SIGJSON = sigNum("JSON")
	}
	kc.SIGDATA = sigNum("DATA")
	kc.SIGSTATS = sigNum("STATS")

	return kc
}

func (k *KeepalivedCollector) newConstMetric(ch chan<- prometheus.Metric, name string, valueType prometheus.ValueType, value float64, lableValues ...string) {
	// TODO: Why constMetric?
	pm, err := prometheus.NewConstMetric(
		k.metrics[name],
		valueType,
		value,
		lableValues...,
	)
	if err != nil {
		logrus.WithError(err).Errorf("Failed to register %q metric", name)
		return
	}

	ch <- pm
}

// Collect get metrics and add to prometheus metric channel
func (k *KeepalivedCollector) Collect(ch chan<- prometheus.Metric) {
	k.Lock()
	defer k.Unlock()

	keepalivedUp := float64(1)

	keepalivedStats, err := k.stats()
	if err != nil {
		logrus.WithField("json", k.useJSON).WithError(err).Error("No data found to be exported")
		keepalivedUp = 0
	}

	k.newConstMetric(ch, "keepalived_up", prometheus.GaugeValue, keepalivedUp)

	if keepalivedUp == 0 {
		return
	}

	for _, vrrp := range keepalivedStats.VRRPs {
		state := ""
		ok := false
		if state, ok = vrrp.Data.getStringState(vrrp.Data.State); !ok {
			logrus.WithField("state", vrrp.Data.State).Warn("Unknown State found for vrrp: ", vrrp.Data.IName)
		}

		k.newConstMetric(ch, "keepalived_advertisements_received_total", prometheus.CounterValue, float64(vrrp.Stats.AdvertRcvd), vrrp.Data.IName, vrrp.Data.Intf, strconv.Itoa(vrrp.Data.VRID), state)
		k.newConstMetric(ch, "keepalived_advertisements_sent_total", prometheus.CounterValue, float64(vrrp.Stats.AdvertSent), vrrp.Data.IName, vrrp.Data.Intf, strconv.Itoa(vrrp.Data.VRID), state)
		k.newConstMetric(ch, "keepalived_become_master_total", prometheus.CounterValue, float64(vrrp.Stats.BecomeMaster), vrrp.Data.IName, vrrp.Data.Intf, strconv.Itoa(vrrp.Data.VRID), state)
		k.newConstMetric(ch, "keepalived_release_master_total", prometheus.CounterValue, float64(vrrp.Stats.ReleaseMaster), vrrp.Data.IName, vrrp.Data.Intf, strconv.Itoa(vrrp.Data.VRID), state)
		k.newConstMetric(ch, "keepalived_packet_length_errors_total", prometheus.CounterValue, float64(vrrp.Stats.PacketLenErr), vrrp.Data.IName, vrrp.Data.Intf, strconv.Itoa(vrrp.Data.VRID), state)
		k.newConstMetric(ch, "keepalived_advertisements_interval_errors_total", prometheus.CounterValue, float64(vrrp.Stats.AdvertIntervalErr), vrrp.Data.IName, vrrp.Data.Intf, strconv.Itoa(vrrp.Data.VRID), state)
		k.newConstMetric(ch, "keepalived_ip_ttl_errors_total", prometheus.CounterValue, float64(vrrp.Stats.IPTTLErr), vrrp.Data.IName, vrrp.Data.Intf, strconv.Itoa(vrrp.Data.VRID), state)
		k.newConstMetric(ch, "keepalived_invalid_type_received_total", prometheus.CounterValue, float64(vrrp.Stats.InvalidTypeRcvd), vrrp.Data.IName, vrrp.Data.Intf, strconv.Itoa(vrrp.Data.VRID), state)
		k.newConstMetric(ch, "keepalived_address_list_errors_total", prometheus.CounterValue, float64(vrrp.Stats.AddrListErr), vrrp.Data.IName, vrrp.Data.Intf, strconv.Itoa(vrrp.Data.VRID), state)
		k.newConstMetric(ch, "keepalived_authentication_invalid_total", prometheus.CounterValue, float64(vrrp.Stats.InvalidAuthType), vrrp.Data.IName, vrrp.Data.Intf, strconv.Itoa(vrrp.Data.VRID), state)
		k.newConstMetric(ch, "keepalived_authentication_mismatch_total", prometheus.CounterValue, float64(vrrp.Stats.AuthTypeMismatch), vrrp.Data.IName, vrrp.Data.Intf, strconv.Itoa(vrrp.Data.VRID), state)
		k.newConstMetric(ch, "keepalived_authentication_failure_total", prometheus.CounterValue, float64(vrrp.Stats.AuthFailure), vrrp.Data.IName, vrrp.Data.Intf, strconv.Itoa(vrrp.Data.VRID), state)
		k.newConstMetric(ch, "keepalived_priority_zero_received_total", prometheus.CounterValue, float64(vrrp.Stats.PRIZeroRcvd), vrrp.Data.IName, vrrp.Data.Intf, strconv.Itoa(vrrp.Data.VRID), state)
		k.newConstMetric(ch, "keepalived_priority_zero_sent_total", prometheus.CounterValue, float64(vrrp.Stats.PRIZeroSent), vrrp.Data.IName, vrrp.Data.Intf, strconv.Itoa(vrrp.Data.VRID), state)
		k.newConstMetric(ch, "keepalived_gratuitous_arp_delay_total", prometheus.CounterValue, float64(vrrp.Data.GArpDelay), vrrp.Data.IName, vrrp.Data.Intf, strconv.Itoa(vrrp.Data.VRID), state)

		for _, ip := range vrrp.Data.VIPs {
			ipAddr := strings.Split(ip, " ")[0]
			intf := strings.Split(ip, " ")[2]

			k.newConstMetric(ch, "keepalived_vrrp_state", prometheus.GaugeValue, float64(vrrp.Data.State), vrrp.Data.IName, intf, strconv.Itoa(vrrp.Data.VRID), ipAddr)

			if k.scriptPath != "" {
				ok := k.checkScript(ipAddr)
				checkScript := float64(0)
				if ok {
					checkScript = 1
				}
				k.newConstMetric(ch, "keepalived_exporter_check_script_status", prometheus.GaugeValue, checkScript, vrrp.Data.IName, intf, strconv.Itoa(vrrp.Data.VRID), ipAddr)
			}
		}
	}

	for _, script := range keepalivedStats.Scripts {
		if scriptStatus, ok := script.getIntStatus(); !ok {
			logrus.WithFields(logrus.Fields{"status": script.Status, "name": script.Name}).Warn("Unknown status")
		} else {
			k.newConstMetric(ch, "keepalived_script_status", prometheus.GaugeValue, float64(scriptStatus), script.Name)
		}

		if scriptState, ok := script.getIntState(script.State); !ok {
			logrus.WithFields(logrus.Fields{"state": script.State, "name": script.Name}).Warn("Unknown state")
		} else {
			k.newConstMetric(ch, "keepalived_script_state", prometheus.GaugeValue, float64(scriptState), script.Name)
		}
	}
}

func (k *KeepalivedCollector) checkScript(vip string) bool {
	script := k.scriptPath + " " + vip
	out, err := exec.Command("/bin/sh", "-c", script).Output()
	if err != nil {
		logrus.WithFields(logrus.Fields{"VIP": vip, "output": string(out)}).WithError(err).Error("Check script failed")
		return false
	}
	return true
}

// Describe outputs metrics descriptions
func (k *KeepalivedCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range k.metrics {
		ch <- m
	}
}
