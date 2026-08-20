package proton

import (
	"testing"
	"time"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

// TestSetEncXAttrStringLargeFile reproduces the 42GB-file commit failure:
// BlockSizes carries one entry per 4MB block (10752 entries for 42GB), and
// the server rejects XAttr longer than 65535 characters with
// "This value is too long" (Code=2024). With compression enabled the armored
// ciphertext must stay far below the cap, and the round-trip must preserve
// every field.
func TestSetEncXAttrStringLargeFile(t *testing.T) {
	key, err := crypto.GenerateKey("test", "test@example.com", "x25519", 0)
	if err != nil {
		t.Fatal(err)
	}
	kr, err := crypto.NewKeyRing(key)
	if err != nil {
		t.Fatal(err)
	}

	// 42GB / 4MB = 10752 blocks, the real-world failing shape
	blockSizes := make([]int64, 10752)
	for i := range blockSizes {
		blockSizes[i] = 4 * 1024 * 1024
	}
	blockSizes[len(blockSizes)-1] = 123456 // partial trailing block

	xattr := &RevisionXAttrCommon{
		ModificationTime: time.Now().Format("2006-01-02T15:04:05-0700"),
		Size:             42*1024*1024*1024 + 123456,
		BlockSizes:       blockSizes,
		Digests:          map[string]string{"SHA1": "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
	}

	req := &CommitRevisionReq{}
	if err := req.SetEncXAttrString(kr, kr, xattr); err != nil {
		t.Fatal(err)
	}

	if len(req.XAttr) >= 65535 {
		t.Fatalf("armored XAttr is %d chars, exceeds the server cap of 65535", len(req.XAttr))
	}
	t.Logf("armored XAttr: %d chars for %d blocks", len(req.XAttr), len(blockSizes))

	// round-trip: decryption is compression-transparent and must restore all fields
	rev := &RevisionMetadata{XAttr: req.XAttr}
	dec, err := rev.GetDecXAttrString(kr, kr)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Size != xattr.Size {
		t.Fatalf("size mismatch: %d != %d", dec.Size, xattr.Size)
	}
	if len(dec.BlockSizes) != len(xattr.BlockSizes) {
		t.Fatalf("blockSizes count mismatch: %d != %d", len(dec.BlockSizes), len(xattr.BlockSizes))
	}
	if dec.BlockSizes[len(dec.BlockSizes)-1] != 123456 {
		t.Fatal("trailing block size lost in round-trip")
	}
	if dec.Digests["SHA1"] != xattr.Digests["SHA1"] {
		t.Fatal("digest lost in round-trip")
	}
}
