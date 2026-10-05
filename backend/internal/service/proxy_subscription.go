package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"math"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	maxSubscriptionProxies = 300
	maxSubscriptionScan    = 5000
	maxProxyNameRunes      = 100
	maxProxyCredentialRune = 100
	maxProxyHostLen        = 255
)

// SubscriptionProxyNode is one HTTP/SOCKS proxy extracted from a subscription.
// Only protocols this gateway can dial are returned. vmess/ss/trojan and other
// tunnel protocols are counted as unsupported instead of being stored as dead proxies.
type SubscriptionProxyNode struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// SubscriptionParseResult is the preview of a parsed subscription body.
type SubscriptionParseResult struct {
	Proxies          []SubscriptionProxyNode `json:"proxies"`
	Format           string                  `json:"format"`
	Unsupported      int                     `json:"unsupported"`
	Invalid          int                     `json:"invalid"`
	Duplicate        int                     `json:"duplicate"`
	Truncated        bool                    `json:"truncated"`
	SkippedProtocols []string                `json:"skipped_protocols,omitempty"`
}

type protocolKind int

const (
	protocolUnknown protocolKind = iota
	protocolSupported
	protocolUnsupported
)

// ParseProxySubscription decodes a subscription body and extracts usable proxies.
// defaultProtocol applies to plain ip:port lines and JSON rows that omit a protocol.
func ParseProxySubscription(body []byte, defaultProtocol string) SubscriptionParseResult {
	text := normalizeSubscriptionText(body)
	if text == "" {
		return SubscriptionParseResult{Proxies: []SubscriptionProxyNode{}, Format: "empty"}
	}
	decoded, layers := unwrapSubscriptionEncoding(text)
	collector := newSubscriptionCollector(normalizeProxyProtocol(defaultProtocol))

	switch {
	case parseClashSubscription(decoded, collector):
		collector.result.Format = encodedFormat("clash", layers)
	case parseJSONSubscription(decoded, collector):
		collector.result.Format = encodedFormat("json", layers)
	default:
		parseSubscriptionLines(decoded, collector)
		if layers > 0 {
			collector.result.Format = "base64"
		} else {
			collector.result.Format = "plain"
		}
	}
	return collector.finish()
}

// NormalizeProxyName trims, strips control characters, and caps a proxy name.
// An empty result becomes "default", matching the historical batch-import name.
func NormalizeProxyName(name string) string {
	cleaned := cleanProxyName(name)
	if cleaned == "" {
		return "default"
	}
	return cleaned
}

// SubscriptionLooksLikeHTML reports whether a body is an HTML document.
func SubscriptionLooksLikeHTML(body []byte) bool {
	sample := body
	if len(sample) > 512 {
		sample = sample[:512]
	}
	lower := strings.ToLower(string(sample))
	return strings.Contains(lower, "<html") || strings.Contains(lower, "<!doctype html")
}

type subscriptionCollector struct {
	defaultProtocol string
	result          SubscriptionParseResult
	seen            map[string]struct{}
	skipped         map[string]struct{}
	scanned         int
}

func newSubscriptionCollector(defaultProtocol string) *subscriptionCollector {
	if defaultProtocol == "" {
		defaultProtocol = "http"
	}
	return &subscriptionCollector{
		defaultProtocol: defaultProtocol,
		seen:            make(map[string]struct{}),
		skipped:         make(map[string]struct{}),
		result: SubscriptionParseResult{
			Proxies: make([]SubscriptionProxyNode, 0),
		},
	}
}

func (c *subscriptionCollector) finish() SubscriptionParseResult {
	c.result.SkippedProtocols = sortedSet(c.skipped)
	if c.result.Proxies == nil {
		c.result.Proxies = []SubscriptionProxyNode{}
	}
	return c.result
}

func (c *subscriptionCollector) stop() bool {
	return c.result.Truncated
}

func (c *subscriptionCollector) markScanned() bool {
	c.scanned++
	if c.scanned > maxSubscriptionScan {
		c.result.Truncated = true
		return false
	}
	return true
}

