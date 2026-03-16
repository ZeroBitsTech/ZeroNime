package observability

import (
	"encoding/json"
	"log"
	"time"
)

func Request(entry map[string]any) {
	entry["ts"] = time.Now().UTC().Format(time.RFC3339)
	raw, _ := json.Marshal(entry)
	log.Printf("%s", raw)
}
