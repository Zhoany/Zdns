package config

import (
	"bufio"
	//"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"
	//"ZZDNS/logger" // 替换为实际路径
)

type ServerConfig struct {
	Port          string `yaml:"port"`
	IPv6          bool   `yaml:"ipv6"`
	BlockList     string `yaml:"blocklist"`
	DefaultServer string `yaml:"defaultserver"`
}

type ForwardConfig struct {
	Server string `yaml:"server"`
	File   string `yaml:"file"`
}

type Config struct {
	Server  ServerConfig    `yaml:"Server"`
	Forward []ForwardConfig `yaml:"Forward"`
}

// TrieNode represents a node in the Trie
type TrieNode struct {
	children map[string]*TrieNode
	upstream string
	isEnd    bool
}

// Trie structure
type Trie struct {
	root *TrieNode
	mu   sync.RWMutex
}

// NewTrie creates a new Trie
func NewTrie() *Trie {
	return &Trie{
		root: &TrieNode{
			children: make(map[string]*TrieNode),
		},
	}
}

// AddDomainRule adds a domain rule to the Trie
func (t *Trie) AddDomainRule(domain, upstream string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Split the domain into parts and reverse the order
	parts := strings.Split(domain, ".")
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}

	node := t.root
	
	for _, part := range parts {
		
		if _, exists := node.children[part]; !exists {
			node.children[part] = &TrieNode{
				children: make(map[string]*TrieNode),
			}
		}
		node = node.children[part]
	}
	node.isEnd = true
	node.upstream = upstream

	// 记录添加的规则
	//logger.GetLogger().Info(fmt.Sprintf("Added domain rule: %s, Upstream: %s, Path: %s", domain, upstream, strings.Join(path, " -> ")))
}



// MatchDomain matches a domain and returns the closest upstream server
func (t *Trie) MatchDomain(domain string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// 将域名分成各部分并反向顺序
	parts := strings.Split(domain, ".")
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}

	node := t.root
	var path []string
	var lastMatchNode *TrieNode

	// 遍历 Trie，记录最后一个匹配的节点
	for _, part := range parts {
		if nextNode, exists := node.children[part]; exists {
			node = nextNode
			path = append(path, part)
			if node.isEnd {
				lastMatchNode = node
			}
		} else {
			break
		}
	}

	if lastMatchNode != nil {
		// 记录匹配到的路径和最终的匹配结果
		//logger.GetLogger().Info(fmt.Sprintf("Matched Path: %s", strings.Join(path, " -> ")))
		//logger.GetLogger().Info(fmt.Sprintf("Matched Rule: Domain: %s, Upstream Server: %s", strings.Join(reverse(parts), "."), lastMatchNode.upstream))
		return lastMatchNode.upstream, true
	}
	return "", false
}

// reverse reverses the order of elements in a slice of strings
func reverse(parts []string) []string {
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return parts
}

// Global configuration variables
var (
	CFG        *Config
	DomainTrie *Trie
	Blocklist  map[string]struct{}
)

// LoadConfig reads and parses the configuration file
func LoadConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("Error reading config file: %v", err)
		return err
	}

	err = yaml.Unmarshal(data, &CFG)
	if err != nil {
		log.Printf("Error parsing config file: %v", err)
		return err
	}

	// Initialize the DomainTrie
	DomainTrie = NewTrie()

	// Initialize the Blocklist
	Blocklist = make(map[string]struct{})
	blocks, err := readAddressesFromFile(CFG.Server.BlockList)
	if err != nil {
		log.Printf("Error reading address file %s: %v", CFG.Server.BlockList, err)
		return err
	}
	for _, domain := range blocks {
		Blocklist[domain] = struct{}{}
	}

	// Process Forward address files
	for _, forward := range CFG.Forward {
		addresses, err := readAddressesFromFile(forward.File)
		if err != nil {
			log.Printf("Error reading address file %s: %v", forward.File, err)
			return err
		}
		for _, domain := range addresses {
			DomainTrie.AddDomainRule(domain, forward.Server)
		}
	}

	return nil
}

// readAddressesFromFile reads addresses from a given file and returns them as a slice of strings
func readAddressesFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var addresses []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		address := strings.TrimSuffix(scanner.Text(), ".")
		addresses = append(addresses, address)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return addresses, nil
}