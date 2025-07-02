package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

// HNStory represents a simplified Hacker News story structure
type HNStory struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	By    string `json:"by"`
	Score int    `json:"score"`
	Type  string `json:"type"`
}

type Stats struct {
	CPUPercent float64 `json:"cpu_percent"`
	CPUCores   int     `json:"cpu_cores"`
	Load1      float64 `json:"load1"`
	Load5      float64 `json:"load5"`
	Load15     float64 `json:"load15"`

	MemoryUsed    uint64  `json:"mem_used"`
	MemoryTotal   uint64  `json:"mem_total"`
	MemoryPercent float64 `json:"mem_percent"`

	SwapUsed    uint64  `json:"swap_used"`
	SwapTotal   uint64  `json:"swap_total"`
	SwapPercent float64 `json:"swap_percent"`

	TempC float64 `json:"temp"`

	DiskUsed    uint64  `json:"disk_used"`
	DiskTotal   uint64  `json:"disk_total"`
	DiskPercent float64 `json:"disk_percent"`
}

// Global state to track the IDs of stories last sent to clients for *delta* updates.
// This is used to ensure we only push *newly appearing* stories on subsequent ticks.
var lastSentStoryIDs struct {
	sync.Mutex
	ids map[int]struct{}
}

func init() {
	lastSentStoryIDs.ids = make(map[int]struct{})
}

func main() {
	// Serve UI files directly from the root path
	http.Handle("/", http.FileServer(http.Dir("ui")))

	// SSE Endpoint for Hacker News updates
	http.HandleFunc("/hn-events", hnEventsHandler)

	// SSE Endpoint for stats
	http.HandleFunc("/stats", statsHandler)

	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for {
		cpuPercent, _ := cpu.Percent(0, false)
		cpuCores, _ := cpu.Counts(true)
		loadAvg, _ := load.Avg()
		vmem, _ := mem.VirtualMemory()
		swap, _ := mem.SwapMemory()
		sensors, _ := host.SensorsTemperatures()
		disk, _ := disk.Usage("/")

		temp := 0.0
		for _, s := range sensors {
			if s.SensorKey == "Package id 0" || s.SensorKey == "Tdie" || s.SensorKey == "coretemp" {
				temp = s.Temperature
				break
			}
		}

		stats := Stats{
			CPUPercent: cpuPercent[0],
			CPUCores:   cpuCores,
			Load1:      loadAvg.Load1,
			Load5:      loadAvg.Load5,
			Load15:     loadAvg.Load15,

			MemoryUsed:    vmem.Used / (1024 * 1024),
			MemoryTotal:   vmem.Total / (1024 * 1024),
			MemoryPercent: vmem.UsedPercent,

			SwapUsed:    swap.Used / (1024 * 1024),
			SwapTotal:   swap.Total / (1024 * 1024),
			SwapPercent: swap.UsedPercent,

			TempC: temp,

			DiskUsed:    disk.Used / (1024 * 1024),
			DiskTotal:   disk.Total / (1024 * 1024),
			DiskPercent: disk.UsedPercent,
		}

		jsonData, _ := json.Marshal(stats)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		time.Sleep(2 * time.Second)
	}
}

// hnEventsHandler manages the Server-Sent Events connection for Hacker News.
func hnEventsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		log.Println("Streaming unsupported by client connection.")
		return
	}

	log.Println("New Hacker News SSE client connected!")

	// Send an initial 'connected' event
	currentTime := time.Now().Format("15:04:05")
	fmt.Fprintf(w, "event: connected\ndata: Connected to HN stream at %s\n\n", currentTime)
	flusher.Flush()

	// --- NEW: Send an initial batch of top stories to the newly connected client ---
	sendInitialHackerNewsStories(w, flusher)

	ctx := r.Context()
	ticker := time.NewTicker(120 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// For subsequent ticks, we only send genuinely new stories
			sendDeltaHackerNewsStories(w, flusher) // Renamed for clarity
		case <-ctx.Done():
			log.Println("Hacker News SSE client disconnected.")
			return
		}
	}
}

