package config

import (
	"ZZDNS/shared"
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// —— YAML 原生结构 ——

type ServerConfig struct {
	Port          string `yaml:"port"`
	// DefaultServer 与 V6 不再持久化，而是通过在 DomainRule 中创建 "*" 记录来实现
	DefaultServer string `yaml:"defaultserver"`
	V6            bool   `yaml:"v6"`
	BlockList     string `yaml:"blocklist"`
	HostsFile     string `yaml:"hostsfile"`
}

type ForwardConfig struct {
	Name   string `yaml:"name"`
	Server string `yaml:"server"`
	File   string `yaml:"file"`
	V6     bool   `yaml:"v6"`
}

type APIConfig struct {
	Port      int    `yaml:"port"`
	User      string `yaml:"user"`
	Password  string `yaml:"password"`
	JWTSecret string `yaml:"jwt_secret"`
}

type Config struct {
	Server  ServerConfig    `yaml:"server"`
	API     APIConfig       `yaml:"api"`
	Forward []ForwardConfig `yaml:"forward"`
}

// —— 兼容旧结构（不再持久化 / 读写） ——

// ConfigMsg 原本用于持久化 API 凭证 / 端口等，现在通过环境变量注入，保留结构以防其他地方引用
type ConfigMsg struct {
	ID           uint   `gorm:"primaryKey"`
	Port         string `gorm:"default:5300"`
	APIPort      int    `gorm:"default:9898"`
	APIUser      string
	APIPassword  string
	APIJWTSecret string
}

// DomainRule 合并了 Name 与 V6 字段，同时用 Domain="*"、Name="default" 表示默认转发

// Group 存储每一组的上游、支持情况、优先级等
type Group struct {
	ID       uint   `gorm:"primaryKey"`
	Name     string `gorm:"uniqueIndex"` // 组名
	Upstream string
	V6       bool
	Priority int // 优先级，数值越小优先级越高
}

// Domain 存储域名到 Group 的映射
type Domain struct {
	ID      uint   `gorm:"primaryKey"`
	Domain  string `gorm:"uniqueIndex"`
	GroupID uint
	Group   Group `gorm:"constraint:OnDelete:CASCADE"` // 外键
}

// HostEntry 用于自定义 hosts
type HostEntry struct {
	ID   uint   `gorm:"primaryKey"`
	Host string `gorm:"uniqueIndex"`
	IP   string
}

// —— Trie 用于后缀匹配 ——

type TrieNode struct {
	children map[string]*TrieNode
	upstream string
	isEnd    bool
}

type Trie struct {
	root *TrieNode
	mu   sync.RWMutex
}

func NewTrie() *Trie {
	return &Trie{root: &TrieNode{children: make(map[string]*TrieNode)}}
}

func (t *Trie) AddDomainRule(domain, upstream string) {
	if domain == "*" {
		return // 通配符记录无需加入 Trie，由调用方单独处理
	}
	domain = strings.ToLower(strings.TrimSpace(domain))
	parts := strings.Split(domain, ".")
	// 反转域名以便从后缀开始匹配
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	node := t.root
	for _, part := range parts {
		if _, ok := node.children[part]; !ok {
			node.children[part] = &TrieNode{children: make(map[string]*TrieNode)}
		}
		node = node.children[part]
	}
	node.isEnd = true
	node.upstream = upstream
}

func (t *Trie) MatchDomain(domain string) (string, bool) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	parts := strings.Split(domain, ".")
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	node := t.root
	var last *TrieNode
	for _, part := range parts {
		if next, ok := node.children[part]; ok {
			node = next
			if node.isEnd {
				last = node
			}
		} else {
			break
		}
	}
	if last != nil {
		return last.upstream, true
	}
	return "", false
}

// —— 全局状态 ——

var (
	mu         sync.RWMutex
	CFG        *Config
	DomainTrie *Trie
	Hosts      map[string]string
	ConfigFile string
)