func (c *subscriptionCollector) add(node SubscriptionProxyNode) bool {
	if len(c.result.Proxies) >= maxSubscriptionProxies {
		c.result.Truncated = true
		return false
	}
	node.Protocol = normalizeProxyProtocol(node.Protocol)
	if node.Protocol == "" {
		node.Protocol = c.defaultProtocol
	}
	node.Host = strings.Trim(strings.TrimSpace(node.Host), "[]")
	node.Username = strings.TrimSpace(node.Username)
	node.Password = strings.TrimSpace(node.Password)
	node.Name = cleanProxyName(node.Name)
	if node.Name == "" {
		node.Name = cleanProxyName(net.JoinHostPort(node.Host, strconv.Itoa(node.Port)))
	}
	if !validSubscriptionHost(node.Host) || node.Port < 1 || node.Port > 65535 || !supportedProxyProtocol(node.Protocol) {
		c.result.Invalid++
		return true
	}
	if len(node.Host) > maxProxyHostLen ||
		utf8.RuneCountInString(node.Username) > maxProxyCredentialRune ||
		utf8.RuneCountInString(node.Password) > maxProxyCredentialRune ||
		strings.ContainsAny(node.Username, "\r\n") ||
		strings.ContainsAny(node.Password, "\r\n") {
		c.result.Invalid++
		return true
	}
	key := node.Protocol + "|" + strings.ToLower(node.Host) + "|" + strconv.Itoa(node.Port) + "|" + node.Username + "|" + node.Password
	if _, ok := c.seen[key]; ok {
		c.result.Duplicate++
		return true
	}
	c.seen[key] = struct{}{}
	c.result.Proxies = append(c.result.Proxies, node)
	return true
}

func (c *subscriptionCollector) unsupported(protocol string) {
	c.result.Unsupported++
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol != "" {
		c.skipped[protocol] = struct{}{}
	}
}

func parseClashSubscription(text string, c *subscriptionCollector) bool {
	if !strings.Contains(text, "proxies:") {
		return false
	}
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil || doc.Proxies == nil {
		return false
	}
	for _, item := range doc.Proxies {
		if !c.markScanned() || c.stop() {
			break
		}
		addMappedProxy(item, c)
	}
	return true
}

func parseJSONSubscription(text string, c *subscriptionCollector) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return false
	}
	var raw any
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return false
	}
	switch value := raw.(type) {
	case []any:
		parseJSONList(value, c)
		return true
	case map[string]any:
		for _, key := range []string{"proxies", "outbounds", "data", "list", "servers", "result", "content"} {
			child, ok := value[key]
			if !ok {
				continue
			}
			switch nested := child.(type) {
			case []any:
				parseJSONList(nested, c)
				return true
			case string:
				parseSubscriptionLines(nested, c)
				return true
			case map[string]any:
				for _, innerKey := range []string{"list", "proxies", "servers", "data", "outbounds"} {
					if arr, ok := nested[innerKey].([]any); ok {
						parseJSONList(arr, c)
						return true
					}
				}
			}
		}
	}
	return false
}

func parseJSONList(items []any, c *subscriptionCollector) {
	for _, item := range items {
		if !c.markScanned() || c.stop() {
			return
		}
		switch value := item.(type) {
		case string:
			parseSubscriptionLine(value, "", c)
		case map[string]any:
			addMappedProxy(value, c)
		default:
			c.result.Invalid++
		}
	}
}

func addMappedProxy(item map[string]any, c *subscriptionCollector) {
	host := subscriptionFirst(subscriptionString(item["server"]), subscriptionString(item["host"]), subscriptionString(item["ip"]), subscriptionString(item["address"]))
	port, portOK := firstInt(item["port"], item["server_port"])
	rawProtocol := subscriptionFirst(subscriptionString(item["protocol"]), subscriptionString(item["type"]), subscriptionString(item["scheme"]))
	name := subscriptionFirst(subscriptionString(item["name"]), subscriptionString(item["remarks"]), subscriptionString(item["tag"]))
	username := subscriptionFirst(subscriptionString(item["username"]), subscriptionString(item["user"]))
	password := subscriptionFirst(subscriptionString(item["password"]), subscriptionString(item["pass"]))

	protocol, kind := classifyProtocol(rawProtocol)
	if kind == protocolUnknown && rawProtocol == "" && looksLikeShadowsocks(item) {
		c.unsupported("ss")
		return
	}
	if kind == protocolUnsupported {
		c.unsupported(rawProtocol)
		return
	}
	if kind == protocolUnknown && rawProtocol != "" {
		c.result.Invalid++
		return
	}
	if protocol == "" {
		protocol = c.defaultProtocol
	}
	if protocol == "http" && asBool(item["tls"]) {
		protocol = "https"
	}
	if !portOK {
		c.result.Invalid++
		return
	}
	c.add(SubscriptionProxyNode{
		Name:     name,
		Protocol: protocol,
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
	})
}

func parseSubscriptionLines(text string, c *subscriptionCollector) {
	for _, line := range strings.Split(text, "\n") {
		if !c.markScanned() || c.stop() {
			return
		}
		parseSubscriptionLine(line, "", c)
	}
}

