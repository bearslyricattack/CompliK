// Copyright 2025 CompliK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package main implements a keyword frequency analyzer for compliance detection records.
//
// This tool connects to a MySQL database containing compliance detection records,
// extracts keywords from JSON arrays, performs frequency analysis, and generates
// visual histogram charts showing the most common compliance issues detected.
//
// Features:
//   - Connects to MySQL database with configurable DSN
//   - Extracts and analyzes keywords from detector_records table
//   - Generates top-N keyword frequency statistics
//   - Creates histogram visualizations with Chinese font support
//   - Cross-platform font detection (Windows, Linux, macOS)
//
// Usage:
//
//	go run main.go
//
// The program will:
//  1. Connect to the database specified in the DSN
//  2. Fetch all keywords from detector_records
//  3. Analyze frequency and display top 50 keywords
//  4. Generate a histogram chart saved as keywords_histogram.png
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

// KeywordStats represents statistical data for a single keyword
type KeywordStats struct {
	Keyword string // The keyword text
	Count   int    // Number of occurrences
}

// KeywordAnalyzer analyzes keyword frequency from database records
type KeywordAnalyzer struct {
	db *sql.DB
}

// NewKeywordAnalyzer creates a new keyword analyzer with database connection
// dsn: MySQL data source name in format: user:password@tcp(host:port)/database?params
func NewKeywordAnalyzer(dsn string) (*KeywordAnalyzer, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Configure connection pool parameters
	db.SetMaxOpenConns(10)           // Maximum open connections
	db.SetMaxIdleConns(5)            // Maximum idle connections
	db.SetConnMaxLifetime(time.Hour) // Connection lifetime

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	fmt.Println("✓ Database connection established successfully!")
	return &KeywordAnalyzer{db: db}, nil
}

// FetchKeywords retrieves all keywords from the detector_records table
// Returns a flat list of keywords (with duplicates) extracted from JSON arrays
func (ka *KeywordAnalyzer) FetchKeywords() ([]string, error) {
	query := "SELECT keywords FROM detector_records WHERE keywords IS NOT NULL"
	rows, err := ka.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %v", err)
	}
	defer rows.Close()

	var allKeywords []string
	recordCount := 0

	for rows.Next() {
		var keywordsJSON string
		if err := rows.Scan(&keywordsJSON); err != nil {
			log.Printf("failed to scan row: %v", err)
			continue
		}

		recordCount++

		// Parse JSON array containing keywords
		var keywords []string
		if err := json.Unmarshal([]byte(keywordsJSON), &keywords); err != nil {
			log.Printf("failed to parse JSON: %v, data: %s", err, keywordsJSON)
			continue
		}

		allKeywords = append(allKeywords, keywords...)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	fmt.Printf("Total records fetched: %d\n", recordCount)
	fmt.Printf("Total keywords extracted: %d (including duplicates)\n", len(allKeywords))

	return allKeywords, nil
}

// AnalyzeKeywords analyzes keyword frequency and returns top N results
// keywords: list of keywords (may contain duplicates)
// topN: maximum number of results to return (sorted by frequency descending)
func (ka *KeywordAnalyzer) AnalyzeKeywords(keywords []string, topN int) []KeywordStats {
	// Count keyword frequency
	countMap := make(map[string]int)
	for _, keyword := range keywords {
		countMap[keyword]++
	}

	// Convert map to slice for sorting
	stats := make([]KeywordStats, 0, len(countMap))
	for keyword, count := range countMap {
		stats = append(stats, KeywordStats{
			Keyword: keyword,
			Count:   count,
		})
	}

	// Sort by frequency in descending order
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Count > stats[j].Count
	})

	// Limit to top N results
	if len(stats) > topN {
		stats = stats[:topN]
	}

	// Print statistics summary
	fmt.Printf("\nTotal unique keywords: %d\n", len(countMap))
	fmt.Printf("\nKeyword Frequency Statistics (Top %d):\n", len(stats))
	fmt.Println("------------------------------------------------------------")

	for i, stat := range stats {
		fmt.Printf("%2d. %-30s : %6d occurrences\n", i+1, stat.Keyword, stat.Count)
	}

	fmt.Println("------------------------------------------------------------")

	return stats
}

// GetChineseFont attempts to load a Chinese-capable font from common system locations
// Tries multiple font paths across Windows, Linux, and macOS systems
// Returns the first successfully loaded font, or an error if none found
func GetChineseFont() (*truetype.Font, error) {
	fontPaths := []string{
		// Windows fonts
		"C:/Windows/Fonts/simhei.ttf", // SimHei (SimHei)
		"C:/Windows/Fonts/msyh.ttc",   // Microsoft YaHei (Microsoft YaHei)
		"C:/Windows/Fonts/simsun.ttc", // SimSun (SimSun)
		// Linux fonts
		"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
		"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/arphic/uming.ttc",
		// macOS fonts
		"/System/Library/Fonts/PingFang.ttc",
		"/Library/Fonts/Arial Unicode.ttf",
	}

	for _, path := range fontPaths {
		if fontData, err := os.ReadFile(path); err == nil {
			font, err := truetype.Parse(fontData)
			if err == nil {
				fmt.Printf("✓ Using font: %s\n", path)
				return font, nil
			}
		}
	}

	return nil, fmt.Errorf("no Chinese font file found in system paths")
}

