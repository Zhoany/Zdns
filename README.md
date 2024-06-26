
# Zdns


## Project Overview

Zdns is a Go-based project for efficiently handling DNS requests. The project achieves caching, logging, configuration management, rule matching, and more through the collaboration of multiple modules.

## Quick Start

### Prerequisites

Before you begin, ensure you have the following tools installed on your system:

- Go programming environment (version 1.16 and above)
- Git

### Installation Steps

1. Clone the repository to your local machine

   ```sh
   git clone https://github.com/Zhoany/Zdns.git
   cd Zdns-new
   ```

2. Download dependencies

   ```sh
   go mod download
   ```

3. Run the project

   ```sh
   go run main.go
   ```

## Configuration

The configuration file is located at `conf/config.yaml`. You can modify this file as needed to suit your environment. Below is an explanation of the parameters in the configuration file.

### server

- `address`: The address and port the server listens on. Example: `":53530"`.
- `resolve_ipv6`: Boolean to enable or disable IPv6 resolution.
- `cache_size`: Size of the cache for DNS queries.
- `max_connects`: Maximum number of concurrent connections.
- `max_workers`: Maximum number of worker threads.
- `max_clients`: Maximum number of clients that can be handled simultaneously.
- `enable_logging`: Boolean to enable or disable logging.
- `log_max_size`: Maximum size of each log file in megabytes (MB).
- `log_max_backups`: Maximum number of backup log files to retain.

### upstream_servers

A list of upstream DNS servers. Each server entry includes:

- `address`: The  address of the upstream DNS server.
- `port`: The port of the upstream DNS server.
- `protocol`: The protocol used to communicate with the upstream DNS server (e.g., `UDP`).
- `domain_rules_file`: Path to the file containing domain-specific rules for this upstream server.

### blocklist_file

- `blocklist_file`: Path to the file containing blocked domains.

### common_upstream

Common upstream DNS server settings:

- `address`: The address of the common upstream DNS server.
- `port`: The port of the common upstream DNS server.
- `protocol`: The protocol used to communicate with the common upstream DNS server (e.g., `DoH` for DNS over HTTPS).

## Supported Protocols

- **UDP**: The server supports listening on UDP protocol and forwarding DNS requests over UDP.
- **DoH (DNS over HTTPS)**: The server supports forwarding DNS requests over HTTPS for enhanced security and privacy.

## Config Example
``` yaml
server:
  address: ":53530"
  resolve_ipv6: true
  cache_size: 2000
  max_connects: 500
  max_workers: 500
  max_clients: 7000
  enable_logging: true
  log_max_size: 50       
  log_max_backups: 7    

upstream_servers:
  - address: "223.5.5.5"
    port: "53"
    protocol: "UDP"
    domain_rules_file: "conf/china-list.txt"
  - address: "114.114.114.114"
    port: "53"
    protocol: "UDP"
    domain_rules_file: "conf/apple-cn.txt"
  - address: "https://cloudflare-dns.com/dns-query"
    port: "443"
    protocol: "DoH"
    domain_rules_file: "conf/other.txt"

blocklist_file: "conf/blocklist.txt"

common_upstream:
  address: "https://dns.google/dns-query"
  port: "443"
  protocol: "DoH"
```
