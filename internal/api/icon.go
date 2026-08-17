package api

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// customIconPrefix is the stored file name for uploaded icons.
const customIconPrefix = "custom-icon"

// allowedIconExts whitelists uploadable icon formats.
var allowedIconExts = map[string]bool{".svg": true, ".png": true}

const maxIconSize = 1 << 20 // 1 MiB

// iconPath resolves the stored path for an uploaded icon with the given ext.
func (s *Server) iconPath(ext string) string {
	return filepath.Join(s.assetsDir, customIconPrefix+ext)
}

// removeStoredIcon deletes every stored custom icon, whatever the extension.
func (s *Server) removeStoredIcon() error {
	for ext := range allowedIconExts {
		if err := os.Remove(s.iconPath(ext)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// uploadIcon handles POST /api/v1/icon (multipart field "icon"). The file is
// stored as custom-icon.<ext> in the assets directory and the config's icon
// field is updated, so favicons pick it up immediately.
func (s *Server) uploadIcon(w http.ResponseWriter, r *http.Request) {
	if err := os.MkdirAll(s.assetsDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to create assets dir")
		return
	}
	file, header, err := r.FormFile("icon")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "missing icon file field")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedIconExts[ext] {
		writeError(w, http.StatusBadRequest, "invalid_request", "icon must be an .svg or .png file")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxIconSize+1))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to read icon")
		return
	}
	if len(data) > maxIconSize {
		writeError(w, http.StatusBadRequest, "invalid_request", "icon exceeds 1 MiB")
		return
	}

	if err := os.WriteFile(s.iconPath(ext), data, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to store icon")
		return
	}
	if err := s.removeStoredIconExcept(ext); err != nil {
		slog.Warn("remove stale icons failed", "error", err)
	}

	cfg := s.cfg.Get()
	cfg.Icon = customIconPrefix + ext
	if err := s.cfg.Update(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to update config")
		return
	}
	slog.Info("icon uploaded", "ext", ext, "actor", actorFrom(r))
	writeJSON(w, http.StatusOK, map[string]any{"icon": cfg.Icon})
}

// removeStoredIconExcept deletes stored icons of other extensions.
func (s *Server) removeStoredIconExcept(keepExt string) error {
	for ext := range allowedIconExts {
		if ext == keepExt {
			continue
		}
		if err := os.Remove(s.iconPath(ext)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// deleteIcon handles DELETE /api/v1/icon: removes the stored file and falls
// back to the built-in default.
func (s *Server) deleteIcon(w http.ResponseWriter, r *http.Request) {
	if err := s.removeStoredIcon(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to remove icon")
		return
	}
	cfg := s.cfg.Get()
	cfg.Icon = ""
	if err := s.cfg.Update(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to update config")
		return
	}
	slog.Info("icon deleted", "actor", actorFrom(r))
	writeJSON(w, http.StatusOK, map[string]any{"icon": ""})
}
