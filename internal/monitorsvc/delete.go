package monitorsvc

import "github.com/keshon/beacon/internal/store"

// Delete removes a monitor by ID.
func Delete(st *store.Store, id string) error {
	return st.DeleteMonitor(id)
}
