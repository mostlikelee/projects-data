package main

import (
	"encoding/csv"
	"encoding/json"
	"log"
	"os"
	"strconv"
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
	// Path to JSON file
	jsonPath := ".tmp/items.json"

	// Get CSV path from environment
	csvPath := os.Getenv("BURNDOWN_CSV_PATH")
	if csvPath == "" {
		log.Fatal("BURNDOWN_CSV_PATH env variable is not set")
	}

	// Open JSON file
	file, err := os.Open(jsonPath)
	if err != nil {
		log.Fatalf("Failed to open JSON file: %v", err)
	}
	defer file.Close()

	var snap Snapshot
	if err := json.NewDecoder(file).Decode(&snap); err != nil {
		log.Fatalf("Failed to decode JSON: %v", err)
	}

	// Aggregate point totals
	var total, remaining int
	for _, item := range snap.Items {
		total += item.Estimate
		if item.Status != "Done" {
			remaining += item.Estimate
		}
	}

	// Create or append to CSV
	csvFile, err := os.OpenFile(csvPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open CSV file: %v", err)
	}
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)

	// Write header only if file is empty
	fi, err := csvFile.Stat()
	if err == nil && fi.Size() == 0 {
		_ = writer.Write([]string{"timestamp", "total_points", "remaining_points"})
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

	log.Printf("Appended snapshot: %s | total: %d | remaining: %d", now, total, remaining)
}
