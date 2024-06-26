package rule

import (
	"bufio"
	"NEWzDNS/config"
	"NEWzDNS/matcher"
	"os"
	"strings"
)

// domainMatcher is a global variable for domain matching
var domainMatcher *matcher.Matcher

// InitDomainMatcher initializes the domain matcher
func InitDomainMatcher() {
	domainMatcher = matcher.NewMatcher(config.Cfg.CommonUpstream)
}

// LoadUpstreamRules loads upstream server rules and constructs domain rules
func LoadUpstreamRules() {
	for _, upstream := range config.Cfg.UpstreamServers {
		loadRulesFromFile(upstream.DomainRulesFile, upstream, domainMatcher.AddDomainRule)
	}
}

func loadRulesFromFile(filename string, upstream config.Upstream, addRuleFunc func(string, config.Upstream)) {
	file, err := os.Open(filename)
	if err != nil {
		
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		domain := strings.TrimSpace(scanner.Text())
		if domain != "" {
			// If the domain does not end with `.`, add it
			if !strings.HasSuffix(domain, ".") {
				domain += "."
			}
			addRuleFunc(domain, upstream)
			
		}
	}
	if err := scanner.Err(); err != nil {
		
	}
}

// LoadBlocklist loads the blocklist
func LoadBlocklist() {
	file, err := os.Open(config.Cfg.BlocklistFile)
	if (err != nil) {
		
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		domain := strings.TrimSpace(scanner.Text())
		if domain != "" {
			// If the domain does not end with `.`, add it
			if (!strings.HasSuffix(domain, ".")) {
				domain += "."
			}
			domainMatcher.AddBlocked(domain)
			
		}
	}
	if err := scanner.Err(); err != nil {
		
	}
}

// MatchDomain matches a domain and returns the upstream configuration and domain rule
func MatchDomain(domain string) (config.Upstream, string, bool) {
	return domainMatcher.MatchRule(domain)
}

// IsBlocked checks if a domain is in the blocklist
func IsBlocked(domain string) bool {
	return domainMatcher.IsBlocked(domain)
}
