package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type SharedFile struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	SizeDisplay string `json:"sizeDisplay"`
	DownloadURL string `json:"downloadUrl"`
	UploadedAt  string `json:"uploadedAt"`
}

func (h *APIHandler) getSharedFilesDir() string {
	dir := filepath.Join(h.store.DataDir(), "shared_files")
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (h *APIHandler) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 100 MB max file size
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, fmt.Sprintf("file too large or invalid multipart form: %v", err), http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid file field: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	safeFilename := filepath.Base(handler.Filename)
	safeFilename = strings.ReplaceAll(safeFilename, "/", "_")
	safeFilename = strings.ReplaceAll(safeFilename, "\\", "_")

	destPath := filepath.Join(h.getSharedFilesDir(), safeFilename)
	dst, err := os.Create(destPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to save file: %v", err), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to write file: %v", err), http.StatusInternalServerError)
		return
	}

	downloadURL := fmt.Sprintf("/api/files/download/%s", safeFilename)
	jsonResponse(w, http.StatusOK, map[string]any{
		"success":     true,
		"name":        safeFilename,
		"size":        written,
		"sizeDisplay": formatBytes(written),
		"downloadUrl": downloadURL,
	})
}

func (h *APIHandler) handleFileList(w http.ResponseWriter, r *http.Request) {
	dir := h.getSharedFilesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		jsonResponse(w, http.StatusOK, []SharedFile{})
		return
	}

	var list []SharedFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		name := e.Name()
		list = append(list, SharedFile{
			Name:        name,
			Size:        info.Size(),
			SizeDisplay: formatBytes(info.Size()),
			DownloadURL: fmt.Sprintf("/api/files/download/%s", name),
			UploadedAt:  info.ModTime().Format("2006-01-02 15:04"),
		})
	}

	if list == nil {
		list = []SharedFile{}
	}
	jsonResponse(w, http.StatusOK, list)
}

func (h *APIHandler) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/api/files/download/")
	filename = filepath.Base(filename)
	if filename == "" || filename == "." {
		http.Error(w, "missing filename", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(h.getSharedFilesDir(), filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	http.ServeFile(w, r, filePath)
}

func (h *APIHandler) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := strings.TrimPrefix(r.URL.Path, "/api/files/delete/")
	filename = filepath.Base(filename)
	if filename == "" || filename == "." {
		http.Error(w, "missing filename", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(h.getSharedFilesDir(), filename)
	_ = os.Remove(filePath)
	jsonResponse(w, http.StatusOK, map[string]any{"success": true, "deleted": filename})
}