// randPassword 生成随机密码/secret
func randPassword(n int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"0123456789" +
		"!@#"
	pwd := make([]byte, n)
	for i := range pwd {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		pwd[i] = charset[idx.Int64()]
	}
	return string(pwd), nil
}

// —— 环境变量辅助 ——

// getenvDefault string 环境变量，有则返回否则默认
func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// normalizePort 保证端口字符串形式，前缀有 ":"（如 "5300" -> ":5300"）
func normalizePort(p string) string {
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, ":") {
		return p
	}
	return ":" + p
}

// loadConfigFromEnv 从环境变量加载 API/Server 配置到 cfg（替代原来 ConfigMsg）
func loadConfigFromEnv(cfg *Config) {
	// Server port
	portEnv := os.Getenv("SERVER_PORT")
	if portEnv == "" {
		portEnv = ":5300"
	} else {
		portEnv = normalizePort(portEnv)
	}

	// API port
	apiPort := 9898
	if s := os.Getenv("API_PORT"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			apiPort = v
		}
	}

	apiUser := getenvDefault("API_USER", "adminlocal")

	apiPassword := os.Getenv("API_PASSWORD")
	if apiPassword == "" {
		pwd, err := randPassword(16)
		if err != nil {
			log.Printf("生成随机 API 密码失败: %v", err)
			apiPassword = "adminlocal" // fallback
		} else {
			apiPassword = pwd
			slog.Info("环境变量未提供 API 密码，自动生成", slog.String("password", apiPassword))
		}
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		secret, err := randPassword(32)
		if err != nil {
			log.Printf("生成随机 JWT Secret 失败: %v", err)
			jwtSecret = "defaultsecret" // fallback
		} else {
			jwtSecret = secret
			slog.Info("环境变量未提供 JWT Secret，自动生成", slog.String("jwt_secret", jwtSecret))
		}
	}

	cfg.Server.Port = portEnv
	cfg.API.Port = apiPort
	cfg.API.User = apiUser
	cfg.API.Password = apiPassword
	cfg.API.JWTSecret = jwtSecret
}

// —— 辅助函数 ——

func resolvePath(baseConfigPath, rel string) string {
	if filepath.IsAbs(rel) || rel == "" {
		return rel
	}
	return filepath.Join(filepath.Dir(baseConfigPath), rel)
}

// 一次性读取并解析地址列表
func readAddressesFromFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := bytes.Split(data, []byte{'\n'})
	out := make([]string, 0, len(lines))
	for _, raw := range lines {
		if idx := bytes.IndexByte(raw, '#'); idx >= 0 {
			raw = raw[:idx]
		}
		s := bytes.TrimSpace(raw)
		if len(s) > 0 {
			out = append(out, strings.ToLower(string(s)))
		}
	}
	return out, nil
}

// 解析 hosts 文件
func readHostsFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := bytes.Split(data, []byte{'\n'})
	res := make(map[string]string, len(lines))
	for _, raw := range lines {
		// 去掉注释
		if idx := bytes.IndexByte(raw, '#'); idx >= 0 {
			raw = raw[:idx]
		}
		fields := bytes.Fields(raw)
		if len(fields) < 2 {
			continue
		}

		// 自动检测哪一列是真正的 IP
		first := string(fields[0])
		second := string(fields[1])
		var ip string
		var hosts []string

		switch {
		case net.ParseIP(first) != nil:
			// 标准格式：IP 在第 0 列
			ip = first
			for _, f := range fields[1:] {
				hosts = append(hosts, string(f))
			}
		case net.ParseIP(second) != nil:
			// 反向格式：IP 在第 1 列
			ip = second
			for i, f := range fields {
				if i == 1 {
					continue
				}
				hosts = append(hosts, string(f))
			}
		default:
			// 无法识别的行跳过
			continue
		}

		// 存入 map
		for _, h := range hosts {
			host := strings.ToLower(strings.TrimSpace(h))
			if host == "" {
				continue
			}
			res[host] = strings.TrimSpace(ip)
		}
	}
	return res, nil
}

