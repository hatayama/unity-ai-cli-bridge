package cli

import (
	"fmt"
	"io"
)

func writef(writer io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(writer, format, args...)
}

func writeln(writer io.Writer, args ...any) {
	_, _ = fmt.Fprintln(writer, args...)
}
