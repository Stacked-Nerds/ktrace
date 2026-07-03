package cli

import (
	"encoding/json"
	"io"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func writeJSON(w io.Writer, graph *models.ResourceGraph) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(graph)
}
