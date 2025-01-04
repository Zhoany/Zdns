package config

import (
	"bufio"
	"log"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"
)

type ServerConfig struct {
	Port          string `yaml:"port"`
	BlockList     string `yaml:"blocklist"`
	V6            bool `yaml:"v6"`
	DefaultServer string `yaml:"defaultserver"`
}

type ForwardConfig struct {
	Server string `yaml:"server"`
	File   string `yaml:"file"`
	V6     bool   `yaml:"v6"` // 保留 v6 字段
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
}

// MatchDomain matches a domain and returns the closest upstream server
func (t *Trie) MatchDomain(domain string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Split the domain into parts and reverse the order
	parts := strings.Split(domain, ".")
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}

	node := t.root
	var lastMatchNode *TrieNode

	// Traverse the Trie to find the closest match
	for _, part := range parts {
		if nextNode, exists := node.children[part]; exists {
			node = nextNode
			if node.isEnd {
				lastMatchNode = node
			}
		} else {
			break
		}
	}

	if lastMatchNode != nil {
		return lastMatchNode.upstream, true
	}
	return "", false
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
		address := strings.TrimSpace(scanner.Text())
		addresses = append(addresses, address)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return addresses, nil
}
