package m3u8_parser

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// ParseResult 解析结果
type ParseResult struct {
	Type           PlaylistType // master 或 media
	URI            string       // 原始地址
	Version        int          // EXT-X-VERSION
	TargetDuration int          // EXT-X-TARGETDURATION
	Segments       []Segment    // 媒体分片列表
	Variants       []Variant    // 多码率列表（仅master）
	Encryption     *Encryption  // 加密信息
}

// PlaylistType 播放列表类型
type PlaylistType string

const (
	PlaylistTypeMaster PlaylistType = "master" // 多码率列表
	PlaylistTypeMedia  PlaylistType = "media"  // 媒体播放列表
)

// Segment 分片信息
type Segment struct {
	Index    int     // 分片索引
	URI      string  // 分片完整URL（含query）
	Title    string  // EXTINF 标题
	Duration float64 // 时长（秒）
}

// Variant 码率变体（master playlist）
type Variant struct {
	Bandwidth  int       // 带宽 (bps)
	Resolution string    // 分辨率
	URI        string    // 媒体播放列表地址
	Codecs     string    // 编解码器信息
	Segments   []Segment // 该码率对应的分片列表
}

// Encryption 加密信息
type Encryption struct {
	Method string // AES-128, SAMPLE-AES 等
	Key    string // key URL
	IV     string // IV (可选)
}

// Parser m3u8 解析器
type Parser struct {
	httpClient *http.Client
	baseURL    string
}

// Option 配置选项
type Option func(*Parser)

// WithHTTPClient 自定义 HTTP 客户端
func WithHTTPClient(client *http.Client) Option {
	return func(p *Parser) {
		p.httpClient = client
	}
}

