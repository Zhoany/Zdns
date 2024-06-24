package rule

import (
	"bufio"
	"NEWzDNS/config"
	"NEWzDNS/matcher"
	"os"
	"strings"
	
)

// domainMatcher 用于域名匹配的全局变量
var domainMatcher *matcher.Matcher

// InitDomainMatcher 初始化域名匹配器
func InitDomainMatcher() {
	domainMatcher = matcher.NewMatcher(config.Cfg.CommonUpstream)
}

// LoadUpstreamRules 加载上游服务器规则并构建域名规则
func LoadUpstreamRules() {
	for _, upstream := range config.Cfg.UpstreamServers {
		loadRulesFromFile(upstream.DomainRulesFile, upstream, domainMatcher.AddDomainRule)
	}
}

func loadRulesFromFile(filename string, upstream config.Upstream, addRuleFunc func(string, config.Upstream)) {
	file, err := os.Open(filename)
	if err != nil {
		//fmt.Printf("Failed to open rules file: %s, error: %v\n", filename, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		domain := strings.TrimSpace(scanner.Text())
		if domain != "" {
			// 如果域名最后没有 `.`，则添加上
			if !strings.HasSuffix(domain, ".") {
				domain += "."
			}
			addRuleFunc(domain, upstream)
		//	fmt.Printf("Added rule for domain %s with upstream %s\n", domain, upstream.Address)
		}
	}
	if err := scanner.Err(); err != nil {
	//	fmt.Printf("Error reading rules file: %v\n", err)
	}
}

// LoadBlocklist 加载封锁列表
func LoadBlocklist() {
	file, err := os.Open(config.Cfg.BlocklistFile)
	if err != nil {
	//	fmt.Printf("Failed to open blocklist file: %s, error: %v\n", config.Cfg.BlocklistFile, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		domain := strings.TrimSpace(scanner.Text())
		if domain != "" {
			// 如果域名最后没有 `.`，则添加上
			if !strings.HasSuffix(domain, ".") {
				domain += "."
			}
			domainMatcher.AddBlocked(domain)
		//	fmt.Printf("Added blocked domain %s\n", domain)
		}
	}
	if err := scanner.Err(); err != nil {
		//fmt.Printf("Error reading blocklist file: %v\n", err)
	}
}

// MatchDomain 匹配域名并返回上游服务器配置
func MatchDomain(domain string) (config.Upstream, string, bool) {
	return domainMatcher.MatchRule(domain)
}

// IsBlocked 检查域名是否在封锁列表中
func IsBlocked(domain string) bool {
	return domainMatcher.IsBlocked(domain)
}