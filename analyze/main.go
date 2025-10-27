package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang/freetype/truetype"
	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

// KeywordStats 关键词统计结构
type KeywordStats struct {
	Keyword string
	Count   int
}

// KeywordAnalyzer 关键词分析器
type KeywordAnalyzer struct {
	db *sql.DB
}

// NewKeywordAnalyzer 创建新的分析器
func NewKeywordAnalyzer(dsn string) (*KeywordAnalyzer, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("数据库连接失败: %v", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("数据库ping失败: %v", err)
	}

	fmt.Println("✓ 数据库连接成功！")
	return &KeywordAnalyzer{db: db}, nil
}

// FetchKeywords 从数据库获取所有关键词
func (ka *KeywordAnalyzer) FetchKeywords() ([]string, error) {
	query := "SELECT keywords FROM detector_records WHERE keywords IS NOT NULL"
	rows, err := ka.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %v", err)
	}
	defer rows.Close()

	var allKeywords []string
	recordCount := 0

	for rows.Next() {
		var keywordsJSON string
		if err := rows.Scan(&keywordsJSON); err != nil {
			log.Printf("扫描行失败: %v", err)
			continue
		}

		recordCount++

		// 解析JSON数组
		var keywords []string
		if err := json.Unmarshal([]byte(keywordsJSON), &keywords); err != nil {
			log.Printf("JSON解析失败: %v, 数据: %s", err, keywordsJSON)
			continue
		}

		allKeywords = append(allKeywords, keywords...)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历结果集失败: %v", err)
	}

	fmt.Printf("共获取到 %d 条记录\n", recordCount)
	fmt.Printf("共提取到 %d 个关键词（包含重复）\n", len(allKeywords))

	return allKeywords, nil
}

// AnalyzeKeywords 分析关键词频率，返回前N个
func (ka *KeywordAnalyzer) AnalyzeKeywords(keywords []string, topN int) []KeywordStats {
	// 统计关键词频率
	countMap := make(map[string]int)
	for _, keyword := range keywords {
		countMap[keyword]++
	}

	// 转换为切片
	stats := make([]KeywordStats, 0, len(countMap))
	for keyword, count := range countMap {
		stats = append(stats, KeywordStats{
			Keyword: keyword,
			Count:   count,
		})
	}

	// 按频率降序排序
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Count > stats[j].Count
	})

	// 取前N个
	if len(stats) > topN {
		stats = stats[:topN]
	}

	// 打印统计结果
	fmt.Printf("\n关键词总数: %d\n", len(countMap))
	fmt.Printf("\n关键词频率统计 (Top %d):\n", len(stats))
	for i := 0; i < 60; i++ {
		fmt.Print("-")
	}
	fmt.Println()

	for i, stat := range stats {
		fmt.Printf("%2d. %-30s : %6d 次\n", i+1, stat.Keyword, stat.Count)
	}

	for i := 0; i < 60; i++ {
		fmt.Print("-")
	}
	fmt.Println()

	return stats
}

// GetChineseFont 获取中文字体（根据操作系统）
func GetChineseFont() (*truetype.Font, error) {
	// 尝试不同操作系统的中文字体路径
	fontPaths := []string{
		// Windows
		"C:/Windows/Fonts/simhei.ttf", // 黑体
		"C:/Windows/Fonts/msyh.ttc",   // 微软雅黑
		"C:/Windows/Fonts/simsun.ttc", // 宋体
		// Linux
		"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
		"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/arphic/uming.ttc",
		// macOS
		"/System/Library/Fonts/PingFang.ttc",
		"/Library/Fonts/Arial Unicode.ttf",
	}

	for _, path := range fontPaths {
		if fontData, err := os.ReadFile(path); err == nil {
			font, err := truetype.Parse(fontData)
			if err == nil {
				fmt.Printf("✓ 使用字体: %s\n", path)
				return font, nil
			}
		}
	}

	return nil, fmt.Errorf("未找到中文字体文件")
}

