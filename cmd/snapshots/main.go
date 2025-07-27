package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type CommentWrapper struct {
	Comments []Comment `json:"comments,omitempty"`
}

type Comment struct {
	Author Author `json:"author"`
	Body   string `json:"body"`
}

type Author struct {
	Login string `json:"login"`
}

type Snapshot struct {
	Timestamp time.Time `json:"timestamp"`
	Items     []Item    `json:"items"`
}

type Response struct {
	Items []Item `json:"items"`
}

type Item struct {
	Assignees   []string   `json:"assignees,omitempty"`
	Content     *Content   `json:"content,omitempty"`
	Estimate    *int       `json:"estimate,omitempty"`
	ID          string     `json:"id,omitempty"`
	Labels      []string   `json:"labels,omitempty"`
	Milestone   *Milestone `json:"milestone,omitempty"`
	Repository  string     `json:"repository,omitempty"`
	Sprint      *Sprint    `json:"sprint,omitempty"`
	Status      *string    `json:"status,omitempty"`
	Title       string     `json:"title,omitempty"`
	Comments    []Comment  `json:"comments,omitempty"`
	IssueNumber int        `json:"issueNumber,omitempty"` // Deprecated, use Content.Number instead

	ChangeType string `json:"changeType,omitempty"` // "added", "modified", "removed"
}

type Content struct {
	Body       string `json:"body,omitempty"`
	Number     int    `json:"number,omitempty"`
	Repository string `json:"repository,omitempty"`
	Title      string `json:"title,omitempty"`
	Type       string `json:"type,omitempty"`
	URL        string `json:"url,omitempty"`
}

type Milestone struct {
	Description string `json:"description,omitempty"`
	DueOn       string `json:"dueOn,omitempty"`
	Title       string `json:"title,omitempty"`
}

type Sprint struct {
	Duration    int    `json:"duration,omitempty"`
	IterationID string `json:"iterationId,omitempty"`
	StartDate   string `json:"startDate,omitempty"`
	Title       string `json:"title,omitempty"`
}

func parseItemsFile(path string, tmpDir string) ([]Item, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading items.json: %w", err)
	}

	var parsed Response
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parsing items.json: %w", err)
	}

	for i, entry := range parsed.Items {
		if entry.Content.Type == "Issue" {
			comments, err := parseCommentsFile(entry.Content.Number, tmpDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping comments for issue %d: %v\n", entry.Content.Number, err)
				continue
			}
			parsed.Items[i].Comments = comments
		}
	}

	return parsed.Items, nil
}

func parseCommentsFile(issueNumber int, tmpDir string) ([]Comment, error) {
	path := filepath.Join(tmpDir, fmt.Sprintf("comments-%d.json", issueNumber))
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var wrapper CommentWrapper
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("parsing comments JSON: %w", err)
	}

	return wrapper.Comments, nil
}

func loadSnapshot(path string) ([]Snapshot, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var snapshots []Snapshot
	if err := json.Unmarshal(data, &snapshots); err != nil {
		return nil, err
	}
	return snapshots, nil
}

func itemsDiff(oldItems, newItems []Item) []Item {
	diff := make([]Item, 0)
	oldItemsMap := make(map[string]Item)
	newItemsMap := make(map[string]Item)

	for _, item := range oldItems {
		oldItemsMap[item.ID] = item
	}
	for _, item := range newItems {
		newItemsMap[item.ID] = item
	}

	for _, newItem := range newItems {
		oldItem, exists := oldItemsMap[newItem.ID]
		if !exists {
			newItem.ChangeType = "added"
			newItem.IssueNumber = newItem.Content.Number
			diff = append(diff, newItem)
		} else {
			if diffItem := createDiffItem(oldItem, newItem); diffItem != nil {
				diffItem.ChangeType = "modified"
				diffItem.IssueNumber = newItem.Content.Number
				diff = append(diff, *diffItem)
			}
		}
	}

	for id, oldItem := range oldItemsMap {
		if _, stillExists := newItemsMap[id]; !stillExists {
			diff = append(diff, Item{
				ID:          oldItem.ID,
				Title:       oldItem.Title,
				IssueNumber: oldItem.Content.Number,
				ChangeType:  "removed",
			})
		}
	}

	return diff
}

