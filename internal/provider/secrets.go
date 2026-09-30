package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

type secretState interface {
	GetKey(context.Context, string) ([]byte, diag.Diagnostics)
	SetKey(context.Context, string, []byte) diag.Diagnostics
}

func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func hashEnv(env map[string]string) string {
	keys := slices.Sorted(maps.Keys(env))
	h := sha256.New()
	for _, key := range keys {
		h.Write([]byte(key))
		h.Write([]byte{0})
		h.Write([]byte(env[key]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// replaceIfSecretChanged forces replacement when a write-only value changes.
// A missing prior hash is the first plan after adoption and does not replace.
func replaceIfSecretChanged(ctx context.Context, creating bool, prior, next secretState, key, sum string, attr path.Path) (path.Paths, diag.Diagnostics) {
	var diags diag.Diagnostics
	var replace path.Paths
	if !creating && prior != nil {
		prev, getDiags := prior.GetKey(ctx, key)
		diags.Append(getDiags...)
		if diags.HasError() {
			return nil, diags
		}
		if len(prev) > 0 && string(prev) != sum {
			replace = append(replace, attr)
		}
	}
	if next != nil {
		diags.Append(next.SetKey(ctx, key, []byte(sum))...)
	}
	return replace, diags
}
