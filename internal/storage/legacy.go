package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/keshon/datastore"

	"github.com/keshon/beacon/internal/monitor"
)

// Перенос со старых JSON-файлов.
//
// До datastore каждая коллекция лежала в своём файле вида {"<ключ>": <данные>}.
// Файлы НЕ удаляются: перенос читает их и оставляет как есть — если что-то
// пойдёт не так, откатиться можно, просто вернув старый бинарник.
//
// Повторного переноса не будет, и это гарантирует запись-отметка, а не
// «коллекции непусты». Разница важна: пользователь, удаливший все мониторы,
// иначе получил бы их обратно при следующем запуске.

const legacyMarkKey = "legacy-import"

type markRec struct {
	Name string    `json:"name"`
	At   time.Time `json:"at"`
	Note string    `json:"note"`
}

func (m *markRec) Key() string { return m.Name }

func (s *Store) importLegacy() error {
	if _, done := s.marks.Get(legacyMarkKey); done {
		return nil
	}

	var (
		monitors map[string]*monitor.Monitor
		states   map[string]*monitor.MonitorState
		events   []CheckRecord
		cfg      json.RawMessage
		peers    map[string]*PeerData
	)
	files := []struct {
		name string
		key  string
		dest any
	}{
		{"monitors.json", "monitors", &monitors},
		{"state.json", "state", &states},
		{"events.json", "events", &events},
		{"config.json", "config", &cfg},
		{"peer_data.json", "peer_data", &peers},
	}

	found := false
	for _, f := range files {
		ok, err := loadWrapped(filepath.Join(s.dataDir, f.name), f.key, f.dest)
		if err != nil {
			return err
		}
		found = found || ok
	}
	if !found {
		// Чистая установка: отмечаем, чтобы больше не искать.
		return s.marks.Put(&markRec{Name: legacyMarkKey, At: time.Now(), Note: "нечего переносить"})
	}

	// Всё одной транзакцией: либо старые данные целиком в новом хранилище,
	// либо ничего и отметки нет — тогда следующий запуск попробует снова.
	err := s.db.Update(func(tx *datastore.Tx) error {
		mons := datastore.In(tx, s.monitors)
		for _, m := range monitors {
			if m == nil || m.ID == "" {
				continue
			}
			if err := mons.Put(&monitorRec{M: m}); err != nil {
				return err
			}
		}

		sts := datastore.In(tx, s.state)
		for _, st := range states {
			if st == nil || st.MonitorID == "" {
				continue
			}
			if err := sts.Put(&stateRec{S: st}); err != nil {
				return err
			}
		}

		// Старое кольцо держало до десяти тысяч записей на всех; новое правило
		// — свои N на каждый монитор, поэтому берём хвост по монитору.
		checks := datastore.In(tx, s.checks)
		perMonitor := map[string][]CheckRecord{}
		for _, rec := range events {
			if rec.MonitorID == "" {
				continue
			}
			perMonitor[rec.MonitorID] = append(perMonitor[rec.MonitorID], rec)
		}
		for _, list := range perMonitor {
			if len(list) > uptimeIndexLimit {
				list = list[len(list)-uptimeIndexLimit:]
			}
			for i := range list {
				cp := list[i]
				if err := checks.Put(&cp); err != nil {
					return err
				}
			}
		}

		if len(cfg) > 0 {
			if err := datastore.In(tx, s.config).Put(&configRec{Raw: cfg}); err != nil {
				return err
			}
		}

		prs := datastore.In(tx, s.peers)
		for _, pd := range peers {
			if pd == nil || pd.NodeID == "" {
				continue
			}
			if err := prs.Put(pd); err != nil {
				return err
			}
		}

		return datastore.In(tx, s.marks).Put(&markRec{
			Name: legacyMarkKey,
			At:   time.Now(),
			Note: fmt.Sprintf("мониторов %d, состояний %d, проверок %d, пиров %d",
				len(monitors), len(states), len(events), len(peers)),
		})
	})
	if err != nil {
		return fmt.Errorf("перенос старых данных: %w", err)
	}
	return nil
}

// loadWrapped reads {"<key>": <value>} from path into dest. Reports whether the
// file held anything; a missing file is not an error.
func loadWrapped(path, key string, dest any) (bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read %q: %w", path, err)
	}
	if len(raw) == 0 {
		return false, nil
	}
	var wrap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return false, fmt.Errorf("parse %q: %w", path, err)
	}
	v, ok := wrap[key]
	if !ok || len(v) == 0 || string(v) == "null" {
		return false, nil
	}
	if err := json.Unmarshal(v, dest); err != nil {
		return false, fmt.Errorf("parse %q key %q: %w", path, key, err)
	}
	return true, nil
}
