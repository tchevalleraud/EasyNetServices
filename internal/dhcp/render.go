package dhcp

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// object is a JSON object of the Kea configuration.
type object = map[string]any

// renderInput is what the configuration of one Kea server is generated from.
type renderInput struct {
	Family   Family
	Settings Settings
	Subnets  []Subnet
	// DataDir holds the lease files; Kea only accepts files in the directory
	// given by KEA_DHCP_DATA_DIR.
	DataDir string
}

// socketName returns the name of the control socket of a family, created by
// Kea in KEA_CONTROL_SOCKET_DIR.
func socketName(f Family) string {
	if f == V6 {
		return "kea6-ctrl-socket"
	}
	return "kea4-ctrl-socket"
}

// render returns the configuration file of the Kea server of a family.
func render(in renderInput) []byte {
	f := in.Family
	fs := in.Settings.Family(f)
	daemon := "kea-dhcp4"
	if f == V6 {
		daemon = "kea-dhcp6"
	}

	ifaces := object{
		"interfaces": orEmpty(in.Settings.Interfaces),
		// Keep retrying while an interface is not ready, e.g. at boot.
		"service-sockets-require-all":     false,
		"service-sockets-retry-wait-time": 5000,
		"service-sockets-max-retries":     720,
	}
	if f == V4 {
		// Raw sockets receive the requests of clients without an address.
		ifaces["dhcp-socket-type"] = "raw"
	}

	conf := object{
		"interfaces-config": ifaces,
		"control-sockets":   []object{{"socket-type": "unix", "socket-name": socketName(f)}},
		"lease-database": object{
			"type":         "memfile",
			"persist":      true,
			"name":         filepath.Join(in.DataDir, "kea-leases"+string(f[1])+".csv"),
			"lfc-interval": 3600,
		},
		"hooks-libraries": []object{{"library": "libdhcp_lease_cmds.so"}},
		"multi-threading": object{
			"enable-multi-threading": true,
			"thread-pool-size":       4,
			"packet-queue-size":      64,
		},
		"valid-lifetime": fs.ValidLifetime,
		"option-data":    optionData(f, fs.Options, ""),
		"loggers": []object{
			{
				"name":           daemon,
				"output-options": []object{{"output": "stdout", "pattern": "%-5p [%c] %m\n"}},
				"severity":       "INFO",
			},
		},
	}
	// One message per packet, or per command of the management server which
	// polls the status, is too verbose for the container logs.
	for _, quiet := range []string{"packets", "commands"} {
		conf["loggers"] = append(conf["loggers"].([]object), object{
			"name":           daemon + "." + quiet,
			"output-options": []object{{"output": "stdout", "pattern": "%-5p [%c] %m\n"}},
			"severity":       "WARN",
		})
	}
	if f == V6 {
		conf["preferred-lifetime"] = fs.PreferredLifetime
	}

	subnets := make([]object, 0, len(in.Subnets))
	for _, s := range in.Subnets {
		subnets = append(subnets, renderSubnet(f, fs, s))
	}

	key, subnetKey := "Dhcp4", "subnet4"
	if f == V6 {
		key, subnetKey = "Dhcp6", "subnet6"
	}
	conf[subnetKey] = subnets

	data, _ := json.MarshalIndent(object{key: conf}, "", "  ")
	return append(data, '\n')
}

func renderSubnet(f Family, fs *FamilySettings, s Subnet) object {
	pools := make([]object, 0, len(s.Pools))
	for _, p := range s.Pools {
		pools = append(pools, object{"pool": p})
	}
	subnet := object{
		"id":          s.ID,
		"subnet":      s.Subnet,
		"pools":       pools,
		"option-data": optionData(f, s.Options, s.Gateway),
	}
	if s.Interface != "" {
		subnet["interface"] = s.Interface
	}
	if s.ValidLifetime != 0 {
		subnet["valid-lifetime"] = s.ValidLifetime
		if f == V6 {
			subnet["preferred-lifetime"] = min(fs.PreferredLifetime, s.ValidLifetime)
		}
	}

	reservations := make([]object, 0, len(s.Reservations))
	for _, r := range s.Reservations {
		res := object{}
		switch {
		case r.DUID != "":
			res["duid"] = r.DUID
		default:
			res["hw-address"] = r.HWAddress
		}
		if f == V6 {
			res["ip-addresses"] = []string{r.IPAddress}
		} else {
			res["ip-address"] = r.IPAddress
		}
		if r.Hostname != "" {
			res["hostname"] = r.Hostname
		}
		reservations = append(reservations, res)
	}
	subnet["reservations"] = reservations
	return subnet
}

// optionData returns the Kea option-data of a set of options. Empty options
// are left out so that they are inherited from the global level.
func optionData(f Family, o Options, gateway string) []object {
	data := []object{}
	add := func(name string, values []string) {
		if len(values) > 0 {
			data = append(data, object{"name": name, "data": strings.Join(values, ", ")})
		}
	}
	if f == V4 {
		if gateway != "" {
			add("routers", []string{gateway})
		}
		add("domain-name-servers", o.DNSServers)
		if o.DomainName != "" {
			add("domain-name", []string{o.DomainName})
		}
		add("domain-search", o.DomainSearch)
		add("ntp-servers", o.NTPServers)
	} else {
		add("dns-servers", o.DNSServers)
		add("domain-search", o.DomainSearch)
		add("sntp-servers", o.NTPServers)
	}
	return data
}

func orEmpty(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}