func createDiffItem(oldItem, newItem Item) *Item {
	diffItem := &Item{
		ID:        newItem.ID,
		Content:   &Content{},
		Milestone: &Milestone{},
		Sprint:    &Sprint{},
		Comments:  make([]Comment, 0),
	}
	changed := false
	var contentChanged bool

	if !slices.Equal(oldItem.Assignees, newItem.Assignees) {
		diffItem.Assignees = newItem.Assignees
		changed = true
	}

	if oldItem.Content.Body != newItem.Content.Body {
		diffItem.Content.Body = newItem.Content.Body
		changed = true
		contentChanged = true
	}

	if oldItem.Content.Title != newItem.Content.Title {
		diffItem.Content.Title = newItem.Content.Title
		changed = true
		contentChanged = true
	}

	if !contentChanged {
		diffItem.Content = nil
	}

	// Handle estimate changes
	switch {
	case oldItem.Estimate == nil && newItem.Estimate != nil && *newItem.Estimate > 0:
		fmt.Println("Adding new estimate:", *newItem.Estimate)
		diffItem.Estimate = newItem.Estimate
		changed = true
	case oldItem.Estimate != nil && newItem.Estimate == nil:
		// Estimate was removed, set to 0 instead of nil
		fmt.Println("Removing estimate")
		diffItem.Estimate = new(int)
		*diffItem.Estimate = 0 // Set to zero to indicate removal
		changed = true
	case oldItem.Estimate != nil && newItem.Estimate != nil && *oldItem.Estimate != *newItem.Estimate:
		fmt.Println("Updating estimate from:", *oldItem.Estimate, "to:", newItem.Estimate)
		diffItem.Estimate = newItem.Estimate
		changed = true
	default:
		diffItem.Estimate = nil // Avoid empty estimate in diff
	}

	if !slices.Equal(oldItem.Labels, newItem.Labels) {
		diffItem.Labels = newItem.Labels
		changed = true
	}

	// Handle milestone changes
	switch {
	case oldItem.Milestone == nil && newItem.Milestone != nil:
		fmt.Println("Adding new milestone:", newItem.Milestone.Title)
		diffItem.Milestone = newItem.Milestone
		changed = true
	case oldItem.Milestone != nil && newItem.Milestone == nil:
		// Milestone was removed set to empty instead of nil
		changed = true
	case oldItem.Milestone != nil && newItem.Milestone != nil && oldItem.Milestone.Title != newItem.Milestone.Title:
		diffItem.Milestone = newItem.Milestone
		changed = true
	default:
		diffItem.Milestone = nil // Avoid empty milestone in diff
	}

	// Handle sprint changes
	switch {
	case oldItem.Sprint == nil && newItem.Sprint != nil:
		fmt.Println("Adding new sprint:", newItem.Sprint.Title)
		diffItem.Sprint = newItem.Sprint
		changed = true
	case oldItem.Sprint != nil && newItem.Sprint == nil:
		// Sprint was removed set to empty instead of nil
		changed = true
	case oldItem.Sprint != nil && newItem.Sprint != nil && oldItem.Sprint.Title != newItem.Sprint.Title:
		fmt.Println("Updating sprint from:", oldItem.Sprint.Title, "to:", newItem.Sprint.Title)
		diffItem.Sprint = newItem.Sprint
		changed = true
	default:
		diffItem.Sprint = nil // Nil sprint means no change
	}

	// Handle status changes
	switch {
	case oldItem.Status == nil && newItem.Status != nil:
		fmt.Println("Adding new status:", *newItem.Status)
		diffItem.Status = newItem.Status
		changed = true
	case oldItem.Status != nil && newItem.Status == nil:
		fmt.Println("Removing status")
		diffItem.Status = new(string)
		*diffItem.Status = "" // Empty string to indicate removal
		changed = true
	case oldItem.Status != nil && newItem.Status != nil && *oldItem.Status != *newItem.Status:
		fmt.Println("Updating status from:", *oldItem.Status, "to:", *newItem.Status)
		diffItem.Status = newItem.Status
		changed = true
	default:
		diffItem.Status = nil // Nil status means no change
	}

	if oldItem.Title != newItem.Title {
		diffItem.Title = newItem.Title
		changed = true
	}

	if !slices.Equal(oldItem.Comments, newItem.Comments) {
		diffItem.Comments = newItem.Comments
		changed = true
	} else {
		diffItem.Comments = nil // Avoid empty comments in diff
	}

	if !changed {
		return nil
	}

	fmt.Printf("Item %s changed: %+v\n", newItem.ID, diffItem)
	return diffItem
}

