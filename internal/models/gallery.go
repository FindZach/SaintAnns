package models

import "encoding/json"

// GroupPhotoRuns folds every run of consecutive, caption-less image blocks
// of at least minRun into a single image_row (photo gallery) block, keeping
// order. Pictures with a caption, and short runs, are left as they are, so an
// article with two illustrated paragraphs isn't turned into an album. The
// WordPress import produced pages with a hundred-plus stacked pictures;
// this is what turns them into galleries that fill the screen.
func GroupPhotoRuns(blocks Blocks, minRun int) (Blocks, bool) {
	type imgPayload struct {
		AssetID uint   `json:"asset_id"`
		Caption string `json:"caption"`
	}
	type rowPayload struct {
		AssetIDs []uint `json:"asset_ids"`
		Caption  string `json:"caption"`
	}
	if minRun < 2 {
		minRun = 2
	}
	var out Blocks
	var run []Block
	var runIDs []uint
	changed := false
	flush := func() {
		if len(run) >= minRun {
			data, _ := json.Marshal(rowPayload{AssetIDs: runIDs})
			out = append(out, Block{Type: BlockImageRow, Data: data})
			changed = true
		} else {
			out = append(out, run...)
		}
		run, runIDs = nil, nil
	}
	for _, b := range blocks {
		if b.Type == BlockImage {
			var p imgPayload
			if json.Unmarshal(b.Data, &p) == nil && p.AssetID != 0 && p.Caption == "" {
				run = append(run, b)
				runIDs = append(runIDs, p.AssetID)
				continue
			}
		}
		flush()
		out = append(out, b)
	}
	flush()
	return out, changed
}
