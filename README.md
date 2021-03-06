# Keepalived Exporter
[![Continuous Integration](https://github.com/cafebazaar/keepalived-exporter/workflows/Continuous%20Integration/badge.svg)](https://github.com/cafebazaar/keepalived-exporter/actions)

Prometheus exporter for [Keepalived](https://keepalived.org) metrics.

## Getting Started
To run it:
```
sudo ./keepalived-exporter [flags]
```
Help on flags
```
./keepalived-exporter --help
```

## Usage
Name               | Description
-------------------|------------
web.listen-address | Address to listen on for web interface and telemetry, defaults to `:2112`.
web.telemetry-path | A path under which to expose metrics, defaults to `/metrics`.
ka.json            | Send SIGJSON and decode JSON file instead of parsing text files, defaults to `false`.
ka.pid-path        | A path for Keepalived PID, defaults to `/var/run/keepalived.pid`.
cs                 | Health Cehck script path to be execute for each VIP.

**Note:** For `ka.json` option requirement is to have Keepalived compiled with `--enable-json` configure option.

## Metrics
| Metric                                          | Notes
|-------------------------------------------------|------------------------------------
| keepalived_up                                   | Status of Keepalived service
| keepalived_vrrp_state                           | State of vrrp
| keepalived_exporter_check_script_status         | Check Script status for each VIP
| keepalived_gratuitous_arp_delay_total           | Gratuitous ARP delay
| keepalived_advertisements_received_total        | Advertisements received
| keepalived_advertisements_sent_total            | Advertisements sent
| keepalived_become_master_total                  | Became master
| keepalived_release_master_total                 | Released master
| keepalived_packet_length_errors_total           | Packet length errors
| keepalived_advertisements_interval_errors_total | Advertisement interval errors
| keepalived_ip_ttl_errors_total                  | TTL errors
| keepalived_invalid_type_received_total          | Invalid type errors
| keepalived_address_list_errors_total            | Address list errors
| keepalived_authentication_invalid_total         | Authentication invalid
| keepalived_authentication_mismatch_total        | Authentication mismatch
| keepalived_authentication_failure_total         | Authentication failure
| keepalived_priority_zero_received_total         | Priority zero received
| keepalived_priority_zero_sent_total             | Priority zero sent
| keepalived_script_status                        | Tracker Script Status
| keepalived_script_state                         | Tracker Script State

## Check Script
You can specify a check script like Keepalived script check to check if all the things is okay or not.
The script will run for each VIP and gives an arg `$1` that contains VIP.

**Note:** The script should be executable.
```
chmod +x check_script.sh
```

### Sample Check Script
```
#!/usr/bin/env bash

ping $1 -c 1 -W 1
```
