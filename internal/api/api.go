package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthdm/foreverstore/internal/node"
	"github.com/anthdm/foreverstore/web"
)

// getLocalLANIP retrieves the host's actual outbound IPv4 address on the local network (Ethernet / Wi-Fi).
func getLocalLANIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		if localAddr.IP != nil && !localAddr.IP.IsLoopback() {
			return localAddr.IP.String()
		}
	}

	// Fallback to iterating interfaces
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.String()
				}
			}
		}
	}
	return "127.0.0.1"
}

// Server exposes the MeshStore REST API over HTTP.
type Server struct {
	node      *node.Node
	startTime time.Time
	httpSrv   *http.Server
}

// NewServer creates a new API server.
func NewServer(n *node.Node) *Server {
	s := &Server{
		node:      n,
		startTime: time.Now().UTC(),
	}

	mux := http.NewServeMux()
	mux.Handle("/", web.Handler())
	mux.HandleFunc("/api/v1/status", s.handleStatus)
	mux.HandleFunc("/api/v1/nodes", s.handleNodes)
	mux.HandleFunc("/api/v1/nodes/add", s.handleAddNode)
	mux.HandleFunc("/api/v1/files", s.handleFiles)
	mux.HandleFunc("/api/v1/files/", s.handleFileItem)
	mux.HandleFunc("/api/v1/transfers", s.handleTransfers)
	mux.HandleFunc("/api/v1/storage", s.handleStorage)
	mux.HandleFunc("/api/v1/binaries", s.handleBinaries)
	mux.HandleFunc("/api/v1/binaries/", s.handleDownloadBinary)

	s.httpSrv = &http.Server{
		Addr:    n.Config.API.ListenAddress,
		Handler: s.corsMiddleware(mux),
	}

	return s
}

// Start begins listening on the configured HTTP address.
func (s *Server) Start() error {
	return s.httpSrv.ListenAndServe()
}

// Stop shuts down the API server.
func (s *Server) Stop() error {
	return s.httpSrv.Close()
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{"error": message})
}

// GET /api/v1/status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	used, max, avail := s.node.Store.Stats()
	listenAddr := s.node.Config.Node.ListenAddress
	_, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		parts := strings.Split(listenAddr, ":")
		if len(parts) > 1 {
			port = parts[len(parts)-1]
		} else {
			port = "7001"
		}
	}
	lanIP := getLocalLANIP()
	lanAddress := net.JoinHostPort(lanIP, port)

	s.writeJSON(w, http.StatusOK, map[string]any{
		"node_id":            s.node.Identity.NodeID,
		"display_name":       s.node.Identity.DisplayName,
		"listen_address":     s.node.Config.Node.ListenAddress,
		"lan_ip":             lanIP,
		"lan_address":        lanAddress,
		"p2p_port":           port,
		"network_id":         s.node.Config.Mesh.NetworkID,
		"uptime_seconds":     int64(time.Since(s.startTime).Seconds()),
		"storage_used":       used,
		"storage_max":        max,
		"storage_avail":      avail,
		"chunk_size_mb":      s.node.Config.Storage.ChunkSizeMB,
		"replication_factor": s.node.Config.Storage.ReplicationFactor,
	})
}

// GET /api/v1/nodes
func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	nodes := s.node.Index.GetNodes()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"nodes": nodes,
		"count": len(nodes),
	})
}

// POST /api/v1/nodes/add
func (s *Server) handleAddNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Address string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Address == "" {
		s.writeError(w, http.StatusBadRequest, "valid peer address required")
		return
	}

	if err := s.node.ConnectPeer(req.Address); err != nil {
		s.writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to connect to peer: %v", err))
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"message": "peer connection initiated",
		"address": req.Address,
	})
}

// GET /api/v1/files or POST /api/v1/files/upload
func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		files := s.node.Index.ListFiles()
		s.writeJSON(w, http.StatusOK, map[string]any{
			"files": files,
			"count": len(files),
		})

	case http.MethodPost:
		// Multipart file upload
		if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB in-memory buffer
			s.writeError(w, http.StatusBadRequest, "invalid multipart form")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "missing 'file' form field")
			return
		}
		defer file.Close()

		logicalPath := r.FormValue("path")
		if logicalPath == "" {
			logicalPath = "/" + header.Filename
		}

		record, err := s.node.UploadFile(header.Filename, logicalPath, file)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("upload failed: %v", err))
			return
		}

		s.writeJSON(w, http.StatusCreated, record)

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// /api/v1/files/{id} and /api/v1/files/{id}/download
func (s *Server) handleFileItem(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/files/")
	parts := strings.Split(path, "/")
	fileID := parts[0]

	if fileID == "" {
		s.writeError(w, http.StatusBadRequest, "file id required")
		return
	}

	if len(parts) == 2 && parts[1] == "download" {
		// Download file
		record, err := s.node.Index.GetFile(fileID)
		if err != nil {
			s.writeError(w, http.StatusNotFound, "file not found")
			return
		}

		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", record.FileName))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", record.SizeBytes))

		if err := s.node.DownloadFile(fileID, w); err != nil {
			// Headers already sent if streaming started, log error
			return
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		record, err := s.node.Index.GetFile(fileID)
		if err != nil {
			s.writeError(w, http.StatusNotFound, "file not found")
			return
		}

		type chunkDetail struct {
			Index     int      `json:"index"`
			ChunkID   string   `json:"chunk_id"`
			SizeBytes int64    `json:"size_bytes"`
			Locations []string `json:"locations"`
		}

		details := make([]chunkDetail, 0, len(record.Chunks))
		for _, ch := range record.Chunks {
			details = append(details, chunkDetail{
				Index:     ch.Index,
				ChunkID:   ch.ChunkID,
				SizeBytes: ch.SizeBytes,
				Locations: s.node.Index.GetChunkLocations(ch.ChunkID),
			})
		}

		s.writeJSON(w, http.StatusOK, map[string]any{
			"file":   record,
			"chunks": details,
		})

	case http.MethodDelete:
		if err := s.node.Index.DeleteFile(fileID); err != nil {
			s.writeError(w, http.StatusNotFound, "file not found")
			return
		}
		_ = s.node.Index.Save()
		s.writeJSON(w, http.StatusOK, map[string]string{"message": "file marked deleted"})

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// GET /api/v1/transfers
func (s *Server) handleTransfers(w http.ResponseWriter, r *http.Request) {
	transfers := s.node.Index.ListTransfers()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"transfers": transfers,
		"count":     len(transfers),
	})
}

