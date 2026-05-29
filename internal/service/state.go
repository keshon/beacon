package service

import (
	"strconv"

	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
)

func GetAllState(st *store.Store) (map[string]*monitor.MonitorState, error) {
	return st.GetAllState()
}

func GetCheckRecords(st *store.Store, limit int) ([]store.CheckRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	return st.GetCheckRecords(limit)
}

func ParseRecordLimit(s string) int {
	if s == "" {
		return 100
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 100
	}
	return n
}
