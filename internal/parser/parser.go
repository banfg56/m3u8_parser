package parser

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
	"time"
)

// M3U8Result 解析结果
type M3U8Result struct {
	MediaURL  string
	Duration  string
	Segments  []Segment
	Bandwidth int
	Resolution string
}

// Segment m3u8切片
type Segment struct {
	URL       string
	Duration  float64
	Title     string
}

// Parse 解析m3u8地址或文件
func Parse(input string) (*M3U8Result, error) {
	var reader io.Reader
	var baseURL string

	// 判断是URL还是文件
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		resp, err := http.Get(input)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch URL: %w", err)
		}
		defer resp.Body.Close()
		reader = resp.Body
		baseURL = getBaseURL(input)
	} else {
		file, err := os.Open(input)
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()
		reader = file
		baseURL = getBaseURL(input)
	}

	return parseM3U8(reader, baseURL)
}

func parseM3U8(reader io.Reader, baseURL string) (*M3U8Result, error) {
	scanner := bufio.NewScanner(reader)
	result := &M3U8Result{
		Segments: make([]Segment, 0),
	}

	var currentDuration float64
	var currentTitle string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "#EXTM3U") {
			continue
		}

		if strings.HasPrefix(line, "#EXT-X-STREAM-INF") {
			// 解析流信息
			result.Bandwidth = extractInt(line, "BANDWIDTH")
			result.Resolution = extractString(line, "RESOLUTION")
			continue
		}

		if strings.HasPrefix(line, "#EXTINF") {
			// 解析片段信息
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				durationStr := strings.TrimSuffix(parts[1], ",")
				d, _ := strconv.ParseFloat(durationStr, 64)
				currentDuration = d
			}
			// 提取标题
			if idx := strings.Index(line, ","); idx != -1 && idx+1 < len(line) {
				currentTitle = strings.TrimSpace(line[idx+1:])
			}
			continue
		}

		if strings.HasPrefix(line, "#EXT-X-ENDLIST") {
			result.Duration = "VOD"
			break
		}

		if strings.HasPrefix(line, "#EXT-X-TARGETDURATION") {
			continue
		}

		// 忽略其他注释行
		if strings.HasPrefix(line, "#") {
			continue
		}

		// 空行跳过
		if line == "" {
			continue
		}

		// 这是一个片段URL
		segmentURL := line
		if !strings.HasPrefix(segmentURL, "http://") && !strings.HasPrefix(segmentURL, "https://") {
			segmentURL = resolveURL(baseURL, segmentURL)
		}

		result.Segments = append(result.Segments, Segment{
			URL:       segmentURL,
			Duration:  currentDuration,
			Title:     currentTitle,
		})
		currentDuration = 0
		currentTitle = ""
	}

	// 计算总时长
	if len(result.Segments) > 0 {
		var totalDuration float64
		for _, seg := range result.Segments {
			totalDuration += seg.Duration
		}
		result.Duration = fmt.Sprintf("%.2fs (%s)", totalDuration, time.Duration(totalDuration*float64(time.Second)).String())
	}

	// 设置媒体URL
	if len(result.Segments) > 0 {
		result.MediaURL = result.Segments[0].URL
	}

	return result, scanner.Err()
}

func getBaseURL(rawURL string) string {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		// 本地文件，取目录
		lastSlash := strings.LastIndex(rawURL, "/")
		if lastSlash > 0 {
			return rawURL[:lastSlash+1]
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
	}
	u.RawQuery = ""
	return u.String()
}

func resolveURL(baseURL, relativeURL string) string {
	if baseURL == "" {
		return relativeURL
	}
	
	if strings.HasPrefix(relativeURL, "/") {
		u, _ := url.Parse(baseURL)
		u.Path = relativeURL
		return u.String()
	}
	
	return baseURL + relativeURL
}

func extractInt(line, key string) int {
	re := regexp.MustCompile(key + `=(\d+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		v, _ := strconv.Atoi(matches[1])
		return v
	}
	return 0
}

func extractString(line, key string) string {
	re := regexp.MustCompile(key + `=([^,\s]+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}