func parseSubscriptionLine(line, fallbackName string, c *subscriptionCollector) {
	line = strings.TrimSpace(line)
	line = strings.Trim(line, `"'`)
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "//") {
		return
	}
	if strings.Contains(line, "://") {
		parseSubscriptionURL(line, c)
		return
	}
	if strings.Contains(line, ",") {
		parseCommaProxy(line, fallbackName, c)
		return
	}
	if at := strings.LastIndex(line, "@"); at > 0 {
		auth := line[:at]
		addr := line[at+1:]
		user, pass := splitUserPass(auth)
		host, port, remark, ok := splitHostPort(addr)
		if ok {
			name := fallbackName
			if remark != "" {
				name = remark
			}
			c.add(SubscriptionProxyNode{
				Name:     name,
				Protocol: c.defaultProtocol,
				Host:     host,
				Port:     port,
				Username: user,
				Password: pass,
			})
			return
		}
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return
	}
	host, port, rest, ok := splitHostPort(fields[0])
	if !ok {
		c.result.Invalid++
		return
	}
	username, password := "", ""
	if rest != "" {
		username, password = splitUserPass(rest)
	}
	name := fallbackName
	if len(fields) > 1 {
		name = strings.Join(fields[1:], " ")
	}
	c.add(SubscriptionProxyNode{
		Name:     name,
		Protocol: c.defaultProtocol,
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
	})
}

func parseSubscriptionURL(line string, c *subscriptionCollector) {
	parsed, err := url.Parse(line)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		c.result.Invalid++
		return
	}
	protocol, kind := classifyProtocol(parsed.Scheme)
	if kind == protocolUnsupported {
		c.unsupported(parsed.Scheme)
		return
	}
	if kind != protocolSupported {
		c.result.Invalid++
		return
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		c.result.Invalid++
		return
	}
	username := ""
	password := ""
	if parsed.User != nil {
		username = parsed.User.Username()
		password, _ = parsed.User.Password()
	}
	if username == "" {
		username = subscriptionFirst(parsed.Query().Get("username"), parsed.Query().Get("user"))
	}
	if password == "" {
		password = subscriptionFirst(parsed.Query().Get("password"), parsed.Query().Get("pass"))
	}
	c.add(SubscriptionProxyNode{
		Name:     parsed.Fragment,
		Protocol: protocol,
		Host:     parsed.Hostname(),
		Port:     port,
		Username: username,
		Password: password,
	})
}

func parseCommaProxy(line, fallbackName string, c *subscriptionCollector) {
	parts := strings.Split(line, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) < 2 {
		c.result.Invalid++
		return
	}
	protocol := c.defaultProtocol
	hostIndex := 0
	if candidate, kind := classifyProtocol(parts[0]); kind == protocolSupported {
		protocol = candidate
		hostIndex = 1
	} else if kind == protocolUnsupported {
		c.unsupported(parts[0])
		return
	}
	if len(parts) <= hostIndex+1 {
		c.result.Invalid++
		return
	}
	port, err := strconv.Atoi(parts[hostIndex+1])
	if err != nil {
		c.result.Invalid++
		return
	}
	username := ""
	password := ""
	if len(parts) > hostIndex+2 {
		username = parts[hostIndex+2]
	}
	if len(parts) > hostIndex+3 {
		password = strings.Join(parts[hostIndex+3:], ",")
	}
	c.add(SubscriptionProxyNode{
		Name:     fallbackName,
		Protocol: protocol,
		Host:     parts[hostIndex],
		Port:     port,
		Username: username,
		Password: password,
	})
}

func splitHostPort(value string) (host string, port int, rest string, ok bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", 0, "", false
	}
	if strings.HasPrefix(value, "[") {
		end := strings.Index(value, "]")
		if end <= 1 || end+1 >= len(value) || value[end+1] != ':' {
			return "", 0, "", false
		}
		host = value[1:end]
		tail := value[end+2:]
		portText, rest, _ := strings.Cut(tail, ":")
		port, err := strconv.Atoi(portText)
		if err != nil {
			return "", 0, "", false
		}
		return host, port, rest, true
	}
	host, tail, found := strings.Cut(value, ":")
	if !found || strings.TrimSpace(host) == "" {
		return "", 0, "", false
	}
	portText, rest, _ := strings.Cut(tail, ":")
	port, err := strconv.Atoi(strings.TrimSpace(portText))
	if err != nil {
		return "", 0, "", false
	}
	return strings.TrimSpace(host), port, rest, true
}

func splitUserPass(auth string) (string, string) {
	user, pass, _ := strings.Cut(auth, ":")
	return user, pass
}

