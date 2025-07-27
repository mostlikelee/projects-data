package main

import (
	"encoding/csv"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Item struct {
	Estimate int     `json:"estimate"`
	Status   string  `json:"status"`
	Sprint   *Sprint `json:"sprint"`
}

type Snapshot struct {
	Items []Item `json:"items"`
}

type Sprint struct {
	Title string `json:"title"`
}

func main() {
	// Read environment variables
	sprintName := os.Getenv("SPRINT_NAME")
	if sprintName == "" {
		log.Fatal("SPRINT_NAME environment variable is not set")
	}
	csvDir := os.Getenv("BURNDOWN_PATH")
	if csvDir == "" {
		log.Fatal("BURNDOWN_CSV_PATH environment variable is not set")
	}

	csvName := strings.TrimSpace(sprintName)
	csvName = strings.ReplaceAll(csvName, " ", "-")
	csvPath := filepath.Join(csvDir, csvName+".csv")

	// Ensure directory exists
	if err := os.MkdirAll(csvDir, 0755); err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}

	// Load and parse items.json
	jsonPath := ".tmp/items.json"
	file, err := os.Open(jsonPath)
	if err != nil {
		log.Fatalf("Failed to open JSON file: %v", err)
	}
	defer file.Close()

	var snap Snapshot
	if err := json.NewDecoder(file).Decode(&snap); err != nil {
		log.Fatalf("Failed to decode JSON: %v", err)
	}

	// Calculate totals only for current sprint
	var total, remaining int
	for _, item := range snap.Items {
		if item.Sprint == nil || item.Sprint.Title != sprintName {
			continue
		}
		total += item.Estimate
		if item.Status != "✔️Awaiting QA" && item.Status != "Done" && item.Status != "✅ Ready for release" {
			remaining += item.Estimate
		}
	}

	// Skip writing if no items matched the sprint
	if total == 0 {
		log.Printf("No items found for sprint %q — skipping CSV write.", sprintName)
		return
	}

	// Open or create CSV file
	csvFile, err := os.OpenFile(csvPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open CSV file: %v", err)
	}
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)

	// Write header if file is new
	fi, err := csvFile.Stat()
	if err == nil && fi.Size() == 0 {
		if err := writer.Write([]string{"timestamp", "total_points", "remaining_points"}); err != nil {
			log.Fatalf("Failed to write CSV header: %v", err)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	record := []string{now, strconv.Itoa(total), strconv.Itoa(remaining)}

	if err := writer.Write(record); err != nil {
		log.Fatalf("Failed to write record to CSV: %v", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Fatalf("Error flushing CSV: %v", err)
	}

	log.Printf("✅ Appended %s.csv: total=%d, remaining=%d", sprintName, total, remaining)
}