// LoadConfig 初始化：从 env 取核心 API/Server 设置，从数据库加载其余内容；首次启动用 YAML 或默认填充数据库
func LoadConfig(configPath string) error {
	mu.Lock()
	defer mu.Unlock()

	if shared.DB == nil {
		return fmt.Errorf("数据库未初始化，请先调用 shared.InitDBFromEnv")
	}

	ConfigFile = configPath

	db := shared.DB

	// 自动迁移（不再包括 ConfigMsg）
	if err := db.AutoMigrate(
		&Group{},
		&Domain{},
		&HostEntry{},
	); err != nil {
		log.Printf("AutoMigrate failed: %v", err)
		return err
	}

	// 初始化 CFG 并从环境变量加载 API / Server 配置
	if CFG == nil {
		CFG = &Config{}
	}
	loadConfigFromEnv(CFG)

	// 判断是否需要 seed：用 Domain 表是否已有内容作为判定依据
	var domainCount int64
	if err := db.Model(&Domain{}).Count(&domainCount).Error; err != nil {
		return err
	}

	if domainCount == 0 {
		if _, err := os.Stat(configPath); err == nil {
			// YAML 存在，读取并写入数据库
			data, err := os.ReadFile(configPath)
			if err != nil {
				log.Printf("Error reading config file: %v", err)
				return err
			}
			var c Config
			if err := yaml.Unmarshal(data, &c); err != nil {
				log.Printf("Error parsing config file: %v", err)
				return err
			}
			if err := seedDatabase(db, &c, configPath); err != nil {
				return err
			}
		} else if os.IsNotExist(err) {
			// YAML 文件不存在，使用默认（API/Server 来自 env）
			defaultConfig := Config{
				Server: ServerConfig{
					Port:          CFG.Server.Port,
					DefaultServer: "udp://8.8.8.8:53",
					V6:            false,
					BlockList:     "",
					HostsFile:     "",
				},
				API: APIConfig{
					Port:      CFG.API.Port,
					User:      CFG.API.User,
					Password:  CFG.API.Password,
					JWTSecret: CFG.API.JWTSecret,
				},
				Forward: []ForwardConfig{},
			}
			if err := seedDatabase(db, &defaultConfig, configPath); err != nil {
				return err
			}
			slog.Info("已使用环境变量/API 默认填充配置",
				slog.String("user", defaultConfig.API.User),
				slog.String("password", defaultConfig.API.Password),
				slog.String("jwt_secret", defaultConfig.API.JWTSecret),
			)
		} else {
			return err
		}
	}

	// 从数据库加载其余内容
	err1 := LoadFromDatabase()
	err2 := LoadHostsFromDB()
	return errors.Join(err1, err2)
}

