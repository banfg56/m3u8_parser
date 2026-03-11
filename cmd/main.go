package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	m3u8 "github.com/banfg56/m3u8_parser"
)

var (
	url       string
	file      string
	content   string
	output    string
	list      bool
	bandwidth int    // 指定码率 (如 1200 表示 1200kbps)
	quality   string // 指定画质 (480p, 720p, 1080p)
)

func init() {
	flag.StringVar(&url, "u", "", "m3u8 URL")
	flag.StringVar(&file, "f", "", "m3u8 file path")
	flag.StringVar(&content, "c", "", "m3u8 content (direct input)")
	flag.StringVar(&output, "o", "", "output m3u8 file path (with resolved segment URLs)")
	flag.BoolVar(&list, "l", false, "list all segment URLs (one per line)")
	flag.IntVar(&bandwidth, "b", 0, "filter by bandwidth (e.g., 1200 = 1200kbps)")
	flag.StringVar(&quality, "q", "", "filter by quality: 480p, 720p, 1080p")
	flag.Usage = func() {
		fmt.Println("Usage: m3u8_parser [options]")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  m3u8_parser -u https://example.com/video.m3u8")
		fmt.Println("  m3u8_parser -u https://example.com/video.m3u8 -q 720p  # 只获取720p")
		fmt.Println("  m3u8_parser -u https://example.com/video.m3u8 -b 1200  # 只获取1200k")
		fmt.Println("  m3u8_parser -f /path/to/video.m3u8 -o output.m3u8")
		fmt.Println("  m3u8_parser -u https://example.com/video.m3u8 -l")
	}
}

func main() {
	flag.Parse()

	parser := m3u8.New()

	var result *m3u8.ParseResult
	var err error

	// 根据输入类型解析
	switch {
	case url != "":
		fmt.Printf("Fetching from URL: %s\n", url)
		result, err = parser.ParseFromURL(url)
	case file != "":
		fmt.Printf("Reading from file: %s\n", file)
		result, err = parser.ParseFromFile(file)
	case content != "":
		fmt.Println("Parsing from command line content")
		result, err = parser.ParseFromContent(content)
	case flag.NArg() > 0:
		// positional argument as URL or file
		input := flag.Arg(0)
		if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
			url = input
			fmt.Printf("Fetching from URL: %s\n", url)
			result, err = parser.ParseFromURL(url)
		} else {
			file = input
			fmt.Printf("Reading from file: %s\n", file)
			result, err = parser.ParseFromFile(file)
		}
	default:
		flag.Usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 输出解析结果
	fmt.Printf("\n=== Parse Result ===\n")
	fmt.Printf("Type: %s\n", result.Type)
	fmt.Printf("Version: %d\n", result.Version)
	fmt.Printf("TargetDuration: %d\n", result.TargetDuration)

	if result.Encryption != nil {
		fmt.Printf("Encryption: Method=%s, Key=%s\n",
			result.Encryption.Method, result.Encryption.Key)
	}

	// 获取所有分片（关联到每个码率）
	var segments []m3u8.Segment
	var variantsWithSegments []m3u8.Variant

	if url != "" && result.Type == m3u8.PlaylistTypeMaster {
		// 使用新的 GetVariantSegments 函数获取带分片的变体
		resultWithSegments, err := parser.GetVariantSegments(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting variant segments: %v\n", err)
		} else {
			result = resultWithSegments
		}

		// 按码率或画质过滤
		if bandwidth > 0 || quality != "" {
			variantsWithSegments = filterVariantsWithSegments(result.Variants, bandwidth, quality)
		} else {
			variantsWithSegments = result.Variants
		}

		// 收集所有分片
		for _, v := range variantsWithSegments {
			segments = append(segments, v.Segments...)
		}
	} else {
		segments = result.Segments
	}

	// 输出变体信息（带分片）
	if result.Type == m3u8.PlaylistTypeMaster {
		fmt.Printf("\n=== Variants with Segments (%d) ===\n", len(result.Variants))
		for i, v := range result.Variants {
			bandwidthKbps := v.Bandwidth / 1000
			fmt.Printf("[%d] Bandwidth: %dkbps, Resolution: %s\n", i, bandwidthKbps, v.Resolution)
			fmt.Printf("    Segments: %d\n", len(v.Segments))
			if len(v.Segments) > 0 {
				fmt.Printf("    First: %s\n", v.Segments[0].URI)
				fmt.Printf("    Last: %s\n", v.Segments[len(v.Segments)-1].URI)
			}
		}
	}

	// 如果没有过滤，输出汇总分片信息
	if bandwidth == 0 && quality == "" {
		fmt.Printf("\n=== All Segments (%d) ===\n", len(segments))
		// 只显示前5个和后5个
		for i, seg := range segments {
			if i < 5 || i >= len(segments)-5 {
				fmt.Printf("[%d] Duration: %.1fs, URL: %s\n", seg.Index, seg.Duration, seg.URI)
			} else if i == 5 {
				fmt.Printf("... (%d more segments)\n", len(segments)-10)
			}
		}
	} else {
		// 过滤模式，显示每个变体的分片
		fmt.Printf("\n=== Filtered Segments (%d) ===\n", len(segments))
		for _, v := range variantsWithSegments {
			bandwidthKbps := v.Bandwidth / 1000
			fmt.Printf("\n--- %dkbps (%s) --- %d segments ---\n", bandwidthKbps, v.Resolution, len(v.Segments))
			for _, seg := range v.Segments {
				fmt.Printf("  %s\n", seg.URI)
			}
		}
	}

	// 输出模式
	if list {
		// 按行输出所有分片 URL
		fmt.Println("\n=== Segment URLs ===")
		for _, seg := range segments {
			fmt.Println(seg.URI)
		}
	}

	if output != "" && len(segments) > 0 {
		// 生成新的 m3u8 文件，包含解析后的完整 URL
		fmt.Printf("\nGenerating m3u8 file: %s\n", output)
		if err := generateM3U8(output, result, segments, url); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating m3u8: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Done! Output saved to %s\n", output)
	}
}

