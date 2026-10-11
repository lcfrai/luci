package product

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

// Observe saved previews without rewriting their provenance or replaying a
// resource operation. One budget bounds the whole history, not each old row.
// Unknown is recoverable: a later observation can establish ready again and
// the existing explicit Stop remains available for a retained container.
func (s *Server) observeLocalReleases(parent context.Context, rows []storedRelease) []storedRelease {
	ctx, cancel := context.WithTimeout(parent, 6*time.Second)
	defer cancel()
	result := make([]storedRelease, 0, len(rows))
	for _, saved := range rows {
		row := saved
		if row.Runtime != nil {
			s.mu.RLock()
			active := s.activeLocalReleases[row.ID]
			s.mu.RUnlock()
			if !active && row.Status != "stopped" && (row.Runtime.ResourcesHeld || row.Status == "building" || row.Status == "unknown") {
				runtime := s.PreviewRuntime()
				row.Status, row.URL = "unknown", ""
				row.Message = "This preview may have been interrupted. Its receipt is retained; refresh to verify it or stop this exact allocation."
				if runtime.Observe != nil && ctx.Err() == nil {
					out, err := runtime.Observe(ctx, saved.Runtime.Identity)
					if err == nil && out.Identity == saved.Runtime.Identity && out.Phase == "running" && out.ResourcesHeld {
						row.URL = out.URL
						if _, originErr := recordedPreviewOrigin(row.URL); originErr == nil && checkPreviewHTTP(ctx, row.URL) == nil {
							row.Status = "ready"
						} else {
							row.URL = ""
						}
					}
				}
			}
			result = append(result, row)
			continue
		}
		if row.Target == "local" {
			switch row.Status {
			case "building":
				s.mu.RLock()
				active := s.activeLocalReleases[row.ID]
				s.mu.RUnlock()
				if !active {
					row.Status, row.URL = "unknown", ""
					row.Message = "This preview build was interrupted. Its source and history are retained. Stop any recorded preview, then create a new preview from the committed source."
				}
			case "ready":
				if ctx.Err() != nil || s.verifyReleasePreview(ctx, row) != nil {
					row.Status, row.URL = "unknown", ""
					row.Message = "The owned preview could not be verified as running at its recorded port. Check Docker and refresh, or stop and recreate the preview. Its source and history are retained."
				} else if checkPreviewHTTP(ctx, row.URL) != nil {
					row.Status, row.URL = "unknown", ""
					row.Message = "The owned preview is running, but its HTTP health check did not succeed. Refresh when it is healthy, or stop and recreate it. Its source and history are retained."
				}
			}
		}
		result = append(result, row)
	}
	return result
}

// Only called after native running/label/loopback checks. Redirects must not
// turn a saved private preview into a request to a different host.
func checkPreviewHTTP(parent context.Context, origin string) error {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New("owned preview did not return a successful HTTP response")
	}
	return nil
}
