package indexplan

import (
	"fmt"
	"sort"

	"github.com/divyekant/carto/internal/scanner"
)

const (
	LargeFileThreshold   = 10000
	LargeModuleThreshold = 50
)

type Module struct {
	Name  string `json:"name"`
	Type  string `json:"type,omitempty"`
	Path  string `json:"path,omitempty"`
	Files int    `json:"files"`
}

type Plan struct {
	Project         string         `json:"project"`
	Path            string         `json:"path"`
	Modules         int            `json:"modules"`
	Files           int            `json:"files"`
	Bytes           int64          `json:"bytes"`
	Languages       map[string]int `json:"languages"`
	TopModules      []Module       `json:"top_modules"`
	Large           bool           `json:"large"`
	Recommendations []string       `json:"recommendations,omitempty"`
}

func Build(absPath, projectName, moduleFilter string) (Plan, error) {
	scanResult, err := scanner.Scan(absPath)
	if err != nil {
		return Plan{}, fmt.Errorf("scan failed: %w", err)
	}

	modules := scanResult.Modules
	if moduleFilter != "" {
		var filterErr error
		modules, filterErr = scanner.ResolveModuleFilter(modules, moduleFilter)
		if filterErr != nil {
			return Plan{}, filterErr
		}
	}

	fileByRel := make(map[string]scanner.FileInfo, len(scanResult.Files))
	for _, f := range scanResult.Files {
		fileByRel[f.RelPath] = f
	}

	languages := map[string]int{}
	var bytes int64
	planModules := make([]Module, 0, len(modules))
	for _, m := range modules {
		for _, rel := range m.Files {
			if f, ok := fileByRel[rel]; ok {
				languages[f.Language]++
				bytes += f.Size
			}
		}
		planModules = append(planModules, Module{
			Name:  m.Name,
			Type:  m.Type,
			Path:  m.RelPath,
			Files: len(m.Files),
		})
	}

	return FromModules(projectName, absPath, planModules, countFiles(planModules), bytes, languages), nil
}

func FromModules(projectName, absPath string, modules []Module, files int, bytes int64, languages map[string]int) Plan {
	topModules := append([]Module(nil), modules...)
	sort.SliceStable(topModules, func(i, j int) bool {
		return topModules[i].Files > topModules[j].Files
	})
	if len(topModules) > 10 {
		topModules = topModules[:10]
	}

	plan := Plan{
		Project:    projectName,
		Path:       absPath,
		Modules:    len(modules),
		Files:      files,
		Bytes:      bytes,
		Languages:  languages,
		TopModules: topModules,
	}
	plan.Large = plan.Files > LargeFileThreshold || plan.Modules > LargeModuleThreshold
	if plan.Large {
		plan.Recommendations = []string{
			"avoid foreground full indexing for routine validation",
			"use --module for targeted probes and --incremental after an initial baseline",
			"run full Ultron-scale indexing as a background UI/server job with eval Memories only",
		}
	}
	return plan
}

func countFiles(modules []Module) int {
	total := 0
	for _, m := range modules {
		total += m.Files
	}
	return total
}