// 生成带完整 URL 的 m3u8 文件
func generateM3U8(output string, result *m3u8.ParseResult, segments []m3u8.Segment, baseURL string) error {
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer f.Close()

	// 写入 m3u8 头
	fmt.Fprintf(f, "#EXTM3U\n")
	fmt.Fprintf(f, "#EXT-X-VERSION:%d\n", result.Version)
	fmt.Fprintf(f, "#EXT-X-TARGETDURATION:%d\n", result.TargetDuration)

	// 写入加密信息
	if result.Encryption != nil {
		fmt.Fprintf(f, "#EXT-X-KEY:METHOD=%s,URI=\"%s\"", result.Encryption.Method, result.Encryption.Key)
		if result.Encryption.IV != "" {
			fmt.Fprintf(f, ",IV=%s", result.Encryption.IV)
		}
		fmt.Fprintln(f)
	}

	// 写入分片
	for _, seg := range segments {
		fmt.Fprintf(f, "#EXTINF:%.1f,%s\n", seg.Duration, seg.Title)
		fmt.Fprintln(f, seg.URI)
	}

	// 结束标记
	fmt.Fprintln(f, "#EXT-X-ENDLIST")

	return nil
}

// filterVariants 根据带宽或画质过滤变体
func filterVariants(variants []m3u8.Variant, bandwidth int, quality string) []m3u8.Variant {
	var filtered []m3u8.Variant

	// 解析画质对应的分辨率
	resolutionMap := map[string][]string{
		"480p":  {"480", "640", "854"},
		"720p":  {"720", "1280"},
		"1080p": {"1080", "1920"},
	}

	for _, v := range variants {
		match := true

		// 按带宽过滤
		if bandwidth > 0 {
			// 找到最接近的带宽（允许 20% 误差）
			lower := bandwidth * 800  // 80% of specified
			upper := bandwidth * 1200 // 120% of specified
			if v.Bandwidth < lower || v.Bandwidth > upper {
				match = false
			}
		}

		// 按画质过滤
		if quality != "" && match {
			resolutions, ok := resolutionMap[strings.ToLower(quality)]
			if !ok {
				continue
			}
			found := false
			for _, res := range resolutions {
				if strings.Contains(v.Resolution, res) {
					found = true
					break
				}
			}
			if !found {
				match = false
			}
		}

		if match {
			filtered = append(filtered, v)
		}
	}

	// 如果没有匹配的，返回第一个（最低码率）
	if len(filtered) == 0 && len(variants) > 0 {
		filtered = []m3u8.Variant{variants[0]}
	}

	return filtered
}

// filterVariantsWithSegments 根据带宽或画质过滤带分片的变体
func filterVariantsWithSegments(variants []m3u8.Variant, bandwidth int, quality string) []m3u8.Variant {
	var filtered []m3u8.Variant

	// 解析画质对应的分辨率
	resolutionMap := map[string][]string{
		"480p":  {"480", "640", "854"},
		"720p":  {"720", "1280"},
		"1080p": {"1080", "1920"},
	}

	for _, v := range variants {
		match := true

		// 按带宽过滤
		if bandwidth > 0 {
			// 找到最接近的带宽（允许 20% 误差）
			lower := bandwidth * 800
			upper := bandwidth * 1200
			if v.Bandwidth < lower || v.Bandwidth > upper {
				match = false
			}
		}

		// 按画质过滤
		if quality != "" && match {
			resolutions, ok := resolutionMap[strings.ToLower(quality)]
			if !ok {
				continue
			}
			found := false
			for _, res := range resolutions {
				if strings.Contains(v.Resolution, res) {
					found = true
					break
				}
			}
			if !found {
				match = false
			}
		}

		if match {
			filtered = append(filtered, v)
		}
	}

	// 如果没有匹配的，返回第一个（最低码率）
	if len(filtered) == 0 && len(variants) > 0 {
		filtered = []m3u8.Variant{variants[0]}
	}

	return filtered
}