// New 创建解析器
func New(opts ...Option) *Parser {
	p := &Parser{
		httpClient: &http.Client{Timeout: 30 * 1000000000}, // 30秒
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// ParseFromURL 从URL解析
func (p *Parser) ParseFromURL(m3u8URL string) (*ParseResult, error) {
	resp, err := p.httpClient.Get(m3u8URL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch: %w", err)
	}
	defer resp.Body.Close()

	p.baseURL = getBaseURL(m3u8URL)
	return p.ParseFromReader(resp.Body)
}

// ParseFromFile 从文件解析
func (p *Parser) ParseFromFile(filepath string) (*ParseResult, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	p.baseURL = getBaseURL(filepath)
	return p.ParseFromReader(file)
}

// ParseFromContent 从内容解析
func (p *Parser) ParseFromContent(content string) (*ParseResult, error) {
	p.baseURL = ""
	return p.ParseFromReader(strings.NewReader(content))
}

// ParseFromReader 从 io.Reader 解析
func (p *Parser) ParseFromReader(reader io.Reader) (*ParseResult, error) {
	scanner := bufio.NewScanner(reader)
	result := &ParseResult{
		Segments: make([]Segment, 0),
		Variants: make([]Variant, 0),
	}

	var currentDuration float64
	var currentTitle string
	var isMaster bool
	segmentIndex := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 忽略空行
		if line == "" {
			continue
		}

		// 忽略非扩展行
		if !strings.HasPrefix(line, "#") {
			// 可能是分片URL或变体URL
			if isMaster {
				// master playlist 中的媒体列表地址
				result.Variants[len(result.Variants)-1].URI = resolveURL(p.baseURL, line)
			} else {
				// 媒体分片
				fullURL := resolveURL(p.baseURL, line)
				result.Segments = append(result.Segments, Segment{
					Index:    segmentIndex,
					URI:      fullURL,
					Duration: currentDuration,
					Title:    currentTitle,
				})
				segmentIndex++
			}
			currentDuration = 0
			currentTitle = ""
			continue
		}

		// 解析扩展标签
		switch {
		case strings.HasPrefix(line, "#EXTM3U"):
			// 先默认为 media，后续如果是 master 会被覆盖
			result.Type = PlaylistTypeMedia

		case strings.HasPrefix(line, "#EXT-X-STREAM-INF"):
			// Master playlist 中的变体信息
			result.Type = PlaylistTypeMaster
			isMaster = true
			variant := Variant{
				Bandwidth:  extractInt(line, "BANDWIDTH"),
				Resolution: extractString(line, "RESOLUTION"),
				Codecs:     extractString(line, "CODECS"),
			}
			result.Variants = append(result.Variants, variant)

		case strings.HasPrefix(line, "#EXT-X-VERSION"):
			result.Version = extractInt(line, "VERSION")

		case strings.HasPrefix(line, "#EXT-X-TARGETDURATION"):
			result.TargetDuration = extractInt(line, "TARGETDURATION")

		case strings.HasPrefix(line, "#EXTINF"):
			// 分片信息
			parts := strings.SplitN(line, ":", 2)
			if len(parts) > 1 {
				durationStr := strings.TrimSuffix(parts[1], ",")
				d, _ := strconv.ParseFloat(durationStr, 64)
				currentDuration = d
			}
			// 提取标题
			if idx := strings.Index(line, ","); idx != -1 && idx+1 < len(line) {
				currentTitle = strings.TrimSpace(line[idx+1:])
			}

		case strings.HasPrefix(line, "#EXT-X-KEY"):
			// 加密信息
			result.Encryption = &Encryption{
				Method: extractString(line, "METHOD"),
				Key:    resolveURL(p.baseURL, extractString(line, "URI")),
				IV:     extractString(line, "IV"),
			}

		case strings.HasPrefix(line, "#EXT-X-ENDLIST"):
			// 结束标记
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 如果没有分片也没有变体，说明是空的播放列表
	if len(result.Segments) == 0 && len(result.Variants) == 0 {
		return nil, fmt.Errorf("empty or invalid playlist")
	}

	return result, nil
}

// GetAllSegments 获取所有分片URL
// 如果是 master playlist，会自动获取所有变体的分片
func (p *Parser) GetAllSegments(m3u8URL string) ([]Segment, error) {
	result, err := p.ParseFromURL(m3u8URL)
	if err != nil {
		return nil, err
	}

	if result.Type == PlaylistTypeMaster {
		// 是 master playlist，需要递归获取每个变体的分片
		var allSegments []Segment
		for _, variant := range result.Variants {
			segments, err := p.GetAllSegments(variant.URI)
			if err != nil {
				continue // 忽略获取失败的变体
			}
			allSegments = append(allSegments, segments...)
		}
		return allSegments, nil
	}

	return result.Segments, nil
}

// GetAllSegmentURLs 获取所有分片URL字符串列表
func (p *Parser) GetAllSegmentURLs(m3u8URL string) ([]string, error) {
	segments, err := p.GetAllSegments(m3u8URL)
	if err != nil {
		return nil, err
	}

	urls := make([]string, len(segments))
	for i, seg := range segments {
		urls[i] = seg.URI
	}
	return urls, nil
}

// GetVariantSegments 获取所有变体及其分片（推荐用于 Master Playlist）
// 返回的 Variants 中每个元素都包含其对应的分片列表
func (p *Parser) GetVariantSegments(m3u8URL string) (*ParseResult, error) {
	result, err := p.ParseFromURL(m3u8URL)
	if err != nil {
		return nil, err
	}

	if result.Type == PlaylistTypeMaster {
		// 是 master playlist，递归获取每个变体的分片
		for i := range result.Variants {
			segments, err := p.GetAllSegments(result.Variants[i].URI)
			if err != nil {
				continue // 忽略获取失败的变体
			}
			result.Variants[i].Segments = segments
		}
	}

	return result, nil
}

// ============ 工具函数 ============

func getBaseURL(rawURL string) string {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		// 本地文件，取目录
		lastSlash := strings.LastIndex(rawURL, "/")
		if lastSlash > 0 {
			return "file://" + rawURL[:lastSlash+1]
		}
		return ""
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	path := u.Path
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash > 0 {
		u.Path = path[:lastSlash+1]
	} else {
		u.Path = "/"
	}
	u.RawQuery = ""
	return u.String()
}

func resolveURL(baseURL, relativeURL string) string {
	if baseURL == "" {
		return relativeURL
	}

	// 已经是完整URL
	if strings.HasPrefix(relativeURL, "http://") || strings.HasPrefix(relativeURL, "https://") {
		return relativeURL
	}

	// 绝对路径
	if strings.HasPrefix(relativeURL, "/") {
		u, _ := url.Parse(baseURL)
		u.Path = relativeURL
		return u.String()
	}

	// 相对路径
	return baseURL + relativeURL
}

func extractInt(line, key string) int {
	// 使用大小写不敏感的匹配，支持 = 或 : 分隔
	re := regexp.MustCompile(`(?i)` + key + `[=:]+(\d+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		v, _ := strconv.Atoi(matches[1])
		return v
	}
	return 0
}

func extractString(line, key string) string {
	re := regexp.MustCompile(key + `="([^"]+)"`)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	// 尝试无引号
	re = regexp.MustCompile(key + `=([^\s,]+)`)
	matches = re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
