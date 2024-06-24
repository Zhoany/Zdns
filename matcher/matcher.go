package matcher

import (
	"NEWzDNS/config"
	
	"strings"
)

// Matcher represents a domain matcher
type Matcher struct {
	domainMap      map[string]config.Upstream
	blockedDomains map[string]bool
	commonUpstream config.Upstream
}

// NewMatcher creates a new Matcher
func NewMatcher(commonUpstream config.Upstream) *Matcher {
	return &Matcher{
		domainMap:      make(map[string]config.Upstream),
		blockedDomains: make(map[string]bool),
		commonUpstream: commonUpstream,
	}
}

// AddDomainRule adds a domain rule to the Matcher
func (m *Matcher) AddDomainRule(domain string, upstream config.Upstream) {
	m.domainMap[strings.ToLower(domain)] = upstream
}

// AddBlocked adds a blocked domain to the Matcher
func (m *Matcher) AddBlocked(domain string) {
	m.blockedDomains[strings.ToLower(domain)] = true
}

// MatchRule matches a domain against the rules and returns the upstream configuration and the matched rule
func (m *Matcher) MatchRule(domain string) (config.Upstream, string, bool) {
	domain = strings.ToLower(domain)

	// Check blocked domains
	if m.blockedDomains[domain] {
	//	fmt.Printf("Domain %s is blocked\n", domain)
		return config.Upstream{}, "", true
	}

	// Exact domain match
	if upstream, found := m.domainMap[domain]; found {
	//	fmt.Printf("Domain %s matched exact rule\n", domain)
		return upstream, domain, true
	}

	// Top-level domain match
	parts := strings.Split(domain, ".")
	for i := 0; i < len(parts)-1; i++ {
		tld := strings.Join(parts[i:], ".")
		if upstream, found := m.domainMap[tld]; found {
		//	fmt.Printf("Domain %s matched top-level domain rule %s\n", domain, tld)
			return upstream, tld, true
		}
	}

	// Default to common upstream
	//fmt.Printf("Domain %s not matched, using common upstream\n", domain)
	return m.commonUpstream, "", false
}

// IsBlocked checks if a domain is in the blocked list
func (m *Matcher) IsBlocked(domain string) bool {
	return m.blockedDomains[strings.ToLower(domain)]
}