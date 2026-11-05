package models

import (
	"encoding/json"
	"testing"
)

func img(id uint, caption string) Block {
	data, _ := json.Marshal(map[string]interface{}{"asset_id": id, "caption": caption, "size": "medium"})
	return Block{Type: BlockImage, Data: data}
}

func heading(text string) Block {
	data, _ := json.Marshal(map[string]interface{}{"level": 2, "text": text})
	return Block{Type: BlockHeading, Data: data}
}

func galleryIDs(t *testing.T, b Block) []uint {
	t.Helper()
	if b.Type != BlockImageRow {
		t.Fatalf("want image_row, got %s", b.Type)
	}
	var d struct {
		AssetIDs []uint `json:"asset_ids"`
	}
	if err := json.Unmarshal(b.Data, &d); err != nil {
		t.Fatal(err)
	}
	return d.AssetIDs
}

func TestGroupPhotoRuns_FoldsAlbumsPerHeading(t *testing.T) {
	in := Blocks{heading("Picnic"), img(1, ""), img(2, ""), img(3, ""), heading("Easter"), img(4, ""), img(5, ""), img(6, ""), img(7, "")}
	out, changed := GroupPhotoRuns(in, 3)
	if !changed || len(out) != 4 {
		t.Fatalf("changed=%v len=%d, want 4 blocks", changed, len(out))
	}
	if got := galleryIDs(t, out[1]); len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Errorf("first gallery = %v", got)
	}
	if got := galleryIDs(t, out[3]); len(got) != 4 || got[3] != 7 {
		t.Errorf("second gallery = %v", got)
	}
}

func TestGroupPhotoRuns_LeavesShortRunsAndCaptions(t *testing.T) {
	in := Blocks{img(1, ""), img(2, ""), heading("x"), img(3, "The new altar"), img(4, ""), img(5, "")}
	out, changed := GroupPhotoRuns(in, 3)
	if changed {
		t.Fatalf("nothing should change, got %d blocks", len(out))
	}
	if len(out) != len(in) {
		t.Fatalf("len=%d want %d", len(out), len(in))
	}
	for i := range in {
		if out[i].Type != in[i].Type {
			t.Errorf("block %d type changed to %s", i, out[i].Type)
		}
	}
}

func TestGroupPhotoRuns_Idempotent(t *testing.T) {
	in := Blocks{img(1, ""), img(2, ""), img(3, "")}
	once, _ := GroupPhotoRuns(in, 3)
	twice, changed := GroupPhotoRuns(once, 3)
	if changed || len(twice) != 1 {
		t.Fatalf("second pass changed=%v len=%d", changed, len(twice))
	}
}