// PlotHistogram 生成直方图（使用Chart自定义绘制）
func (ka *KeywordAnalyzer) PlotHistogram(stats []KeywordStats, savePath string) error {
	// 加载中文字体
	font, err := GetChineseFont()
	if err != nil {
		log.Printf("警告: %v, 将使用默认字体（可能无法显示中文）", err)
		font = nil
	}

	// 准备X轴和Y轴数据
	xValues := make([]float64, len(stats))
	yValues := make([]float64, len(stats))
	labels := make([]string, len(stats))

	maxValue := 0.0
	for i, stat := range stats {
		xValues[i] = float64(i)
		yValues[i] = float64(stat.Count)
		labels[i] = stat.Keyword
		if yValues[i] > maxValue {
			maxValue = yValues[i]
		}
	}

	// 创建标题样式
	titleStyle := chart.Style{
		FontSize: 18,
	}
	if font != nil {
		titleStyle.Font = font
	}

	// 创建Y轴样式
	yAxisStyle := chart.Style{
		FontSize: 10,
	}
	if font != nil {
		yAxisStyle.Font = font
	}

	// 创建Y轴名称样式
	yAxisNameStyle := chart.Style{
		FontSize: 14,
	}
	if font != nil {
		yAxisNameStyle.Font = font
	}

	// 创建图表
	graph := chart.Chart{
		Title:      fmt.Sprintf("关键词频率分布直方图 (Top %d)", len(stats)),
		TitleStyle: titleStyle,
		Width:      2400,
		Height:     1000,
		Background: chart.Style{
			Padding: chart.Box{
				Top:    60,
				Left:   100,
				Right:  40,
				Bottom: 180,
			},
		},
		XAxis: chart.XAxis{
			Ticks: generateTicks(labels, font),
		},
		YAxis: chart.YAxis{
			Name:      "出现次数",
			NameStyle: yAxisNameStyle,
			Style:     yAxisStyle,
		},
		Series: []chart.Series{
			chart.ContinuousSeries{
				Style: chart.Style{
					StrokeWidth: 0,
					FillColor:   drawing.ColorTransparent,
				},
				XValues: xValues,
				YValues: yValues,
			},
		},
	}

	// 添加柱状图绘制
	graph.Elements = []chart.Renderable{
		func(r chart.Renderer, canvasBox chart.Box, defaults chart.Style) {
			// 计算柱子宽度
			barWidth := 30.0
			canvasWidth := float64(canvasBox.Width())
			canvasHeight := float64(canvasBox.Height())

			for i, stat := range stats {
				// 计算柱子位置
				xRatio := float64(i) / float64(len(stats)-1)
				if len(stats) == 1 {
					xRatio = 0.5
				}
				yRatio := float64(stat.Count) / maxValue

				centerX := canvasBox.Left + int(xRatio*canvasWidth)
				barLeft := centerX - int(barWidth/2)
				barRight := centerX + int(barWidth/2)
				barTop := canvasBox.Top + int((1-yRatio)*canvasHeight)
				barBottom := canvasBox.Bottom

				// 渐变色
				intensity := uint8(80 + (175 * i / len(stats)))
				barColor := drawing.Color{R: 50, G: 100, B: intensity, A: 255}

				// 绘制柱子
				r.SetFillColor(barColor)
				r.SetStrokeColor(drawing.ColorBlack)
				r.SetStrokeWidth(0.5)

				// 绘制矩形
				r.MoveTo(barLeft, barTop)
				r.LineTo(barRight, barTop)
				r.LineTo(barRight, barBottom)
				r.LineTo(barLeft, barBottom)
				r.LineTo(barLeft, barTop)
				r.FillStroke()

				// 在柱子上方显示数值
				if font != nil {
					r.SetFont(font)
				}
				r.SetFontSize(8)
				r.SetFillColor(drawing.ColorBlack)

				label := fmt.Sprintf("%d", stat.Count)
				textBox := r.MeasureText(label)
				textX := centerX - textBox.Width()/2
				textY := barTop - 5

				r.Text(label, textX, textY)
			}
		},
	}

	// 保存为PNG文件
	f, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer f.Close()

	if err := graph.Render(chart.PNG, f); err != nil {
		return fmt.Errorf("渲染图表失败: %v", err)
	}

	fmt.Printf("\n✓ 直方图已保存到: %s\n", savePath)
	return nil
}

// generateTicks 生成X轴刻度标签
func generateTicks(labels []string, font *truetype.Font) []chart.Tick {
	ticks := make([]chart.Tick, len(labels))

	// 创建刻度样式
	tickStyle := chart.Style{
		FontSize:            8,
		TextRotationDegrees: 60.0,
	}
	if font != nil {
		tickStyle.Font = font
	}

	for i, label := range labels {
		ticks[i] = chart.Tick{
			Value: float64(i),
			Label: label,
		}
	}

	return ticks
}

// Close 关闭数据库连接
func (ka *KeywordAnalyzer) Close() error {
	if ka.db != nil {
		fmt.Println("\n数据库连接已关闭")
		return ka.db.Close()
	}
	return nil
}

// Run 执行完整的分析流程
func (ka *KeywordAnalyzer) Run(topN int, savePath string) error {
	fmt.Println("============================================================")
	fmt.Println("                    关键词分析程序启动                    ")
	fmt.Println("============================================================")

	// 获取关键词
	keywords, err := ka.FetchKeywords()
	if err != nil {
		return err
	}

	if len(keywords) == 0 {
		fmt.Println("⚠ 没有找到任何关键词数据！")
		return nil
	}

	// 分析关键词
	stats := ka.AnalyzeKeywords(keywords, topN)

	// 生成直方图
	if err := ka.PlotHistogram(stats, savePath); err != nil {
		return err
	}

	fmt.Println("============================================================")
	fmt.Println("                        分析完成！                        ")
	fmt.Println("============================================================")

	return nil
}

func main() {
	// 数据库连接配置
	dsn := "root:@tcp(/complik?charset=utf8mb4&parseTime=True&timeout=10s"

	// 创建分析器
	analyzer, err := NewKeywordAnalyzer(dsn)
	if err != nil {
		log.Fatalf("❌ 创建分析器失败: %v", err)
	}
	defer analyzer.Close()

	// 执行分析，显示前50个最常见的关键词
	if err := analyzer.Run(50, "keywords_histogram.png"); err != nil {
		log.Fatalf("❌ 程序执行出错: %v", err)
	}
}