func classifyProtocol(raw string) (string, protocolKind) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "http":
		return "http", protocolSupported
	case "https":
		return "https", protocolSupported
	case "socks", "socks5":
		return "socks5", protocolSupported
	case "socks5h":
		return "socks5h", protocolSupported
	case "ss", "ssr", "shadowsocks", "shadowsocksr", "vmess", "vless", "trojan", "trojan-go",
		"hysteria", "hysteria2", "hy2", "tuic", "wireguard", "wg", "anytls", "mieru", "juicity", "snell", "ssh":
		return strings.ToLower(strings.TrimSpace(raw)), protocolUnsupported
	default:
		return "", protocolUnknown
	}
}

func looksLikeShadowsocks(item map[string]any) bool {
	method := strings.ToLower(subscriptionString(item["method"]))
	if method == "" {
		return false
	}
	switch {
	case strings.Contains(method, "aes"),
		strings.Contains(method, "chacha"),
		strings.Contains(method, "blake"),
		strings.Contains(method, "rc4"),
		strings.HasPrefix(method, "2022-"),
		method == "none",
		method == "plain",
		method == "xchacha20-ietf-poly1305":
		return true
	default:
		return false
	}
}

func normalizeProxyProtocol(protocol string) string {
	canonical, kind := classifyProtocol(protocol)
	if kind == protocolSupported {
		return canonical
	}
	return ""
}

func supportedProxyProtocol(protocol string) bool {
	_, kind := classifyProtocol(protocol)
	return kind == protocolSupported
}

func validSubscriptionHost(host string) bool {
	if host == "" || len(host) > maxProxyHostLen || strings.ContainsAny(host, " /\t\r\n") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return true
	}
	if strings.Contains(host, "..") || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	return true
}

func cleanProxyName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) <= maxProxyNameRunes {
		return name
	}
	runes := []rune(name)
	return string(runes[:maxProxyNameRunes])
}

func normalizeSubscriptionText(body []byte) string {
	body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.TrimSpace(text)
}

func unwrapSubscriptionEncoding(text string) (string, int) {
	current := text
	layers := 0
	for layers < 2 && !looksLikeSubscription(current) {
		decoded, ok := decodeBase64Text(current)
		if !ok {
			break
		}
		current = decoded
		layers++
	}
	return current, layers
}

func looksLikeSubscription(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	switch trimmed[0] {
	case '{', '[':
		return true
	}
	if strings.Contains(trimmed, "://") || strings.Contains(trimmed, "proxies:") {
		return true
	}
	line := trimmed
	if index := strings.IndexByte(trimmed, '\n'); index >= 0 {
		line = trimmed[:index]
	}
	_, _, _, ok := splitHostPort(strings.TrimSpace(line))
	return ok
}

func decodeBase64Text(text string) (string, bool) {
	compact := strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', ' ':
			return -1
		default:
			return r
		}
	}, strings.TrimSpace(text))
	if len(compact) < 16 || len(compact)%4 == 1 {
		return "", false
	}
	for _, r := range compact {
		if !isBase64Rune(r) {
			return "", false
		}
	}
	if pad := len(compact) % 4; pad != 0 {
		compact += strings.Repeat("=", 4-pad)
	}
	decoded, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(compact)
		if err != nil {
			return "", false
		}
	}
	if !isMostlyText(decoded) {
		return "", false
	}
	out := strings.TrimSpace(string(decoded))
	if out == "" {
		return "", false
	}
	return out, true
}

func isBase64Rune(r rune) bool {
	switch {
	case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		return true
	case r == '+' || r == '/' || r == '-' || r == '_' || r == '=':
		return true
	default:
		return false
	}
}

func isMostlyText(body []byte) bool {
	if len(body) == 0 || bytes.Contains(body, []byte{0}) {
		return false
	}
	printable := 0
	for _, b := range body {
		if b == '\n' || b == '\r' || b == '\t' || (b >= 32 && b < 127) || b >= 128 {
			printable++
		}
	}
	return printable*5 >= len(body)*4
}

func encodedFormat(base string, layers int) string {
	if layers > 0 {
		return base + "-base64"
	}
	return base
}

func subscriptionFirst(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstInt(values ...any) (int, bool) {
	for _, value := range values {
		if value == nil {
			continue
		}
		if number, ok := subscriptionInt(value); ok {
			return number, true
		}
	}
	return 0, false
}

func subscriptionString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func subscriptionInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case uint:
		return int(typed), true
	case uint8:
		return int(typed), true
	case uint16:
		return int(typed), true
	case uint32:
		return int(typed), true
	case uint64:
		if typed > math.MaxInt32 {
			return 0, false
		}
		return int(typed), true
	case float32:
		return wholePort(float64(typed))
	case float64:
		return wholePort(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(parsed), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func wholePort(value float64) (int, bool) {
	if value < 1 || value > 65535 || value != math.Trunc(value) {
		return 0, false
	}
	return int(value), true
}

func asBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func sortedSet(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
