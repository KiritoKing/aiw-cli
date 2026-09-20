package skills

import "embed"

// Files is the build-time AIW skill snapshot used only when Node or npx is absent.
//
//go:embed aiw-init aiw-reference
var Files embed.FS
