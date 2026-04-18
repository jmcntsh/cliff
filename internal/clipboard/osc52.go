package clipboard

import (
	"encoding/base64"
	"fmt"
	"os"
)

func WriteOSC52(text string) {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	fmt.Fprintf(os.Stderr, "\x1b]52;c;%s\x07", encoded)
}