// PlotHistogram generates a histogram chart and saves it as a PNG image
// stats: keyword statistics to visualize
// savePath: output file path for the PNG image
func (ka *KeywordAnalyzer) PlotHistogram(stats []KeywordStats, savePath string) error {
	// Load Chinese font for proper text rendering
	font, err := GetChineseFont()
	if err != nil {
		log.Printf("Warning: %v, will use default font (Chinese characters may not display correctly)", err)
		font = nil
	}

	// Prepare X-axis and Y-axis data
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

	// Configure title style
	titleStyle := chart.Style{
		FontSize: 18,
	}
	if font != nil {
		titleStyle.Font = font
	}

	// Configure Y-axis label style
	yAxisStyle := chart.Style{
		FontSize: 10,
	}
	if font != nil {
		yAxisStyle.Font = font
	}

	// Configure Y-axis name style
	yAxisNameStyle := chart.Style{
		FontSize: 14,
	}
	if font != nil {
		yAxisNameStyle.Font = font
	}

	// Create the chart configuration
	graph := chart.Chart{
		Title:      fmt.Sprintf("Keyword Frequency Distribution Histogram (Top %d)", len(stats)),
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
			Name:      "Occurrences",
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

	// Add custom bar chart rendering
	graph.Elements = []chart.Renderable{
		func(r chart.Renderer, canvasBox chart.Box, defaults chart.Style) {
			// Define bar width
			barWidth := 30.0
			canvasWidth := float64(canvasBox.Width())
			canvasHeight := float64(canvasBox.Height())

			for i, stat := range stats {
				// Calculate bar position
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

				// Apply gradient color based on position
				intensity := uint8(80 + (175 * i / len(stats)))
				barColor := drawing.Color{R: 50, G: 100, B: intensity, A: 255}

				// Configure bar rendering
				r.SetFillColor(barColor)
				r.SetStrokeColor(drawing.ColorBlack)
				r.SetStrokeWidth(0.5)

				// Draw the bar rectangle
				r.MoveTo(barLeft, barTop)
				r.LineTo(barRight, barTop)
				r.LineTo(barRight, barBottom)
				r.LineTo(barLeft, barBottom)
				r.LineTo(barLeft, barTop)
				r.FillStroke()

				// Display count value above the bar
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

	// Save the chart as PNG file
	f, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer f.Close()

	if err := graph.Render(chart.PNG, f); err != nil {
		return fmt.Errorf("failed to render chart: %v", err)
	}

	fmt.Printf("\n✓ Histogram saved to: %s\n", savePath)
	return nil
}

// generateTicks creates X-axis tick labels with rotation for better readability
func generateTicks(labels []string, font *truetype.Font) []chart.Tick {
	ticks := make([]chart.Tick, len(labels))

	// Configure tick style with 60-degree rotation
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

// Close closes the database connection and releases resources
func (ka *KeywordAnalyzer) Close() error {
	if ka.db != nil {
		fmt.Println("\nDatabase connection closed")
		return ka.db.Close()
	}
	return nil
}

// Run executes the complete keyword analysis workflow
// topN: number of top keywords to analyze and display
// savePath: file path where the histogram will be saved
func (ka *KeywordAnalyzer) Run(topN int, savePath string) error {
	fmt.Println("============================================================")
	fmt.Println("           Keyword Analysis Program Started               ")
	fmt.Println("============================================================")

	// Fetch keywords from database
	keywords, err := ka.FetchKeywords()
	if err != nil {
		return err
	}

	if len(keywords) == 0 {
		fmt.Println("⚠ No keyword data found!")
		return nil
	}

	// Analyze keyword frequency
	stats := ka.AnalyzeKeywords(keywords, topN)

	// Generate histogram visualization
	if err := ka.PlotHistogram(stats, savePath); err != nil {
		return err
	}

	fmt.Println("============================================================")
	fmt.Println("              Analysis Completed Successfully!             ")
	fmt.Println("============================================================")

	return nil
}

func main() {
	// Database connection configuration
	// Format: user:password@tcp(host:port)/database?params
	dsn := "root:@tcp(127.0.0.1:3306)/complik?charset=utf8mb4&parseTime=True&timeout=10s"

	// Create analyzer instance
	analyzer, err := NewKeywordAnalyzer(dsn)
	if err != nil {
		log.Fatalf("❌ Failed to create analyzer: %v", err)
	}
	defer analyzer.Close()

	// Run analysis: display top 50 most common keywords and generate histogram
	if err := analyzer.Run(50, "keywords_histogram.png"); err != nil {
		log.Fatalf("❌ Program execution failed: %v", err)
	}
}