// seedDatabase：把 YAML 或默认配置写入 Group/Domain/HostEntry
func seedDatabase(db *gorm.DB, c *Config, basePath string) error {
	const batchSize = 1000

	return db.Transaction(func(tx *gorm.DB) error {
		// 1. 构造 Group 列表（Default + Forward）
		groups := make([]Group, 0, len(c.Forward)+1)
		if c.Server.DefaultServer != "" {
			groups = append(groups, Group{
				Name:     "default",
				Upstream: c.Server.DefaultServer,
				V6:       c.Server.V6,
				Priority: 0,
			})
		}
		for idx, f := range c.Forward {
			groups = append(groups, Group{
				Name:     f.Name,
				Upstream: f.Server,
				V6:       f.V6,
				Priority: idx + 1,
			})
		}

		if err := tx.
			Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "name"}},
				DoNothing: true,
			}).
			CreateInBatches(groups, batchSize).Error; err != nil {
			return err
		}

		// 2. 构造 Domain 列表（并发读取文件）
		var dg errgroup.Group
		var muDomains sync.Mutex
		domains := make([]Domain, 0)

		for _, f := range c.Forward {
			if f.File == "" {
				continue
			}
			fcopy := f
			dg.Go(func() error {
				path := resolvePath(basePath, fcopy.File)
				doms, err := readAddressesFromFile(path)
				if err != nil {
					return err
				}
				var grp Group
				if err := tx.Where("name = ?", fcopy.Name).First(&grp).Error; err != nil {
					return err
				}
				muDomains.Lock()
				for _, d := range doms {
					domains = append(domains, Domain{
						Domain:  d,
						GroupID: grp.ID,
					})
				}
				muDomains.Unlock()
				return nil
			})
		}
		if err := dg.Wait(); err != nil {
			return err
		}
		if len(domains) > 0 {
			if err := tx.
				Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "domain"}},
					DoNothing: true,
				}).
				CreateInBatches(domains, batchSize).Error; err != nil {
				return err
			}
		}

		// 3. hosts 文件（如果配置了）
		if c.Server.HostsFile != "" {
			hostsMap, err := readHostsFile(resolvePath(basePath, c.Server.HostsFile))
			if err != nil {
				return err
			}
			hostEntries := make([]HostEntry, 0, len(hostsMap))
			for h, ip := range hostsMap {
				hostEntries = append(hostEntries, HostEntry{Host: h, IP: ip})
			}
			if len(hostEntries) > 0 {
				if err := tx.CreateInBatches(hostEntries, batchSize).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// LoadFromDatabase 从数据库加载所有配置到内存，并构建域名匹配 Trie（排除通配符和 default 组）
func LoadFromDatabase() error {
	if CFG == nil {
		CFG = &Config{}
	}

	if shared.DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	db := shared.DB

	// 1. 加载 Group，并构建映射
	var groups []Group
	if err := db.Order("priority ASC").Find(&groups).Error; err != nil {
		return fmt.Errorf("加载 Group 失败: %w", err)
	}
	fwdByName := make(map[string]ForwardConfig, len(groups))
	fwdByID := make(map[uint]ForwardConfig, len(groups))
	for _, g := range groups {
		fc := ForwardConfig{Name: g.Name, Server: g.Upstream, V6: g.V6}
		fwdByName[g.Name] = fc
		fwdByID[g.ID] = fc
	}

	// 2. 构建并填充 Trie（仅针对非通配符、非 default 组）
	DomainTrie = NewTrie()
	var domains []Domain
	if err := db.Order("domain = '*' DESC").Order("id ASC").Find(&domains).Error; err != nil {
		return fmt.Errorf("加载 Domain 失败: %w", err)
	}

	// 3. 设置默认组（来自 Group 名为 "default" 的那一条）
	if defFC, ok := fwdByName["default"]; ok {
		CFG.Server.DefaultServer = defFC.Server
		CFG.Server.V6 = defFC.V6
	}

	for _, d := range domains {
		if d.Domain == "*" {
			continue
		}
		fc, ok := fwdByID[d.GroupID]
		if !ok || fc.Name == "default" {
			continue
		}
		DomainTrie.AddDomainRule(d.Domain, fc.Server)
	}

	// 4. 重建 CFG.Forward 列表，保持 priority 顺序
	CFG.Forward = make([]ForwardConfig, 0, len(groups))
	for _, g := range groups {
		if fc, ok := fwdByName[g.Name]; ok {
			CFG.Forward = append(CFG.Forward, fc)
		}
	}

	return nil
}

func LoadHostsFromDB() error {
	if shared.DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	db := shared.DB

	var entries []HostEntry
	if err := db.Find(&entries).Error; err != nil {
		return err
	}

	tmp := make(map[string]string, len(entries))
	for _, e := range entries {
		rawHost := strings.TrimSpace(e.Host)
		rawIP := strings.TrimSpace(e.IP)
		var host, ip string

		// 如果 e.Host 看起来像 IP，则列可能颠倒
		if net.ParseIP(rawHost) != nil && net.ParseIP(rawIP) == nil {
			ip = rawHost
			host = rawIP
		} else {
			host = rawHost
			ip = rawIP
		}

		host = strings.ToLower(host)
		if host == "" || ip == "" {
			continue
		}
		tmp[host] = ip
	}

	Hosts = tmp
	return nil
}