// GET, POST, or PATCH /api/v1/storage
func (s *Server) handleStorage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		totalCap, totalUsed, avail, onlineNodes, totalNodes := s.node.Index.MeshStorageStats()
		rFactor := s.node.Config.Storage.ReplicationFactor
		usableCapacity := totalCap
		if rFactor > 0 {
			usableCapacity = totalCap / int64(rFactor)
		}

		s.writeJSON(w, http.StatusOK, map[string]any{
			"raw_capacity_bytes":    totalCap,
			"used_bytes":            totalUsed,
			"available_bytes":       avail,
			"usable_capacity_bytes": usableCapacity,
			"replication_factor":    rFactor,
			"nodes_online":          onlineNodes,
			"nodes_total":           totalNodes,
			"storage_health":        s.calculateStorageHealth(onlineNodes, totalNodes),
		})

	case http.MethodPost, http.MethodPatch:
		var req struct {
			MaxStorageGB int64 `json:"max_storage_gb"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MaxStorageGB <= 0 {
			s.writeError(w, http.StatusBadRequest, "valid max_storage_gb required (must be > 0)")
			return
		}

		s.node.SetStorageQuotaGB(req.MaxStorageGB)
		s.writeJSON(w, http.StatusOK, map[string]any{
			"message":        "storage quota updated successfully",
			"max_storage_gb": req.MaxStorageGB,
		})

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) calculateStorageHealth(online int, total int) string {
	if total == 0 || online == total {
		return "healthy"
	}
	if online > 0 {
		return "degraded"
	}
	return "critical"
}

func (s *Server) handleBinaries(w http.ResponseWriter, r *http.Request) {
	type BinaryInfo struct {
		Name        string `json:"name"`
		Platform    string `json:"platform"`
		Arch        string `json:"arch"`
		Description string `json:"description"`
		DownloadURL string `json:"download_url"`
		SizeBytes   int64  `json:"size_bytes"`
		Available   bool   `json:"available"`
	}

	targets := []struct {
		Filename string
		Platform string
		Arch     string
		Desc     string
	}{
		{"diofos-darwin-arm64", "macOS", "ARM64 (Apple Silicon M1/M2/M3/M4)", "Standalone binary for Apple Silicon Macs"},
		{"diofos-darwin-amd64", "macOS", "x86_64 (Intel)", "Standalone binary for Intel-based Macs"},
		{"diofos-windows-amd64.exe", "Windows", "x86_64 (64-bit)", "Standalone executable for 64-bit Windows"},
		{"diofos-linux-amd64", "Linux", "x86_64", "Standalone binary for 64-bit Linux distributions"},
		{"diofos-linux-arm64", "Linux", "ARM64", "Standalone binary for ARM64 Linux / Raspberry Pi"},
	}

	binaries := make([]BinaryInfo, 0, len(targets))
	for _, t := range targets {
		distPath := filepath.Join("bin", "dist", t.Filename)
		info, err := os.Stat(distPath)
		var size int64
		avail := false
		if err == nil && !info.IsDir() {
			size = info.Size()
			avail = true
		}
		binaries = append(binaries, BinaryInfo{
			Name:        t.Filename,
			Platform:    t.Platform,
			Arch:        t.Arch,
			Description: t.Desc,
			DownloadURL: "/api/v1/binaries/" + t.Filename,
			SizeBytes:   size,
			Available:   avail,
		})
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"binaries": binaries,
	})
}

func (s *Server) handleDownloadBinary(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/api/v1/binaries/")
	filename = filepath.Base(filename)
	if filename == "." || filename == "/" || filename == "" {
		s.writeError(w, http.StatusBadRequest, "binary filename required")
		return
	}

	distPath := filepath.Join("bin", "dist", filename)
	info, err := os.Stat(distPath)
	if err != nil || info.IsDir() {
		fallback := filepath.Join("bin", filename)
		info, err = os.Stat(fallback)
		if err != nil || info.IsDir() {
			s.writeError(w, http.StatusNotFound, "binary not found")
			return
		}
		distPath = fallback
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, distPath)
}
