package mockbunny

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// handleDeleteRecord handles DELETE /dnszone/{zoneId}/records/{id}
// Returns 204 No Content on success, 404 if zone or record not found, 400 for invalid IDs.
func (s *Server) handleDeleteRecord(w http.ResponseWriter, r *http.Request) {
	// Parse zone ID from URL
	zoneIDStr := chi.URLParam(r, "zoneId")
	zoneID, err := strconv.ParseInt(zoneIDStr, 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid zone ID: %v", err), http.StatusBadRequest)
		return
	}

	// Parse record ID from URL
	recordIDStr := chi.URLParam(r, "id")
	recordID, err := strconv.ParseInt(recordIDStr, 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid record ID: %v", err), http.StatusBadRequest)
		return
	}

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	// Check if zone exists
	zone, ok := s.state.zones[zoneID]
	if !ok {
		http.Error(w, "zone not found", http.StatusNotFound)
		return
	}

	// Find and remove record
	found := false
	for i, record := range zone.Records {
		if record.ID == recordID {
			zone.Records = append(zone.Records[:i], zone.Records[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "record not found", http.StatusNotFound)
		return
	}

	// Update zone's DateModified
	zone.DateModified = time.Now().UTC()

	// Return 204 No Content on success
	w.WriteHeader(http.StatusNoContent)
}
