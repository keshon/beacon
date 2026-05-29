package commands

import (
	"encoding/json"
	"io"
)

func writeJSONToWriter(out io.Writer, v any) {
	json.NewEncoder(out).Encode(v)
}
