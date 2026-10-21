package common

import (
	"time"
)

// FileType classifies a generated simulation artifact.
type FileType string

const (
	SimulationFileAnalysisReport FileType = "analysis_report" // PDF analysis report (Level-3)
)

// File is metadata for a file generated as part of an analysis (e.g. a Level-3 PDF report).
// The file bytes themselves are stored in a MongoDB GridFS bucket and referenced by GridFSID
// (hex id); this metadata is embedded on the Analysis entity, not on the Simulation.
type File struct {
	UID         string    `json:"uid"`                    // entry id within the simulation
	Name        string    `json:"name"`                   // e.g. "Predator-Prey Analysis.pdf"
	Type        FileType  `json:"type"`                   // what the artifact is
	MimeType    string    `json:"mime_type"`               // e.g. "application/pdf"
	Size        int64     `json:"size"`                   // file size in bytes
	GridFSID    string    `json:"gridfs_id"`               // file id in the GridFS bucket (hex)
	Step        *int64    `json:"step,omitempty"`          // step the analysis covers, if scoped
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"`
}