// sendInitialHackerNewsStories fetches the current top stories and sends them
// to a newly connected client. It also populates the global lastSentStoryIDs.
func sendInitialHackerNewsStories(w http.ResponseWriter, flusher http.Flusher) {
	log.Println("Sending initial batch of Hacker News stories to new client...")
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get("https://hacker-news.firebaseio.com/v0/topstories.json?print=pretty")
	if err != nil {
		log.Printf("Error fetching initial top story IDs: %v", err)
		fmt.Fprintf(w, "event: error\ndata: Error fetching initial top story IDs: %v\n\n", err)
		flusher.Flush()
		return
	}
	defer resp.Body.Close()

	var currentStoryIDs []int
	if err := json.NewDecoder(resp.Body).Decode(&currentStoryIDs); err != nil {
		log.Printf("Error decoding initial top story IDs: %v", err)
		fmt.Fprintf(w, "event: error\ndata: Error decoding initial top story IDs: %v\n\n", err)
		flusher.Flush()
		return
	}

	lastSentStoryIDs.Lock()
	defer lastSentStoryIDs.Unlock()

	// Process and send a limited number of initial stories
	const initialStoriesLimit = 10 // Send up to 10 initial stories
	storiesToFetch := currentStoryIDs
	if len(storiesToFetch) > initialStoriesLimit {
		storiesToFetch = storiesToFetch[:initialStoriesLimit]
	}

	for _, id := range storiesToFetch {
		storyURL := fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json?print=pretty", id)
		storyResp, err := client.Get(storyURL)
		if err != nil {
			log.Printf("Warning: Error fetching initial story %d: %v", id, err)
			fmt.Fprintf(w, "event: story-error\ndata: Error fetching initial story %d: %v\n\n", id, err)
			flusher.Flush()
			continue
		}
		// No defer storyResp.Body.Close() inside the loop if using fixed client,
		// close happens automatically or explicit close if managing connections.
		// For simple client per request, defer is fine but can be costly in loop.
		// Best practice: close immediately if not deferred per iteration.
		defer storyResp.Body.Close() // This defer will only run when sendInitialHackerNewsStories returns

		var story HNStory
		if err := json.NewDecoder(storyResp.Body).Decode(&story); err != nil {
			log.Printf("Warning: Error decoding initial story %d: %v", id, err)
			fmt.Fprintf(w, "event: story-error\ndata: Error decoding initial story %d: %v\n\n", id, err)
			flusher.Flush()
			continue
		}

		if story.Type != "story" || story.Title == "" || story.URL == "" {
			log.Printf("Skipping non-story or incomplete initial item (ID: %d, Type: %s)", story.ID, story.Type)
			continue
		}

		storyJSON, err := json.Marshal(story)
		if err != nil {
			log.Printf("Warning: Error marshalling initial story %d to JSON: %v", id, err)
			fmt.Fprintf(w, "event: story-error\ndata: Error marshalling initial story %d to JSON: %v\n\n", id, err)
			flusher.Flush()
			continue
		}

		fmt.Fprintf(w, "id: %d\n", story.ID)
		fmt.Fprintf(w, "event: new-story\n") // Use 'new-story' event type for consistency
		fmt.Fprintf(w, "data: %s\n\n", storyJSON)
		flusher.Flush()

		log.Printf("Sent initial story (ID: %d): %s\n", story.ID, story.Title)

		// Mark this story as sent so it's not re-sent by delta updates immediately
		lastSentStoryIDs.ids[story.ID] = struct{}{}
	}
}

// sendDeltaHackerNewsStories fetches the latest top stories and sends only the truly new ones
// that *haven't* been seen by the server's global state yet.
func sendDeltaHackerNewsStories(w http.ResponseWriter, flusher http.Flusher) {
	log.Println("Checking for delta Hacker News stories...")
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get("https://hacker-news.firebaseio.com/v0/topstories.json?print=pretty")
	if err != nil {
		log.Printf("Error fetching top story IDs for delta: %v", err)
		fmt.Fprintf(w, "event: error\ndata: Error fetching top story IDs for delta: %v\n\n", err)
		flusher.Flush()
		return
	}
	defer resp.Body.Close()

	var currentStoryIDs []int
	if err := json.NewDecoder(resp.Body).Decode(&currentStoryIDs); err != nil {
		log.Printf("Error decoding top story IDs for delta: %v", err)
		fmt.Fprintf(w, "event: error\ndata: Error decoding top story IDs for delta: %v\n\n", err)
		flusher.Flush()
		return
	}

	lastSentStoryIDs.Lock()
	defer lastSentStoryIDs.Unlock()

	var newStoryIDs []int
	for _, id := range currentStoryIDs {
		if _, exists := lastSentStoryIDs.ids[id]; !exists {
			newStoryIDs = append(newStoryIDs, id)
		}
		// Add all current IDs to the map, so they are marked as sent for future checks
		lastSentStoryIDs.ids[id] = struct{}{}
	}

	if len(newStoryIDs) == 0 {
		log.Println("No new (delta) Hacker News stories found.")
		fmt.Fprintf(w, "event: no-new-data\ndata: No new stories at %s\n\n", time.Now().Format("15:04:05"))
		flusher.Flush()
		return
	}

	log.Printf("Found %d new (delta) Hacker News stories to send.", len(newStoryIDs))

	const maxNewStoriesToSend = 5
	if len(newStoryIDs) > maxNewStoriesToSend {
		newStoryIDs = newStoryIDs[:maxNewStoriesToSend]
	}

	for _, id := range newStoryIDs {
		storyURL := fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json?print=pretty", id)
		storyResp, err := http.Get(storyURL) // Use http.Get directly
		if err != nil {
			log.Printf("Warning: Error fetching delta story %d: %v", id, err)
			fmt.Fprintf(w, "event: story-error\ndata: Error fetching delta story %d: %v\n\n", id, err)
			flusher.Flush()
			continue
		}
		defer storyResp.Body.Close() // This defer will only run when sendDeltaHackerNewsStories returns

		var story HNStory
		if err := json.NewDecoder(storyResp.Body).Decode(&story); err != nil {
			log.Printf("Warning: Error decoding delta story %d: %v", id, err)
			fmt.Fprintf(w, "event: story-error\ndata: Error decoding delta story %d: %v\n\n", id, err)
			flusher.Flush()
			continue
		}

		if story.Type != "story" || story.Title == "" || story.URL == "" {
			log.Printf("Skipping non-story or incomplete delta item (ID: %d, Type: %s)", story.ID, story.Type)
			continue
		}

		storyJSON, err := json.Marshal(story)
		if err != nil {
			log.Printf("Warning: Error marshalling delta story %d to JSON: %v", id, err)
			fmt.Fprintf(w, "event: story-error\ndata: Error marshalling delta story %d to JSON: %v\n\n", id, err)
			flusher.Flush()
			continue
		}

		fmt.Fprintf(w, "id: %d\n", story.ID)
		fmt.Fprintf(w, "event: new-story\n")
		fmt.Fprintf(w, "data: %s\n\n", storyJSON)

		flusher.Flush()
		log.Printf("Sent new (delta) story (ID: %d): %s\n", story.ID, story.Title)
	}
}
