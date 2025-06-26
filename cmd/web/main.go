package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"vlsa/internal/bus"
	vlsaLog "vlsa/internal/log"
)

// Global state for simplicity (in-memory storage)
var (
	currentLogs []vlsaLog.Log
	logsMutex   sync.RWMutex
)

func main() {
	// Set up bus channel reader for debug logging
	go func() {
		fmt.Println("[WEB] Starting bus channel reader for debug logs")
		for msg := range bus.LogChannel {
			fmt.Printf("[VLSA-DEBUG] %s\n", msg)
		}
	}()
	
	// Serve static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("cmd/web/static/"))))
	
	// API routes
	http.HandleFunc("/api/upload", handleUpload)
	http.HandleFunc("/api/logs", handleLogs)
	http.HandleFunc("/api/logs/", handleLogDetail) // For /api/logs/{id}/source and /api/logs/{id}
	
	// Serve main page
	http.HandleFunc("/", handleIndex)
	
	fmt.Println("VLSA Web Server starting on http://localhost:8080")
	fmt.Println("Make sure to run this from the project root directory")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	
	indexPath := filepath.Join("cmd", "web", "static", "index.html")
	http.ServeFile(w, r, indexPath)
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("[WEB] Upload request received from %s\n", r.RemoteAddr)
	
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		fmt.Printf("[WEB] Error parsing form: %v\n", err)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}
	
	file, header, err := r.FormFile("logfile")
	if err != nil {
		fmt.Printf("[WEB] Error getting file: %v\n", err)
		http.Error(w, "Error getting file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	
	fmt.Printf("[WEB] Received file: %s\n", header.Filename)
	
	// Create temporary file
	tempFile, err := os.CreateTemp("", "vlsa_upload_*."+getFileExtension(header.Filename))
	if err != nil {
		fmt.Printf("[WEB] Error creating temp file: %v\n", err)
		http.Error(w, "Error creating temp file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())
	
	fmt.Printf("[WEB] Created temp file: %s\n", tempFile.Name())
	
	// Copy uploaded file to temp file
	_, err = file.Seek(0, 0)
	if err != nil {
		fmt.Printf("[WEB] Error seeking file: %v\n", err)
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}
	
	_, err = io.Copy(tempFile, file)
	if err != nil {
		fmt.Printf("[WEB] Error copying file: %v\n", err)
		http.Error(w, "Error copying file", http.StatusInternalServerError)
		return
	}
	tempFile.Close()
	
	fmt.Printf("[WEB] File copied successfully, starting log processing...\n")
	
	// Process logs using existing VLSA logic
	logChannel := make(chan vlsaLog.LogProcessingMsg)
	go func() {
		vlsaLog.ProcessLogs(tempFile.Name(), logChannel)
	}()
	
	fmt.Printf("[WEB] Waiting for log processing to complete...\n")
	
	// Collect processed logs
	var processedLogs []vlsaLog.Log
	var processingError string
	for msg := range logChannel {
		fmt.Printf("[WEB] Progress update: %d%%\n", msg.Progress)
		if msg.Error != "" {
			processingError = msg.Error
			fmt.Printf("[WEB] Processing error: %s\n", processingError)
		}
		if msg.Progress == 100 {
			processedLogs = msg.Logs
			break
		}
	}
	
	// Check if there was an error during processing
	if processingError != "" {
		fmt.Printf("[WEB] Log processing failed: %s\n", processingError)
		http.Error(w, processingError, http.StatusBadRequest)
		return
	}
	
	fmt.Printf("[WEB] Log processing completed, found %d logs\n", len(processedLogs))
	
	// Store logs in global state
	logsMutex.Lock()
	currentLogs = processedLogs
	logsMutex.Unlock()
	
	fmt.Printf("[WEB] Logs stored in global state, sending response\n")
	
	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   len(processedLogs),
		"message": fmt.Sprintf("Successfully processed %d logs", len(processedLogs)),
	})
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	logsMutex.RLock()
	logs := make([]map[string]interface{}, len(currentLogs))
	for i, log := range currentLogs {
		logs[i] = map[string]interface{}{
			"id":      i,
			"time":    log.Time.Format("15:04:05"),
			"service": log.Service,
			"message": log.Message,
			"sources": len(log.Sources),
		}
	}
	logsMutex.RUnlock()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func handleLogDetail(w http.ResponseWriter, r *http.Request) {
	// Parse URL path to get log ID and action
	path := strings.TrimPrefix(r.URL.Path, "/api/logs/")
	parts := strings.Split(path, "/")
	
	if len(parts) < 1 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	
	logID, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "Invalid log ID", http.StatusBadRequest)
		return
	}
	
	logsMutex.Lock()
	defer logsMutex.Unlock()
	
	if logID < 0 || logID >= len(currentLogs) {
		http.Error(w, "Log not found", http.StatusNotFound)
		return
	}
	
	// Handle DELETE request for log deletion
	if r.Method == http.MethodDelete {
		// Remove log from slice
		currentLogs = append(currentLogs[:logID], currentLogs[logID+1:]...)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Log deleted successfully",
		})
		return
	}
	
	// Handle GET request for source code
	if len(parts) == 2 && parts[1] == "source" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		currentLog := currentLogs[logID]
		
		// Get selected source index from query parameter, default to 0
		sourceIdx := 0
		if idx := r.URL.Query().Get("source"); idx != "" {
			if parsed, err := strconv.Atoi(idx); err == nil && parsed < len(currentLog.Sources) {
				sourceIdx = parsed
			}
		}
		
		if len(currentLog.Sources) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"path":    "",
				"line":    0,
				"content": "No source code available",
				"sources": []map[string]interface{}{},
			})
			return
		}
		
		if sourceIdx >= len(currentLog.Sources) {
			sourceIdx = 0
		}
		
		source := currentLog.Sources[sourceIdx]
		
		// Build sources list for frontend
		sources := make([]map[string]interface{}, len(currentLog.Sources))
		for i, s := range currentLog.Sources {
			sources[i] = map[string]interface{}{
				"path": s.Path,
				"line": s.Line,
			}
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":         source.Path,
			"line":         source.Line,
			"content":      source.SourceCode,
			"sources":      sources,
			"selectedIdx":  sourceIdx,
		})
		return
	}
	
	http.Error(w, "Invalid path", http.StatusBadRequest)
}

func getFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext != "" {
		return ext[1:] // Remove the dot
	}
	return "txt"
}
