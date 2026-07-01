package runner

import "github.com/keshon/beacon/internal/storage"

// Delete removes a monitor by ID.
func Delete(st *storage.Store, id string) error {
	return st.DeleteMonitor(id)
}