func mergeItem(base, diff Item) Item {
	if diff.Assignees != nil {
		base.Assignees = diff.Assignees
	}
	if diff.Content != nil && diff.Content.Body != "" {
		base.Content.Body = diff.Content.Body
	}
	if diff.Content != nil && diff.Content.Title != "" {
		base.Content.Title = diff.Content.Title
	}

	// Handle estimate changes
	if diff.Estimate != nil && *diff.Estimate == 0 {
		base.Estimate = nil // Set to nil if empty
	} else if diff.Estimate != nil && *diff.Estimate != 0 {
		if base.Estimate == nil {
			base.Estimate = new(int)
		}
		*base.Estimate = *diff.Estimate
	}

	if diff.Labels != nil {
		base.Labels = diff.Labels
	}

	// convert empty milestone to nil
	if diff.Milestone != nil && diff.Milestone.Title == "" {
		base.Milestone = nil
	}

	if diff.Milestone != nil && diff.Milestone.Title != "" {
		base.Milestone = diff.Milestone
	}

	// convert empty sprint to nil
	if diff.Sprint != nil && diff.Sprint.Title == "" {
		base.Sprint = nil
	}

	if diff.Sprint != nil && diff.Sprint.Title != "" {
		base.Sprint = diff.Sprint
	}

	// Handle status changes
	if diff.Status != nil && *diff.Status == "" {
		base.Status = nil // Set to nil if empty
	}
	if diff.Status != nil && *diff.Status != "" {
		if base.Status == nil {
			base.Status = new(string)
		}
		*base.Status = *diff.Status
	}

	if diff.Title != "" {
		base.Title = diff.Title
	}
	if diff.Comments != nil {
		base.Comments = diff.Comments
	}
	return base
}

func (i Item) IsZero() bool {
	return i.ID == ""
}

func reconstructState(snapshots []Snapshot) []Item {
	stateMap := make(map[string]Item)
	for _, item := range snapshots[0].Items {
		stateMap[item.ID] = item
	}
	for _, snapshot := range snapshots[1:] {
		for _, item := range snapshot.Items {
			if item.IsZero() {
				continue
			}
			switch item.ChangeType {
			case "removed":
				delete(stateMap, item.ID)
			case "added":
				if _, exists := stateMap[item.ID]; !exists {
					stateMap[item.ID] = item
				}
			default:
				stateMap[item.ID] = mergeItem(stateMap[item.ID], item)
			}
		}
	}
	state := make([]Item, 0, len(stateMap))
	for _, item := range stateMap {
		state = append(state, item)
	}
	return state
}

func main() {
	sprintName := os.Getenv("SPRINT_NAME")
	if sprintName == "" {
		fmt.Println("SPRINT_NAME is required")
		os.Exit(1)
	}

	snapshotPath := os.Getenv("SNAPSHOT_PATH")
	if snapshotPath == "" {
		snapshotPath = "./snapshots"
	}

	// normalize sprint name
	sprintName = strings.TrimSpace(sprintName)
	sprintName = strings.ReplaceAll(sprintName, " ", "-")

	snapshotPath = filepath.Join(snapshotPath, fmt.Sprintf("%s.json", sprintName))

	tmpDir := ".tmp"
	items, err := parseItemsFile(filepath.Join(tmpDir, "items.json"), tmpDir)
	if err != nil {
		fmt.Println("Error parsing items:", err)
		os.Exit(1)
	}

	snapshots, err := loadSnapshot(snapshotPath)
	if err != nil {
		fmt.Println("Error loading snapshots:", err)
		os.Exit(1)
	}

	if len(snapshots) == 0 {
		for i, item := range items {
			item.IssueNumber = item.Content.Number
			items[i] = item
		}

		snapshot := Snapshot{
			Timestamp: time.Now().UTC(),
			Items:     items,
		}
		snapshots = append(snapshots, snapshot)
		data, _ := json.MarshalIndent(snapshots, "", "  ")
		if err := os.MkdirAll(filepath.Dir(snapshotPath), 0755); err != nil {
			fmt.Println("Failed to create snapshot directory:", err)
			os.Exit(1)
		}
		if err := os.WriteFile(snapshotPath, data, 0644); err != nil {
			fmt.Println("Failed to write snapshot:", err)
			os.Exit(1)
		}
		fmt.Println("Created initial snapshot at", snapshotPath)
		return
	}

	previousState := reconstructState(snapshots)
	diffedItems := itemsDiff(previousState, items)
	if len(diffedItems) == 0 {
		fmt.Println("No changes since last snapshot, nothing to append.")
		return
	}

	snapshot := Snapshot{
		Timestamp: time.Now().UTC(),
		Items:     diffedItems,
	}
	snapshots = append(snapshots, snapshot)
	data, _ := json.MarshalIndent(snapshots, "", "  ")
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0755); err != nil {
		fmt.Println("Failed to create snapshot directory:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(snapshotPath, data, 0644); err != nil {
		fmt.Println("Failed to write snapshot:", err)
		os.Exit(1)
	}

	fmt.Println("Appended new snapshot to", snapshotPath)
}
